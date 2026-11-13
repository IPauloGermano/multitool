package runner

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
	"github.com/IPauloGermano/multitool/internal/system"
)

func securityRunner(t *testing.T, mock *MockExecutor, targetOS system.OSType) *Runner {
	t.Helper()
	reg, err := registry.New(filepath.Join(t.TempDir(), "installed.json"))
	if err != nil {
		t.Fatal(err)
	}
	r := NewWithExecutor(reg, mock, false)
	info := *r.sysInfo
	r.sysInfo = &info
	r.sysInfo.OS = targetOS
	r.sysInfo.SkillDirs = nil
	return r
}

func TestRustupUninstallPreservesUserFiles(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			sentinel := errors.New("official uninstall failed")
			mock := &MockExecutor{}
			if fail {
				mock.errors = map[string]error{"rustup self uninstall": sentinel}
			}
			r := securityRunner(t, mock, system.OSLinux)
			files := []string{".cargo/credentials.toml", ".cargo/config.toml", ".cargo/bin/unrelated", ".rustup/settings.toml"}
			for _, name := range files {
				path := filepath.Join(home, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("preserve me"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := r.reg.MarkInstalled("rustup"); err != nil {
				t.Fatal(err)
			}
			err := r.Uninstall(*catalog.FindTool("rustup"))
			if fail && !errors.Is(err, sentinel) {
				t.Fatalf("expected wrapped error, got %v", err)
			}
			if !fail && err != nil {
				t.Fatal(err)
			}
			if r.reg.IsInstalled("rustup") != fail {
				t.Fatal("registry does not reflect uninstall result")
			}
			for _, name := range files {
				data, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(name)))
				if err != nil || string(data) != "preserve me" {
					t.Fatalf("user file changed: %s: %v", name, err)
				}
			}
		})
	}
}

func TestArtifactIntegrityBeforeConsumption(t *testing.T) {
	body := []byte("verified installer")
	goodHash := fmt.Sprintf("%x", sha256.Sum256(body))
	for _, recipe := range []string{"script", "installer", "archive", "binary", "multi"} {
		for _, valid := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", recipe, valid), func(t *testing.T) {
				t.Setenv("LOCALAPPDATA", t.TempDir())
				mock := &MockExecutor{downloadBody: body}
				r := securityRunner(t, mock, system.OSWindows)
				hash := goodHash
				if !valid {
					hash = strings.Repeat("0", 64)
				}
				target := &catalog.Target{Type: catalog.TypeDirect, SHA256: hash}
				var err error
				switch recipe {
				case "script":
					r.sysInfo.OS = system.OSLinux
					err = r.installScript("uv", target)
				case "binary":
					r.sysInfo.OS = system.OSLinux
					err = r.installScript("orca", target)
				case "archive":
					err = r.installDirectWindowsTargetWithActivity(nil, "wabt", target)
				case "multi":
					target.SHA256 = ""
					target.ArtifactSHA256 = map[string]string{"apktool.bat": goodHash, "apktool.jar": hash}
					err = r.installDirectWindowsTargetWithActivity(nil, "apktool", target)
				default:
					err = r.installDirectWindowsTargetWithActivity(nil, "orca", target)
				}
				if valid && err != nil {
					t.Fatal(err)
				}
				if !valid && (err == nil || !strings.Contains(err.Error(), "integridade")) {
					t.Fatalf("expected integrity error, got %v", err)
				}
				for _, cmd := range mock.executed {
					if cmd[0] == "curl" || cmd[0] == "curl.exe" {
						for i, arg := range cmd {
							if arg == "-o" {
								if _, err := os.Stat(filepath.Dir(cmd[i+1])); !os.IsNotExist(err) {
									t.Fatal("temporary directory was not removed")
								}
							}
						}
					} else if !valid {
						t.Fatalf("artifact consumed before validation: %v", cmd)
					}
				}
				if recipe == "multi" && !valid {
					if _, err := os.Stat(filepath.Join(os.Getenv("LOCALAPPDATA"), "multitool", "bin", "apktool.bat")); !os.IsNotExist(err) {
						t.Fatal("published before all checksums passed")
					}
				}
			})
		}
	}
}

func TestRustupMissingUninstallerReturnsError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := securityRunner(t, &MockExecutor{missing: map[string]bool{"rustup": true}}, system.OSLinux)
	if err := r.reg.MarkInstalled("rustup"); err != nil {
		t.Fatal(err)
	}
	if err := r.Uninstall(*catalog.FindTool("rustup")); err == nil {
		t.Fatal("missing official uninstaller must fail")
	}
	if !r.reg.IsInstalled("rustup") {
		t.Fatal("failed uninstall removed registry entry")
	}
}

func TestArtifactDownloadFailureAndInvalidHash(t *testing.T) {
	for _, hash := range []string{"invalid", strings.Repeat("0", 64)} {
		mock := &MockExecutor{errors: map[string]error{"curl": errors.New("network failed")}}
		r := securityRunner(t, mock, system.OSLinux)
		dir := t.TempDir()
		_, err := r.downloadArtifact(nil, &catalog.Target{SHA256: hash}, dir, "artifact", "https://example.invalid/artifact")
		if err == nil {
			t.Fatal("expected failure")
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("partial artifact left behind: %v", err)
		}
		if hash == "invalid" && len(mock.executed) != 0 {
			t.Fatal("download attempted with invalid checksum")
		}
	}
}

func TestUnhashedDownloadWarnsAndConflictingHashesFail(t *testing.T) {
	mock := &MockExecutor{}
	r := securityRunner(t, mock, system.OSLinux)
	log, err := os.CreateTemp(t.TempDir(), "stderr-*")
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stderr
	os.Stderr = log
	defer func() { os.Stderr = previous; _ = log.Close() }()
	if _, err := r.downloadArtifact(nil, &catalog.Target{}, t.TempDir(), "artifact", "https://example.invalid/artifact"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log.Name())
	if err != nil || !strings.Contains(string(data), "AVISO:") || !strings.Contains(string(data), "sem SHA-256") {
		t.Fatalf("missing explicit warning: %s, %v", data, err)
	}
	mock.executed = nil
	_, err = r.downloadArtifact(nil, &catalog.Target{SHA256: strings.Repeat("0", 64), ArtifactSHA256: map[string]string{"artifact": strings.Repeat("1", 64)}}, t.TempDir(), "artifact", "https://example.invalid/artifact")
	if err == nil || len(mock.executed) != 0 {
		t.Fatal("conflicting checksum configuration was ignored")
	}
}

func TestIntegrityFailureDoesNotChangeInstalledState(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(fmt.Sprint(update), func(t *testing.T) {
			t.Setenv("LOCALAPPDATA", t.TempDir())
			r := securityRunner(t, &MockExecutor{downloadBody: []byte("untrusted")}, system.OSWindows)
			r.sysInfo.Arch = "amd64"
			tool := catalog.Tool{ID: "orca", Name: "Orca", Windows: &catalog.Target{Type: catalog.TypeDirect, SHA256: strings.Repeat("0", 64)}}
			var err error
			if update {
				if err := r.reg.MarkInstalled(tool.ID); err != nil {
					t.Fatal(err)
				}
				err = r.Update(tool)
			} else {
				err = r.Install(tool)
			}
			if err == nil || !strings.Contains(err.Error(), "integridade") {
				t.Fatalf("expected integrity error: %v", err)
			}
			if r.reg.IsInstalled(tool.ID) != update {
				t.Fatal("failed verification changed installed state")
			}
		})
	}
}
