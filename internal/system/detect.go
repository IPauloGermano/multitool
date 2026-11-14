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
	MgrWinget PkgManager = "winget"
	MgrApt    PkgManager = "apt"
	MgrDnf    PkgManager = "dnf"
	MgrPacman PkgManager = "pacman"
	MgrZypper PkgManager = "zypper"
	MgrEopkg  PkgManager = "eopkg"
	MgrBrew   PkgManager = "brew"
	MgrNone   PkgManager = "none"
)

type SystemInfo struct {
	OS             OSType          `json:"os"`
	Arch           string          `json:"arch"`
	Distro         LinuxDistro     `json:"distro,omitempty"`
	DistroPretty   string          `json:"distro_pretty,omitempty"`
	PrimaryManager PkgManager      `json:"primary_manager"`
	HasSudo        bool            `json:"has_sudo"`
	NeedsSudo      bool            `json:"needs_sudo"`
	AvailableTools map[string]bool `json:"available_tools"`
	SkillDirs      []string        `json:"skill_dirs"`
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

	seen := make(map[string]bool)
	addTarget := func(t string) {
		clean := filepath.Clean(t)
		key := clean
		if runtime.GOOS == "windows" {
			key = strings.ToLower(clean)
		}
		if !seen[key] {
			seen[key] = true
			targets = append(targets, clean)
		}
	}

	// 1. Claude Code
	claudeDir := filepath.Join(home, ".claude")
	if fi, err := os.Stat(claudeDir); err == nil && fi.IsDir() {
		addTarget(filepath.Join(claudeDir, "skills"))
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
		addTarget(filepath.Join(openCodeDir, "skills"))
	}

	// 3. Gemini CLI / Antigravity
	geminiDir := filepath.Join(home, ".gemini")
	hasGemini := false
	if fi, err := os.Stat(geminiDir); err == nil && fi.IsDir() {
		hasGemini = true
	}
	hasAntigravity := false
	if _, err := exec.LookPath("agy"); err == nil {
		hasAntigravity = true
	} else if _, err := exec.LookPath("antigravity"); err == nil {
		hasAntigravity = true
	} else if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			if fi, err := os.Stat(filepath.Join(localAppData, "Programs", "antigravity")); err == nil && fi.IsDir() {
				hasAntigravity = true
			} else if fi, err := os.Stat(filepath.Join(localAppData, "agy")); err == nil && fi.IsDir() {
				hasAntigravity = true
			}
		}
	}

	if hasGemini || hasAntigravity {
		// Diretório padrão Gemini CLI
		addTarget(filepath.Join(geminiDir, "skills"))

		// Antigravity global customization discovery root (~/.gemini/config/skills)
		configDir := filepath.Join(geminiDir, "config")
		if fi, err := os.Stat(configDir); err == nil && fi.IsDir() {
			addTarget(filepath.Join(configDir, "skills"))
		} else if hasAntigravity {
			addTarget(filepath.Join(configDir, "skills"))
		}

		// Antigravity app directory (~/.gemini/antigravity/skills)
		antigravityDir := filepath.Join(geminiDir, "antigravity")
		if fi, err := os.Stat(antigravityDir); err == nil && fi.IsDir() {
			addTarget(filepath.Join(antigravityDir, "skills"))
		}
	}

	// 4. Codex e Agent Skills
	codexDir := filepath.Join(home, ".codex")
	agentsDir := filepath.Join(home, ".agents")
	hasCodex := false
	if fi, err := os.Stat(codexDir); err == nil && fi.IsDir() {
		hasCodex = true
	} else if _, err := exec.LookPath("codex"); err == nil {
		hasCodex = true
	}

	hasAgents := false
	if fi, err := os.Stat(agentsDir); err == nil && fi.IsDir() {
		hasAgents = true
	}

	if hasCodex {
		// Codex carrega prioritariamente de ~/.codex/skills
		addTarget(filepath.Join(codexDir, "skills"))
		// E também compartilha o padrão aberto ~/.agents/skills
		addTarget(filepath.Join(agentsDir, "skills"))
	} else if hasAgents {
		addTarget(filepath.Join(agentsDir, "skills"))
	}

	return targets
}
