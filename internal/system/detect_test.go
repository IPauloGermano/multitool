package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOSRelease(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name       string
		content    string
		wantDistro LinuxDistro
	}{
		{
			name: "Ubuntu 24.04",
			content: `NAME="Ubuntu"
VERSION="24.04 LTS (Noble Numbat)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 24.04 LTS"
`,
			wantDistro: DistroUbuntu,
		},
		{
			name: "Debian_12",
			content: `PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
NAME="Debian GNU/Linux"
VERSION_ID="12"
VERSION="12 (bookworm)"
VERSION_CODENAME=bookworm
ID=debian
`,
			wantDistro: DistroDebian,
		},
		{
			name: "Fedora Linux 40",
			content: `NAME="Fedora Linux"
VERSION="40 (Container Image)"
ID=fedora
VERSION_ID=40
PRETTY_NAME="Fedora Linux 40 (Container Image)"
`,
			wantDistro: DistroFedora,
		},
		{
			name: "Arch Linux",
			content: `NAME="Arch Linux"
PRETTY_NAME="Arch Linux"
ID=arch
BUILD_ID=rolling
`,
			wantDistro: DistroArch,
		},
		{
			name: "openSUSE Tumbleweed",
			content: `NAME="openSUSE Tumbleweed"
ID="opensuse-tumbleweed"
ID_LIKE="opensuse suse"
PRETTY_NAME="openSUSE Tumbleweed"
`,
			wantDistro: DistroSuse,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tc.name+".release")
			if err := os.WriteFile(filePath, []byte(tc.content), 0644); err != nil {
				t.Fatalf("failed to write fixture: %v", err)
			}

			distro, _ := parseOSRelease(filePath)
			if distro != tc.wantDistro {
				t.Errorf("got distro %v, want %v", distro, tc.wantDistro)
			}
		})
	}
}

func TestGetSystemInfo(t *testing.T) {
	info := GetSystemInfo()
	if info.OS == "" {
		t.Errorf("Expected OS to be set, got empty")
	}
	if info.Arch == "" {
		t.Errorf("Expected Arch to be set, got empty")
	}
}

func TestAgentDetectionIsReadOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Setenv("PATH", "")
	for _, dir := range []string{
		filepath.Join(home, ".claude"), filepath.Join(home, ".gemini"),
		filepath.Join(home, ".codex"),
		filepath.Join(home, ".config", "opencode"), filepath.Join(os.Getenv("LOCALAPPDATA"), "opencode"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	previous := currentInfo
	currentInfo = nil
	t.Cleanup(func() { currentInfo = previous })
	targets := DetectAgentSkillTargets()
	if len(targets) != 5 {
		t.Fatalf("expected 5 agent targets, got %v", targets)
	}
	hasCodex := false
	hasAgents := false
	for _, target := range targets {
		if target == filepath.Join(home, ".codex", "skills") {
			hasCodex = true
		}
		if target == filepath.Join(home, ".agents", "skills") {
			hasAgents = true
		}
	}
	if !hasCodex || !hasAgents {
		t.Fatalf("expected both .codex/skills and .agents/skills in %v", targets)
	}
	_ = GetSystemInfo()
	for _, target := range targets {
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("detection created %s: %v", target, err)
		}
	}
}

func TestCodexDetectionVariants(t *testing.T) {
	for _, tc := range []struct {
		name, marker string
		wantCodex   bool
		wantAgents  bool
	}{
		{"codex configuration", ".codex", true, true},
		{"agent skills home", ".agents", false, true},
		{"not installed", "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("LOCALAPPDATA", filepath.Join(home, "local"))
			t.Setenv("PATH", "")
			if tc.marker != "" {
				if err := os.MkdirAll(filepath.Join(home, tc.marker), 0755); err != nil {
					t.Fatal(err)
				}
			}
			codexTarget := filepath.Join(home, ".codex", "skills")
			agentsTarget := filepath.Join(home, ".agents", "skills")
			foundCodex := false
			foundAgents := false
			for _, dir := range DetectAgentSkillTargets() {
				if dir == codexTarget {
					foundCodex = true
				}
				if dir == agentsTarget {
					foundAgents = true
				}
			}
			if foundCodex != tc.wantCodex {
				t.Fatalf("Codex target detection = %v, want %v", foundCodex, tc.wantCodex)
			}
			if foundAgents != tc.wantAgents {
				t.Fatalf("Agents target detection = %v, want %v", foundAgents, tc.wantAgents)
			}
			if _, err := os.Stat(codexTarget); !os.IsNotExist(err) {
				t.Fatalf("detection modified filesystem for codexTarget: %v", err)
			}
			if _, err := os.Stat(agentsTarget); !os.IsNotExist(err) {
				t.Fatalf("detection modified filesystem for agentsTarget: %v", err)
			}
		})
	}
}

func TestAntigravityDetectionVariants(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "local"))
	t.Setenv("PATH", "")

	// Create .gemini with config and antigravity subdirs
	for _, dir := range []string{
		filepath.Join(home, ".gemini"),
		filepath.Join(home, ".gemini", "config"),
		filepath.Join(home, ".gemini", "antigravity"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	targets := DetectAgentSkillTargets()
	wantTargets := []string{
		filepath.Join(home, ".gemini", "skills"),
		filepath.Join(home, ".gemini", "config", "skills"),
		filepath.Join(home, ".gemini", "antigravity", "skills"),
	}

	for _, want := range wantTargets {
		found := false
		for _, target := range targets {
			if target == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected Antigravity target %s in %v", want, targets)
		}
	}
}
