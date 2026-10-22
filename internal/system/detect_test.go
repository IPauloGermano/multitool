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
