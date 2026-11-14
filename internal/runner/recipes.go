package runner

import (
	"fmt"
	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/system"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// -------------------------------------------------------------
// AGENT SKILLS LINKING
// -------------------------------------------------------------

func cleanTargetPath(p string) string {
	p = strings.TrimPrefix(p, `\\?\`)
	p = strings.TrimPrefix(p, `\??\`)
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return filepath.Clean(p)
}

// FindSkillsInDir localiza todos os diretórios com SKILL.md dentro de um repositório clonado.
func FindSkillsInDir(skillDir string) map[string]string {
	results := make(map[string]string)
	baseName := filepath.Base(skillDir)

	// 1. Skill direta no topo do repositório
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err == nil {
		results[baseName] = skillDir
		return results
	}

	ignoreDirs := map[string]bool{
		".git": true, "node_modules": true, "tests": true, "fixtures": true,
		"demos": true, "docs": true, ".github": true, ".cargo": true, ".vscode": true,
	}

	// 2. Diretórios canônicos padrão de coleções de skills
	candidates := []string{
		filepath.Join(skillDir, "skills"),
		filepath.Join(skillDir, ".agents", "skills"),
		filepath.Join(skillDir, ".claude", "skills"),
		filepath.Join(skillDir, ".openclaw", "skills"),
	}

	foundInCandidates := false
	for _, cand := range candidates {
		if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
			_ = filepath.Walk(cand, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() && ignoreDirs[info.Name()] {
					return filepath.SkipDir
				}
				if !info.IsDir() && info.Name() == "SKILL.md" {
					dir := filepath.Dir(path)
					name := filepath.Base(dir)
					if _, exists := results[name]; !exists {
						results[name] = dir
						foundInCandidates = true
					}
				}
				return nil
			})
		}
	}

	// 3. Fallback: varredura em subpastas caso não esteja nas pastas padrão
	if !foundInCandidates {
		_ = filepath.Walk(skillDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() && (ignoreDirs[info.Name()] || strings.HasPrefix(info.Name(), ".")) {
				return filepath.SkipDir
			}
			if !info.IsDir() && info.Name() == "SKILL.md" {
				dir := filepath.Dir(path)
				name := filepath.Base(dir)
				if _, exists := results[name]; !exists {
					results[name] = dir
				}
			}
			return nil
		})
	}

	// 4. Se o repositório possuir 1 única skill interna, mapeia também o ID/baseName do pacote
	if _, exists := results[baseName]; !exists {
		if len(results) == 1 {
			for _, dir := range results {
				results[baseName] = dir
				break
			}
		} else if len(results) == 0 {
			results[baseName] = skillDir
		} else {
			results[baseName] = skillDir
		}
	}

	return results
}

func LinkSkillToTargets(skillDir string, targets []string) (int, error) {
	linked := 0
	skills := FindSkillsInDir(skillDir)

	for _, targetDir := range targets {
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return linked, fmt.Errorf("criar diretório de skills %s: %w", targetDir, err)
		}

		for skillName, sDir := range skills {
			dest := filepath.Join(targetDir, skillName)
			if fi, err := os.Lstat(dest); err == nil {
				resolved, rerr := os.Readlink(dest)
				if rerr == nil {
					resClean := cleanTargetPath(resolved)
					sClean := cleanTargetPath(sDir)
					if !filepath.IsAbs(resClean) {
						resClean = cleanTargetPath(filepath.Join(filepath.Dir(dest), resClean))
					}
					matches := resClean == sClean
					if runtime.GOOS == "windows" {
						matches = strings.EqualFold(resClean, sClean)
					}
					if matches {
						continue
					}
					_ = os.Remove(dest)
				} else if fi.IsDir() {
					continue
				}
			}

			if err := os.Symlink(sDir, dest); err != nil {
				if fallbackErr := createSkillLinkFallback(sDir, dest, err); fallbackErr != nil {
					return linked, fmt.Errorf("criar link de skill %s: %w", dest, fallbackErr)
				}
			}
			linked++
		}
	}
	return linked, nil
}

func UnlinkSkillFromTargets(skillName string, targets []string) {
	for _, targetDir := range targets {
		dest := filepath.Join(targetDir, skillName)
		_ = os.Remove(dest)
	}
}

func (r *Runner) installNative(packageID string) error {
	return r.installNativeWithActivity(nil, packageID)
}

func (r *Runner) installNativeWithActivity(actFn func(string), packageID string) error {
	switch r.sysInfo.PrimaryManager {
	case system.MgrApt:
		return r.execWithSudoAndActivity(actFn, "apt-get", "install", "-y", packageID)
	case system.MgrDnf:
		return r.execWithSudoAndActivity(actFn, "dnf", "install", "-y", packageID)
	case system.MgrPacman:
		return r.execWithSudoAndActivity(actFn, "pacman", "-S", "--noconfirm", "--needed", packageID)
	case system.MgrZypper:
		return r.execWithSudoAndActivity(actFn, "zypper", "install", "-y", packageID)
	case system.MgrEopkg:
		return r.execWithSudoAndActivity(actFn, "eopkg", "install", "-y", packageID)
	case system.MgrBrew:
		return r.execWithActivity(actFn, "brew", "install", packageID)
	default:
		return fmt.Errorf("nenhum gerenciador nativo suportado encontrado para instalar %s", packageID)
	}
}

func (r *Runner) uninstallNative(packageID string) error {
	return r.uninstallNativeWithActivity(nil, packageID)
}

func (r *Runner) uninstallNativeWithActivity(actFn func(string), packageID string) error {
	switch r.sysInfo.PrimaryManager {
	case system.MgrApt:
		return r.execWithSudoAndActivity(actFn, "apt-get", "remove", "-y", packageID)
	case system.MgrDnf:
		return r.execWithSudoAndActivity(actFn, "dnf", "remove", "-y", packageID)
	case system.MgrPacman:
		return r.execWithSudoAndActivity(actFn, "pacman", "-R", "--noconfirm", packageID)
	case system.MgrZypper:
		return r.execWithSudoAndActivity(actFn, "zypper", "remove", "-y", packageID)
	case system.MgrEopkg:
		return r.execWithSudoAndActivity(actFn, "eopkg", "remove", "-y", packageID)
	case system.MgrBrew:
		return r.execWithActivity(actFn, "brew", "uninstall", packageID)
	default:
		return fmt.Errorf("nenhum gerenciador nativo suportado encontrado para remover %s", packageID)
	}
}

func (r *Runner) updateNative(packageID string) error {
	return r.updateNativeWithActivity(nil, packageID)
}

func (r *Runner) updateNativeWithActivity(actFn func(string), packageID string) error {
	switch r.sysInfo.PrimaryManager {
	case system.MgrApt:
		return r.execWithSudoAndActivity(actFn, "apt-get", "install", "--only-upgrade", "-y", packageID)
	case system.MgrDnf:
		return r.execWithSudoAndActivity(actFn, "dnf", "upgrade", "-y", packageID)
	case system.MgrPacman:
		return r.execWithSudoAndActivity(actFn, "pacman", "-S", "--noconfirm", packageID)
	case system.MgrZypper:
		return r.execWithSudoAndActivity(actFn, "zypper", "update", "-y", packageID)
	case system.MgrEopkg:
		return r.execWithSudoAndActivity(actFn, "eopkg", "upgrade", "-y", packageID)
	case system.MgrBrew:
		return r.execWithActivity(actFn, "brew", "upgrade", packageID)
	default:
		return fmt.Errorf("nenhum gerenciador nativo suportado encontrado para atualizar %s", packageID)
	}
}

func (r *Runner) installScript(id string, target *catalog.Target) error {
	return r.installScriptWithActivity(nil, id, target)
}

func (r *Runner) installScriptWithActivity(actFn func(string), id string, target *catalog.Target) error {
	if r.dryRun {
		return nil
	}
	if id == "flathub" {
		if !r.haveCommand("flatpak") {
			return fmt.Errorf("flatpak não encontrado no sistema para configurar flathub")
		}
		return r.execWithActivity(actFn, "flatpak", "--noninteractive", "remote-add", "--if-not-exists", "flathub", "https://dl.flathub.org/repo/flathub.flatpakrepo")
	}
	if id == "opengym" || id == "openstock" || id == "openmaicapp" || id == "craftlauncher" {
		if r.sysInfo.OS == system.OSLinux {
			if target.SHA256 != "" || len(target.ArtifactSHA256) > 0 {
				return fmt.Errorf("checksum de artefato não se aplica a clone git: %s", id)
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			base := filepath.Join(home, ".local", "share", "apps")
			if err := os.MkdirAll(base, 0755); err != nil {
				return err
			}
			return r.execWithActivity(actFn, "git", "clone", "--depth", "1", "https://github.com/"+target.PackageID, filepath.Join(base, id))
		}
	}
	if r.sysInfo.OS != system.OSLinux && !(id == "gbrain" && r.sysInfo.OS == system.OSDarwin) {
		return fmt.Errorf("script de instalação direta não implementado para: %s em %s", id, r.sysInfo.OS)
	}
	dir, err := os.MkdirTemp("", "multitool-download-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	download := func(name, url string) (string, error) {
		return r.downloadArtifact(actFn, target, dir, name, url)
	}
	runScript := func(shell, url string, args ...string) error {
		path, err := download(id+"-install.sh", url)
		if err != nil {
			return err
		}
		return r.execWithActivity(actFn, shell, append([]string{path}, args...)...)
	}
	installBinary := func(url string) error {
		path, err := download(filepath.Base(url), url)
		if err != nil {
			return err
		}
		return r.execWithActivity(actFn, "sh", "-c", `mkdir -p "$HOME/.local/bin" && cp "$1" "$HOME/.local/bin/$2" && chmod +x "$HOME/.local/bin/$2"`, "multitool", path, id)
	}
	switch id {
	case "ollama":
		return runScript("sh", "https://ollama.com/install.sh")
	case "hermes":
		return runScript("bash", "https://hermes-agent.nousresearch.com/install.sh")
	case "rustup":
		return runScript("sh", "https://sh.rustup.rs", "-y")
	case "uv":
		return runScript("sh", "https://astral.sh/uv/install.sh")
	case "docker":
		return runScript("sh", "https://get.docker.com")
	case "zed":
		return runScript("sh", "https://zed.dev/install.sh")
	case "tailscale":
		return runScript("sh", "https://tailscale.com/install.sh")
	case "goose":
		return runScript("bash", "https://github.com/block/goose/releases/download/stable/download_cli.sh")
	case "trivy":
		path, err := download("trivy-install.sh", "https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh")
		if err != nil {
			return err
		}
		return r.execWithActivity(actFn, "sh", "-c", `mkdir -p "$HOME/.local/bin" && sh "$1" -b "$HOME/.local/bin"`, "multitool", path)
	case "ohmyzsh":
		path, err := download("ohmyzsh-install.sh", "https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh")
		if err != nil {
			return err
		}
		return r.execWithActivity(actFn, "sh", "-c", `RUNZSH=no CHSH=no KEEP_ZSHRC=yes sh "$1"`, "multitool", path)
	case "orca":
		asset, err := r.directAssetName(id)
		if err != nil {
			return err
		}
		return installBinary("https://github.com/stablyai/orca/releases/latest/download/" + asset)
	case "openmausbot":
		asset, err := r.directAssetName(id)
		if err != nil {
			return err
		}
		return installBinary("https://github.com/milind-soni/OpenMausBot/releases/latest/download/" + asset)
	case "gbrain":
		asset, err := r.directAssetName(id)
		if err != nil {
			return err
		}
		return installBinary("https://github.com/garrytan/gbrain/releases/latest/download/" + asset)
	case "cherrystudio":
		asset, err := r.directAssetName(id)
		if err != nil {
			return err
		}
		return installBinary("https://github.com/CherryHQ/cherry-studio/releases/download/v2.2.2/" + asset)
	case "temurin":
		arch, err := r.adoptiumArch()
		if err != nil {
			return err
		}
		url := fmt.Sprintf("https://api.adoptium.net/v3/binary/latest/21/ga/linux/%s/jdk/hotspot/normal/eclipse", arch)
		path, err := download("temurin.tar.gz", url)
		if err != nil {
			return err
		}
		return r.execWithActivity(actFn, "sh", "-c", `mkdir -p "$HOME/.local/share/adoptium" && tar -xzf "$1" -C "$HOME/.local/share/adoptium" --strip-components=1 && mkdir -p "$HOME/.local/bin" && ln -sf "$HOME/.local/share/adoptium/bin/java" "$HOME/.local/bin/java"`, "multitool", path)
	case "ghidra":
		path, err := download("ghidra.zip", "https://github.com/NationalSecurityAgency/ghidra/releases/download/Ghidra_11.1.2_build/ghidra_11.1.2_PUBLIC_20240709.zip")
		if err != nil {
			return err
		}
		cmd := `if command -v unzip >/dev/null 2>&1; then unzip -q "$1" -d "$2"; ` +
			`elif command -v python3 >/dev/null 2>&1; then python3 -m zipfile -e "$1" "$2"; ` +
			`elif [ -x "$HOME/.local/share/adoptium/bin/jar" ]; then (cd "$2" && "$HOME/.local/share/adoptium/bin/jar" xf "$1"); ` +
			`else (cd "$2" && jar xf "$1"); fi && ` +
			`mkdir -p "$HOME/.local/share/ghidra" "$HOME/.local/bin" && mv "$2"/ghidra_*/* "$HOME/.local/share/ghidra" && chmod +x "$HOME/.local/share/ghidra/ghidraRun" && ln -sf "$HOME/.local/share/ghidra/ghidraRun" "$HOME/.local/bin/ghidra"`
		return r.execWithActivity(actFn, "sh", "-c", cmd, "multitool", path, dir)
	}
	return fmt.Errorf("script de instalação direta não implementado para: %s", id)
}

// directAssetName lista somente combinações publicadas pelos projetos upstream.
func (r *Runner) directAssetName(id string) (string, error) {
	arch := r.sysInfo.Arch
	if arch == "" {
		arch = runtime.GOARCH
	}
	platform := string(r.sysInfo.OS) + "/" + arch
	assets := map[string]map[string]string{
		"gbrain":        {"linux/amd64": "gbrain-linux-x64", "darwin/arm64": "gbrain-darwin-arm64"},
		"orca":          {"linux/amd64": "orca-linux.AppImage", "linux/arm64": "orca-linux-arm64.AppImage", "windows/amd64": "orca-windows-setup.exe"},
		"openmausbot":   {"linux/amd64": "OpenMausBot.AppImage", "windows/amd64": "OpenMausBot-setup.exe"},
		"cherrystudio":  {"linux/amd64": "Cherry-Studio-2.2.2-linux-x64.AppImage", "linux/arm64": "Cherry-Studio-2.2.2-linux-arm64.AppImage"},
		"wabt":          {"windows/amd64": "wabt-1.0.42-windows-x64.tar.gz"},
		"craftlauncher": {"windows/amd64": "x64", "windows/arm64": "arm64"},
	}
	if asset := assets[id][platform]; asset != "" {
		return asset, nil
	}
	return "", fmt.Errorf("%s não possui artefato suportado para %s", id, platform)
}

func (r *Runner) adoptiumArch() (string, error) {
	arch := r.sysInfo.Arch
	if arch == "" {
		arch = runtime.GOARCH
	}
	switch arch {
	case "amd64":
		return "x64", nil
	case "arm64":
		return "aarch64", nil
	case "arm":
		return "arm", nil
	default:
		return "", fmt.Errorf("temurin não possui artefato suportado para %s/%s", r.sysInfo.OS, arch)
	}
}

// uninstallScript remove de forma limpa binários, symlinks e diretórios criados por scripts diretos no Linux.
func (r *Runner) uninstallScript(tool catalog.Tool, target *catalog.Target) error {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}

	switch tool.ID {
	case "temurin":
		if home != "" {
			_ = os.RemoveAll(filepath.Join(home, ".local", "share", "adoptium"))
			_ = os.Remove(filepath.Join(home, ".local", "bin", "java"))
		}

	case "ghidra":
		if home != "" {
			_ = os.RemoveAll(filepath.Join(home, ".local", "share", "ghidra"))
			_ = os.Remove(filepath.Join(home, ".local", "bin", "ghidra"))
		}

	case "uv":
		if home != "" {
			_ = os.Remove(filepath.Join(home, ".local", "bin", "uv"))
			_ = os.Remove(filepath.Join(home, ".local", "bin", "uvx"))
			_ = os.Remove(filepath.Join(home, ".cargo", "bin", "uv"))
			_ = os.Remove(filepath.Join(home, ".cargo", "bin", "uvx"))
			_ = os.RemoveAll(filepath.Join(home, ".local", "share", "uv"))
		}

	case "rustup":
		command := "rustup"
		if r.haveCommand("rustup") {
			// O desinstalador oficial é responsável pelos seus próprios arquivos.
		} else if home != "" {
			cargoRustup := filepath.Join(home, ".cargo", "bin", "rustup")
			if runtime.GOOS == "windows" {
				cargoRustup += ".exe"
			}
			if _, err := os.Stat(cargoRustup); err == nil {
				command = cargoRustup
			} else {
				return fmt.Errorf("desinstalador rustup não encontrado: %w", err)
			}
		} else {
			return fmt.Errorf("desinstalador rustup não encontrado")
		}
		if err := r.executor.Execute(command, "self", "uninstall", "-y"); err != nil {
			return fmt.Errorf("falha ao desinstalar rustup via %s: %w", command, err)
		}

	case "ollama":
		if r.haveCommand("systemctl") {
			_ = r.execWithSudoIfNeeded("systemctl", "stop", "ollama")
			_ = r.execWithSudoIfNeeded("systemctl", "disable", "ollama")
			_ = r.execWithSudoIfNeeded("rm", "-f", "/etc/systemd/system/ollama.service")
		}
		_ = r.execWithSudoIfNeeded("rm", "-f", "/usr/local/bin/ollama", "/usr/bin/ollama")
		_ = r.execWithSudoIfNeeded("rm", "-rf", "/usr/share/ollama")
		if home != "" {
			_ = os.Remove(filepath.Join(home, ".local", "bin", "ollama"))
		}

	case "hermes":
		if home != "" {
			_ = os.Remove(filepath.Join(home, ".local", "bin", "hermes"))
			_ = os.RemoveAll(filepath.Join(home, ".hermes"))
			_ = os.RemoveAll(filepath.Join(home, ".local", "share", "hermes"))
		}

	case "orca":
		if home != "" {
			_ = os.Remove(filepath.Join(home, ".local", "bin", "orca"))
			_ = os.RemoveAll(filepath.Join(home, ".local", "share", "orca"))
			_ = os.RemoveAll(filepath.Join(home, ".config", "orca"))
		}

	case "openmausbot":
		if home != "" {
			_ = os.Remove(filepath.Join(home, ".local", "bin", "openmausbot"))
		}

	case "gbrain":
		if home != "" {
			_ = os.Remove(filepath.Join(home, ".local", "bin", "gbrain"))
			_ = os.RemoveAll(filepath.Join(home, ".gbrain"))
		}

	case "opengym", "openstock", "openmaicapp", "craftlauncher":
		if home != "" {
			_ = os.RemoveAll(filepath.Join(home, ".local", "share", "apps", tool.ID))
		}

	case "ohmyzsh":
		if home != "" {
			uninstaller := filepath.Join(home, ".oh-my-zsh", "tools", "uninstall.sh")
			if _, err := os.Stat(uninstaller); err == nil {
				_ = r.executor.Execute("sh", uninstaller)
			} else {
				_ = os.RemoveAll(filepath.Join(home, ".oh-my-zsh"))
			}
		}

	case "zed":
		if home != "" {
			_ = os.Remove(filepath.Join(home, ".local", "bin", "zed"))
			_ = os.RemoveAll(filepath.Join(home, ".local", "share", "zed"))
		}

	case "trivy":
		if home != "" {
			_ = os.Remove(filepath.Join(home, ".local", "bin", "trivy"))
		}

	case "goose":
		if home != "" {
			_ = os.Remove(filepath.Join(home, ".local", "bin", "goose"))
			_ = os.RemoveAll(filepath.Join(home, ".config", "goose"))
		}

	case "cherrystudio":
		if home != "" {
			_ = os.Remove(filepath.Join(home, ".local", "bin", "cherrystudio"))
		}

	case "flathub":
		if r.haveCommand("flatpak") {
			_ = r.execWithTimeout(15*time.Second, "flatpak", "--noninteractive", "remote-delete", "--force", "flathub")
			_ = r.execWithTimeout(15*time.Second, "flatpak", "--user", "--noninteractive", "remote-delete", "--force", "flathub")
		}

	default:
		// Limpeza preventiva de binários e symlinks em ~/.local/bin para qualquer outra ferramenta script
		if home != "" {
			if target != nil && target.Binary != "" {
				_ = os.Remove(filepath.Join(home, ".local", "bin", target.Binary))
			}
			_ = os.Remove(filepath.Join(home, ".local", "bin", tool.ID))
		}
	}

	return nil
}

func (r *Runner) getSkillDir(id string) string {
	if r.sysInfo.OS == system.OSWindows {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, "multitool", "skills", id)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "multitool", "skills", id)
}

func (r *Runner) installGitSkill(id, packageID string) error {
	return r.installGitSkillWithActivity(nil, id, packageID)
}

func (r *Runner) installDirectWindows(toolID string) error {
	return r.installDirectWindowsWithActivity(nil, toolID)
}

func (r *Runner) installDirectWindowsWithActivity(actFn func(string), toolID string) error {
	tool := catalog.FindTool(toolID)
	if tool == nil || tool.Windows == nil {
		return fmt.Errorf("alvo Windows não encontrado: %s", toolID)
	}
	return r.installDirectWindowsTargetWithActivity(actFn, toolID, tool.Windows)
}

func (r *Runner) installDirectWindowsTargetWithActivity(actFn func(string), toolID string, target *catalog.Target) error {
	if r.dryRun {
		return nil
	}
	dir, err := os.MkdirTemp("", "multitool-download-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	download := func(name, url string) (string, error) {
		return r.downloadArtifact(actFn, target, dir, name, url)
	}
	localApp := os.Getenv("LOCALAPPDATA")
	if localApp == "" {
		home, _ := os.UserHomeDir()
		localApp = filepath.Join(home, "AppData", "Local")
	}
	binDir := filepath.Join(localApp, "multitool", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	switch toolID {
	case "ghidra":
		ghidraDir := filepath.Join(localApp, "multitool", "ghidra")
		if err := os.MkdirAll(ghidraDir, 0755); err != nil {
			return err
		}

		zipURL := "https://github.com/NationalSecurityAgency/ghidra/releases/download/Ghidra_11.1.2_build/ghidra_11.1.2_PUBLIC_20240709.zip"
		zipPath, err := download("ghidra.zip", zipURL)
		if err != nil {
			return err
		}

		if fi, err := os.Stat(zipPath); err == nil && fi.Size() > 0 {
			if err := extractZipStripRoot(zipPath, ghidraDir); err != nil {
				return fmt.Errorf("falha ao extrair Ghidra: %w", err)
			}
		}

		batContent := "@echo off\r\ncall \"%LOCALAPPDATA%\\multitool\\ghidra\\ghidraRun.bat\" %*\r\n"
		_ = os.WriteFile(filepath.Join(binDir, "ghidraRun.bat"), []byte(batContent), 0755)
		_ = os.WriteFile(filepath.Join(binDir, "ghidra.bat"), []byte(batContent), 0755)
		return nil

	case "apktool":
		batURL := "https://raw.githubusercontent.com/iBotPeaches/Apktool/master/scripts/windows/apktool.bat"
		jarURL := "https://github.com/iBotPeaches/Apktool/releases/latest/download/apktool.jar"
		// Cada arquivo possui seu próprio hash; SHA256 sozinho não é ambíguo silenciosamente.
		if target.SHA256 != "" && len(target.ArtifactSHA256) == 0 {
			return fmt.Errorf("apktool requer artifact_sha256 por arquivo (apktool.bat e apktool.jar)")
		}
		batPath, err := download("apktool.bat", batURL)
		if err != nil {
			return err
		}
		jarPath, err := download("apktool.jar", jarURL)
		if err != nil {
			return err
		}
		for name, path := range map[string]string{"apktool.bat": batPath, "apktool.jar": jarPath} {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(binDir, name), data, 0755); err != nil {
				return err
			}
		}
		return nil

	case "orca":
		asset, err := r.directAssetName(toolID)
		if err != nil {
			return err
		}
		setupURL := "https://github.com/stablyai/orca/releases/latest/download/" + asset

		setupPath, err := download(filepath.Base(setupURL), setupURL)
		if err != nil {
			return err
		}
		return r.execWithActivity(actFn, setupPath, "/S")

	case "openmausbot":
		asset, err := r.directAssetName(toolID)
		if err != nil {
			return err
		}
		setupURL := "https://github.com/milind-soni/OpenMausBot/releases/latest/download/" + asset

		setupPath, err := download(filepath.Base(setupURL), setupURL)
		if err != nil {
			return err
		}
		return r.execWithActivity(actFn, setupPath, "/S")

	case "craftlauncher":
		tag := "v0.19.0"
		if resolver, ok := r.executor.(ReleaseTagResolver); ok {
			tag = resolver.ResolveLatestTag("storytold/craft-launcher", tag)
		}
		version := strings.TrimPrefix(tag, "v")
		arch, err := r.directAssetName(toolID)
		if err != nil {
			return err
		}
		setupURL := fmt.Sprintf("https://github.com/storytold/craft-launcher/releases/download/%s/artcraft-launcher-%s-windows-%s.msi", tag, version, arch)

		setupPath, err := download(filepath.Base(setupURL), setupURL)
		if err != nil {
			return err
		}
		if err := r.execWithActivity(actFn, "msiexec.exe", "/i", setupPath, "/qn", "/norestart"); err != nil {
			return err
		}

		batContent := fmt.Sprintf("@echo off\r\nstart \"\" \"%s\" %%*\r\n", filepath.Join(localApp, "Programs", "ArtCraft", "Launcher", "artcraft-launcher.exe"))
		_ = os.WriteFile(filepath.Join(binDir, "artcraft-launcher.bat"), []byte(batContent), 0755)
		_ = os.WriteFile(filepath.Join(binDir, "craft-launcher.bat"), []byte(batContent), 0755)
		_ = os.WriteFile(filepath.Join(binDir, "craftlauncher.bat"), []byte(batContent), 0755)
		return nil

	case "wabt":
		wabtDir := filepath.Join(localApp, "multitool", "wabt")
		if err := os.MkdirAll(wabtDir, 0755); err != nil {
			return err
		}
		asset, err := r.directAssetName(toolID)
		if err != nil {
			return err
		}
		tarURL := "https://github.com/WebAssembly/wabt/releases/download/1.0.42/" + asset

		tarPath, err := download("wabt.tar.gz", tarURL)
		if err != nil {
			return err
		}
		if err := r.execWithActivity(actFn, "tar.exe", "-xzf", tarPath, "-C", wabtDir, "--strip-components=1"); err != nil {
			return fmt.Errorf("falha ao extrair WABT: %w", err)
		}
		entries, _ := os.ReadDir(filepath.Join(wabtDir, "bin"))
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".exe") {
				src := filepath.Join(wabtDir, "bin", e.Name())
				dst := filepath.Join(binDir, e.Name())
				data, err := os.ReadFile(src)
				if err == nil {
					_ = os.WriteFile(dst, data, 0755)
				}
			}
		}
		return nil

	default:
		return fmt.Errorf("ferramenta direta não suportada no Windows: %s", toolID)
	}
}

func (r *Runner) installGitSkillWithActivity(actFn func(string), id, packageID string) error {
	if r.dryRun {
		return nil
	}
	repoURL := fmt.Sprintf("https://github.com/%s.git", packageID)
	dest := r.getSkillDir(id)

	if _, err := os.Stat(dest); err == nil {
		if err := r.execWithActivity(actFn, "git", "-c", "credential.helper=", "-C", dest, "pull", "--ff-only"); err != nil {
			return err
		}
	} else {
		if err := r.execWithActivity(actFn, "git", "-c", "credential.helper=", "clone", "--depth", "1", repoURL, dest); err != nil {
			return err
		}
	}

	targets := r.sysInfo.SkillDirs
	if len(targets) > 0 {
		linked, err := LinkSkillToTargets(dest, targets)
		if err != nil {
			return err
		}
		if linked > 0 {
			fmt.Printf("  ↳ %d skill(s) linkada(s) para agentes locais.\n", linked)
		}
	}
	return nil
}

func (r *Runner) uninstallGitSkill(id string) error {
	dest := r.getSkillDir(id)
	expectedClean := cleanTargetPath(dest)

	for _, target := range r.sysInfo.SkillDirs {
		entries, err := os.ReadDir(target)
		if err != nil {
			// Se o diretório alvo não existir, apenas tenta remover link direto se houver
			link := filepath.Join(target, id)
			_ = os.Remove(link)
			continue
		}

		for _, entry := range entries {
			link := filepath.Join(target, entry.Name())
			resolved, err := os.Readlink(link)
			if err != nil {
				// Se o link tiver o nome exato do pacote e for inválido ou diretório removível
				if entry.Name() == id {
					_ = os.Remove(link)
				}
				continue
			}

			resolvedClean := cleanTargetPath(resolved)
			if !filepath.IsAbs(resolvedClean) {
				resolvedClean = cleanTargetPath(filepath.Join(filepath.Dir(link), resolvedClean))
			}

			sep := string(filepath.Separator)
			matches := resolvedClean == expectedClean || strings.HasPrefix(resolvedClean, expectedClean+sep)
			if runtime.GOOS == "windows" {
				matches = strings.EqualFold(resolvedClean, expectedClean) || strings.HasPrefix(strings.ToLower(resolvedClean), strings.ToLower(expectedClean)+sep)
			}

			if matches || entry.Name() == id {
				_ = os.Remove(link)
			}
		}
	}
	return os.RemoveAll(dest)
}
