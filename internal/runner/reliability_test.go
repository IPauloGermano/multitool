package runner

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/system"
)

func TestBatchPartialFailuresAreJoined(t *testing.T) {
	first := errors.New("first failure")
	second := errors.New("second failure")
	for _, operation := range []string{"install", "uninstall", "update", "parallel-skills", "mixed"} {
		t.Run(operation, func(t *testing.T) {
			mock := &MockExecutor{errors: map[string]error{"bad-one": first, "bad-two": second}}
			r := securityRunner(t, mock, system.OSLinux)
			r.sysInfo.NeedsSudo = false
			r.sysInfo.SkillDirs = nil
			var tools []catalog.Tool
			for _, id := range []string{"bad-one", "good", "bad-two", "after-failure"} {
				target := &catalog.Target{Type: catalog.TypeNpm, PackageID: id, Binary: id}
				if operation == "parallel-skills" || operation == "mixed" && id != "after-failure" {
					target.Type = catalog.TypeSkill
				}
				tools = append(tools, catalog.Tool{ID: id, Name: id, Linux: target})
			}
			// Isola também os diretórios de clone das skills.
			t.Setenv("HOME", t.TempDir())
			t.Setenv("USERPROFILE", os.Getenv("HOME"))
			var ok, fail int
			var err error
			switch operation {
			case "uninstall":
				ok, fail, err = r.RunBatchUninstallWithErrors(tools)
			case "update":
				ok, fail, err = r.RunBatchUpdateWithErrors(tools)
			default:
				ok, fail, err = r.RunBatchInstallWithErrors(tools)
			}
			if ok != 2 || fail != 2 || !errors.Is(err, first) || !errors.Is(err, second) {
				t.Fatalf("ok=%d fail=%d error=%v", ok, fail, err)
			}
			if !strings.Contains(err.Error(), "bad-one") || !strings.Contains(err.Error(), "bad-two") {
				t.Fatal("missing tool IDs in joined error")
			}
			foundLast := false
			for _, cmd := range mock.executed {
				if strings.Contains(strings.Join(cmd, " "), "after-failure") {
					foundLast = true
				}
			}
			if !foundLast {
				t.Fatal("batch stopped at first failure")
			}
		})
	}
}

func TestDownloadArchitectureSelection(t *testing.T) {
	for _, tc := range []struct {
		id, arch, asset string
		os              system.OSType
	}{
		{"gbrain", "amd64", "gbrain-linux-x64", system.OSLinux},
		{"gbrain", "arm64", "gbrain-darwin-arm64", system.OSDarwin},
		{"gbrain", "arm64", "", system.OSLinux},
		{"gbrain", "amd64", "", system.OSDarwin},
		{"orca", "arm64", "orca-linux-arm64.AppImage", system.OSLinux},
		{"orca", "amd64", "orca-linux.AppImage", system.OSLinux},
		{"orca", "arm64", "", system.OSWindows},
		{"openmausbot", "arm64", "", system.OSLinux},
		{"cherrystudio", "arm64", "Cherry-Studio-2.2.2-linux-arm64.AppImage", system.OSLinux},
		{"cherrystudio", "amd64", "Cherry-Studio-2.2.2-linux-x64.AppImage", system.OSLinux},
		{"wabt", "arm64", "", system.OSWindows},
		{"wabt", "amd64", "wabt-1.0.42-windows-x64.tar.gz", system.OSWindows},
		{"craftlauncher", "arm64", "arm64", system.OSWindows},
		{"craftlauncher", "amd64", "x64", system.OSWindows},
		{"craftlauncher", "386", "", system.OSWindows},
	} {
		t.Run(tc.id+"/"+string(tc.os)+"/"+tc.arch, func(t *testing.T) {
			mock := &MockExecutor{}
			r := securityRunner(t, mock, tc.os)
			r.sysInfo.Arch = tc.arch
			asset, err := r.directAssetName(tc.id)
			if asset != tc.asset || (err != nil) != (tc.asset == "") {
				t.Fatalf("asset=%q err=%v", asset, err)
			}
			if tc.asset == "" && tc.os != system.OSWindows {
				err := r.installScript(tc.id, &catalog.Target{Type: catalog.TypeScript})
				if err == nil || len(mock.executed) != 0 {
					t.Fatalf("unsupported download attempted: %v, %v", err, mock.executed)
				}
			}
		})
	}
	r := securityRunner(t, &MockExecutor{}, system.OSLinux)
	r.sysInfo.Arch = "unknown"
	if _, err := r.adoptiumArch(); err == nil {
		t.Fatal("unknown architecture silently defaulted to x64")
	}
}

func TestSkillProvisioningCreatesDirectoriesOnlyWhenExecuting(t *testing.T) {
	r := securityRunner(t, &MockExecutor{}, system.OSLinux)
	root := t.TempDir()
	target := filepath.Join(root, "agent", "skills")
	r.sysInfo.SkillDirs = []string{target}
	r.dryRun = true
	if err := r.installGitSkill("test-skill", "owner/repo"); err != nil {
		t.Fatal(err)
	}
	if n := r.RelinkAllSkills(); n != 0 {
		t.Fatalf("dry-run linked %d skills", n)
	}
	if _, err := os.Stat(filepath.Dir(target)); !os.IsNotExist(err) {
		t.Fatal("dry-run created agent directories")
	}
	skill := filepath.Join(root, "test-skill")
	if err := os.Mkdir(skill, 0755); err != nil {
		t.Fatal(err)
	}
	n, err := LinkSkillToTargets(skill, []string{target})
	if _, statErr := os.Stat(target); statErr != nil {
		t.Fatalf("provisioning did not create directory: %v", statErr)
	}
	if err != nil {
		if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(1314)) {
			return
		}
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected one link, got %d", n)
	}
	if n, err := LinkSkillToTargets(skill, []string{target}); n != 0 || err != nil {
		t.Fatalf("linking not idempotent: %d, %v", n, err)
	}
}
