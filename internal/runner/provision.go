package runner

import (
	"fmt"
	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/system"
)

type runtimePkg struct {
	command  string
	apt      []string
	dnf      []string
	pacman   []string
	zypper   []string
	eopkg    []string
	brew     []string
	wingetID string
}

func (r *Runner) ensureRuntime(toolID string, target *catalog.Target) error {
	var req *runtimePkg

	switch target.Type {
	case catalog.TypeNpm:
		req = &runtimePkg{
			command:  "npm",
			apt:      []string{"nodejs", "npm"},
			dnf:      []string{"nodejs", "npm"},
			pacman:   []string{"nodejs", "npm"},
			zypper:   []string{"nodejs", "npm"},
			eopkg:    []string{"nodejs"},
			brew:     []string{"node"},
			wingetID: "OpenJS.NodeJS.LTS",
		}
	case catalog.TypePipx:
		req = &runtimePkg{
			command:  "pipx",
			apt:      []string{"pipx"},
			dnf:      []string{"pipx"},
			pacman:   []string{"python-pipx"},
			zypper:   []string{"python3-pipx"},
			eopkg:    []string{"pipx"},
			brew:     []string{"pipx"},
			wingetID: "Python.Pipx",
		}
	case catalog.TypeCargo:
		req = &runtimePkg{
			command:  "cargo",
			apt:      []string{"cargo"},
			dnf:      []string{"cargo"},
			pacman:   []string{"rust"},
			zypper:   []string{"cargo"},
			eopkg:    []string{"rust"},
			brew:     []string{"rust"},
			wingetID: "Rustlang.Rustup",
		}
	case catalog.TypeFlatpak:
		req = &runtimePkg{
			command: "flatpak",
			apt:     []string{"flatpak"},
			dnf:     []string{"flatpak"},
			pacman:  []string{"flatpak"},
			zypper:  []string{"flatpak"},
			eopkg:   []string{"flatpak"},
		}
	case catalog.TypeSkill:
		req = &runtimePkg{
			command:  "git",
			apt:      []string{"git"},
			dnf:      []string{"git"},
			pacman:   []string{"git"},
			zypper:   []string{"git"},
			eopkg:    []string{"git"},
			brew:     []string{"git"},
			wingetID: "Git.Git",
		}
	case catalog.TypeDirect:
		if toolID != "orca" && toolID != "openmausbot" && toolID != "apktool" && toolID != "ghidra" && toolID != "craftlauncher" && toolID != "wabt" {
			req = &runtimePkg{
				command:  "git",
				apt:      []string{"git"},
				dnf:      []string{"git"},
				pacman:   []string{"git"},
				zypper:   []string{"git"},
				eopkg:    []string{"git"},
				brew:     []string{"git"},
				wingetID: "Git.Git",
			}
		}
	case catalog.TypeScript:
		if toolID == "opengym" || toolID == "openstock" || toolID == "openmaicapp" {
			req = &runtimePkg{
				command:  "git",
				apt:      []string{"git"},
				dnf:      []string{"git"},
				pacman:   []string{"git"},
				zypper:   []string{"git"},
				eopkg:    []string{"git"},
				brew:     []string{"git"},
				wingetID: "Git.Git",
			}
		} else if toolID == "ghidra" && !r.haveCommand("unzip") && !r.haveCommand("python3") {
			req = &runtimePkg{
				command: "unzip",
				apt:     []string{"unzip"},
				dnf:     []string{"unzip"},
				pacman:  []string{"unzip"},
				zypper:  []string{"unzip"},
				eopkg:   []string{"unzip"},
				brew:    []string{"unzip"},
			}
		}
	}

	if req == nil {
		return nil
	}

	if r.haveCommand(req.command) {
		return nil
	}

	fmt.Printf("➜ Runtime '%s' ausente. Auto-provisionando via gerenciador do sistema...\n", req.command)
	LogOp("runtime auto-provision %s inicio", req.command)

	var err error
	if r.sysInfo.OS == system.OSWindows {
		if req.wingetID == "" {
			return fmt.Errorf("runtime '%s' não encontrado e não é suportado no Windows", req.command)
		}
		err = r.executor.Execute("winget", "install", "--exact", "--id", req.wingetID,
			"--accept-source-agreements", "--accept-package-agreements", "--silent")
		if isWingetAlreadyInstalled(err) || isWingetRebootRequired(err) {
			err = nil
		}
	} else if r.sysInfo.OS == system.OSDarwin {
		if len(req.brew) == 0 {
			return fmt.Errorf("runtime '%s' não encontrado para macOS", req.command)
		}
		args := append([]string{"install"}, req.brew...)
		err = r.executor.Execute("brew", args...)
	} else if r.sysInfo.OS == system.OSLinux {
		var pkgs []string
		switch r.sysInfo.PrimaryManager {
		case system.MgrApt:
			pkgs = req.apt
			if len(pkgs) > 0 {
				args := append([]string{"install", "-y"}, pkgs...)
				err = r.execWithSudoIfNeeded("apt-get", args...)
			}
		case system.MgrDnf:
			pkgs = req.dnf
			if len(pkgs) > 0 {
				args := append([]string{"install", "-y"}, pkgs...)
				err = r.execWithSudoIfNeeded("dnf", args...)
			}
		case system.MgrPacman:
			pkgs = req.pacman
			if len(pkgs) > 0 {
				args := append([]string{"-S", "--noconfirm", "--needed"}, pkgs...)
				err = r.execWithSudoIfNeeded("pacman", args...)
			}
		case system.MgrZypper:
			pkgs = req.zypper
			if len(pkgs) > 0 {
				args := append([]string{"install", "-y"}, pkgs...)
				err = r.execWithSudoIfNeeded("zypper", args...)
			}
		case system.MgrEopkg:
			pkgs = req.eopkg
			if len(pkgs) > 0 {
				args := append([]string{"install", "-y"}, pkgs...)
				err = r.execWithSudoIfNeeded("eopkg", args...)
			}
		case system.MgrBrew:
			pkgs = req.brew
			if len(pkgs) > 0 {
				args := append([]string{"install"}, pkgs...)
				err = r.executor.Execute("brew", args...)
			}
		default:
			return fmt.Errorf("gerenciador de pacotes nativo não suportado para instalar runtime '%s'", req.command)
		}
	} else {
		return fmt.Errorf("sistema operacional não suportado para auto-provisionamento de runtime '%s'", req.command)
	}

	if err != nil {
		LogOp("runtime auto-provision %s fim rc=1 erro=%v", req.command, err)
		return fmt.Errorf("falha ao auto-provisionar runtime '%s': %w", req.command, err)
	}

	if req.command == "flatpak" && r.sysInfo.OS == system.OSLinux {
		_ = r.executor.Execute("flatpak", "--noninteractive", "remote-add", "--if-not-exists", "flathub", "https://dl.flathub.org/repo/flathub.flatpakrepo")
	}

	fmt.Printf("✔ Runtime '%s' provisionado com sucesso.\n", req.command)
	LogOp("runtime auto-provision %s fim rc=0", req.command)
	r.cacheMu.Lock()
	if r.cmdCache == nil {
		r.cmdCache = make(map[string]bool)
	}
	r.cmdCache[req.command] = true
	r.cacheMu.Unlock()
	return nil
}
