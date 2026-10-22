package system

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type OSType string

const (
	OSWindows OSType = "windows"
	OSLinux   OSType = "linux"
	OSDarwin  OSType = "darwin"
	OSUnknown OSType = "unknown"
)

type LinuxDistro string

const (
	DistroDebian  LinuxDistro = "debian"
	DistroUbuntu  LinuxDistro = "ubuntu"
	DistroFedora  LinuxDistro = "fedora"
	DistroRhel    LinuxDistro = "rhel"
	DistroArch    LinuxDistro = "arch"
	DistroCachy   LinuxDistro = "cachyos"
	DistroSuse    LinuxDistro = "suse"
	DistroSolus   LinuxDistro = "solus"
	DistroUnknown LinuxDistro = "unknown"
)

type PkgManager string

const (
	MgrWinget  PkgManager = "winget"
	MgrApt     PkgManager = "apt"
	MgrDnf     PkgManager = "dnf"
	MgrPacman  PkgManager = "pacman"
	MgrZypper  PkgManager = "zypper"
	MgrEopkg   PkgManager = "eopkg"
	MgrBrew    PkgManager = "brew"
	MgrNone    PkgManager = "none"
)

type SystemInfo struct {
	OS             OSType      `json:"os"`
	Arch           string      `json:"arch"`
	Distro         LinuxDistro `json:"distro,omitempty"`
	DistroPretty   string      `json:"distro_pretty,omitempty"`
	PrimaryManager PkgManager  `json:"primary_manager"`
	HasSudo        bool        `json:"has_sudo"`
	NeedsSudo      bool        `json:"needs_sudo"`
	AvailableTools map[string]bool `json:"available_tools"`
	SkillDirs      []string    `json:"skill_dirs"`
}

var currentInfo *SystemInfo

func GetSystemInfo() *SystemInfo {
	if currentInfo != nil {
		return currentInfo
	}

	info := &SystemInfo{
		OS:             CurrentOS(),
		Arch:           runtime.GOARCH,
		AvailableTools: make(map[string]bool),
	}

	// Check common CLI tools
	toolList := []string{
		"winget", "apt-get", "dnf", "pacman", "zypper", "eopkg", "brew",
		"npm", "pipx", "cargo", "git", "curl", "flatpak", "sudo", "docker", "podman",
	}
	for _, tool := range toolList {
		if _, err := exec.LookPath(tool); err == nil {
			info.AvailableTools[tool] = true
		}
	}

	if info.OS == OSWindows {
		info.PrimaryManager = MgrWinget
		info.NeedsSudo = false
		info.HasSudo = false
	} else if info.OS == OSLinux {
		info.Distro, info.DistroPretty = parseOSRelease("/etc/os-release")
		info.PrimaryManager = detectLinuxManager(info.Distro, info.AvailableTools)
		info.HasSudo = info.AvailableTools["sudo"]
		info.NeedsSudo = os.Geteuid() != 0 && info.HasSudo
	} else if info.OS == OSDarwin {
		if info.AvailableTools["brew"] {
			info.PrimaryManager = MgrBrew
		} else {
			info.PrimaryManager = MgrNone
		}
	}

	info.SkillDirs = DetectAgentSkillTargets()
	currentInfo = info
	return info
}

func CurrentOS() OSType {
	switch runtime.GOOS {
	case "windows":
		return OSWindows
	case "linux":
		return OSLinux
	case "darwin":
		return OSDarwin
	default:
		return OSUnknown
	}
}

func parseOSRelease(filePath string) (LinuxDistro, string) {
	file, err := os.Open(filePath)
	if err != nil {
		return DistroUnknown, "Linux"
	}
	defer file.Close()

	var id, idLike, prettyName string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
		} else if strings.HasPrefix(line, "ID_LIKE=") {
			idLike = strings.Trim(strings.TrimPrefix(line, "ID_LIKE="), `"`)
		} else if strings.HasPrefix(line, "PRETTY_NAME=") {
			prettyName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
		}
	}

	idLower := strings.ToLower(id)
	idLikeLower := strings.ToLower(idLike)

	var distro LinuxDistro
	switch {
	case idLower == "ubuntu" || strings.Contains(idLikeLower, "ubuntu"):
		distro = DistroUbuntu
	case idLower == "debian" || idLower == "deepin" || strings.Contains(idLikeLower, "debian"):
		distro = DistroDebian
	case idLower == "fedora":
		distro = DistroFedora
	case idLower == "rhel" || idLower == "centos" || idLower == "almalinux" || idLower == "rocky" || strings.Contains(idLikeLower, "rhel"):
		distro = DistroRhel
	case idLower == "cachyos":
		distro = DistroCachy
	case idLower == "arch" || idLower == "artix" || idLower == "manjaro" || strings.Contains(idLikeLower, "arch"):
		distro = DistroArch
	case strings.HasPrefix(idLower, "opensuse") || idLower == "sles" || strings.Contains(idLikeLower, "suse"):
		distro = DistroSuse
	case idLower == "solus":
		distro = DistroSolus
	default:
		distro = DistroUnknown
	}

	if prettyName == "" {
		prettyName = string(distro)
	}
	return distro, prettyName
}

func detectLinuxManager(distro LinuxDistro, available map[string]bool) PkgManager {
	switch distro {
	case DistroUbuntu, DistroDebian:
		if available["apt-get"] {
			return MgrApt
		}
	case DistroFedora, DistroRhel:
		if available["dnf"] {
			return MgrDnf
		}
	case DistroArch, DistroCachy:
		if available["pacman"] {
			return MgrPacman
		}
	case DistroSuse:
		if available["zypper"] {
			return MgrZypper
		}
	case DistroSolus:
		if available["eopkg"] {
			return MgrEopkg
		}
	}

	// Fallback por verificação de binários presentes
	if available["apt-get"] {
		return MgrApt
	}
	if available["dnf"] {
		return MgrDnf
	}
	if available["pacman"] {
		return MgrPacman
	}
	if available["zypper"] {
		return MgrZypper
	}
	if available["eopkg"] {
		return MgrEopkg
	}
	return MgrNone
}

func DetectAgentSkillTargets() []string {
	var targets []string
	home, err := os.UserHomeDir()
	if err != nil {
		return targets
	}

	// 1. Claude Code
	claudeDir := filepath.Join(home, ".claude")
	if fi, err := os.Stat(claudeDir); err == nil && fi.IsDir() {
		target := filepath.Join(claudeDir, "skills")
		_ = os.MkdirAll(target, 0755)
		targets = append(targets, target)
	}

	// 2. OpenCode
	var openCodeDir string
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			openCodeDir = filepath.Join(localAppData, "opencode")
		}
	}
	if openCodeDir == "" {
		openCodeDir = filepath.Join(home, ".config", "opencode")
	}
	if fi, err := os.Stat(openCodeDir); err == nil && fi.IsDir() {
		target := filepath.Join(openCodeDir, "skills")
		_ = os.MkdirAll(target, 0755)
		targets = append(targets, target)
	}

	// 3. Gemini CLI / Antigravity
	geminiDir := filepath.Join(home, ".gemini")
	if fi, err := os.Stat(geminiDir); err == nil && fi.IsDir() {
		target := filepath.Join(geminiDir, "skills")
		_ = os.MkdirAll(target, 0755)
		targets = append(targets, target)
	}

	return targets
}
