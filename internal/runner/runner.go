package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
	"github.com/IPauloGermano/multitool/internal/system"
)

type CommandExecutor interface {
	Execute(name string, args ...string) error
}

type OSExecutor struct{}

func (o *OSExecutor) Execute(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// -------------------------------------------------------------
// LOG DE AUDITORIA COM ROTAÇÃO (512 KB)
// -------------------------------------------------------------
var (
	logMu       sync.Mutex
	customLog   string
	maxLogBytes int64 = 512 * 1024
)

func DefaultLogPath() string {
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, "multitool", "multitool.log")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "multitool", "multitool.log")
}

func SetLogPath(p string) {
	logMu.Lock()
	defer logMu.Unlock()
	customLog = p
}

func getLogPath() string {
	if customLog != "" {
		return customLog
	}
	return DefaultLogPath()
}

func logGira(path string) {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() <= maxLogBytes {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	keep := data[len(data)-int(maxLogBytes):]
	_ = os.WriteFile(path, keep, 0644)
}

func LogOp(format string, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()

	p := getLogPath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	logGira(p)

	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(f, "[%s] %s\n", timestamp, msg)
}

// -------------------------------------------------------------
// AGENT SKILLS LINKING
// -------------------------------------------------------------
func LinkSkillToTargets(skillDir string, targets []string) (int, error) {
	linked := 0
	skillName := filepath.Base(skillDir)

	for _, targetDir := range targets {
		dest := filepath.Join(targetDir, skillName)
		if _, err := os.Lstat(dest); err == nil {
			continue
		}

		if err := os.Symlink(skillDir, dest); err == nil {
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

// -------------------------------------------------------------
// RUNNER CORE
// -------------------------------------------------------------
type Runner struct {
	reg      *registry.Registry
	sysInfo  *system.SystemInfo
	executor CommandExecutor
	dryRun   bool
}

func New(reg *registry.Registry, dryRun bool) *Runner {
	return &Runner{
		reg:      reg,
		sysInfo:  system.GetSystemInfo(),
		executor: &OSExecutor{},
		dryRun:   dryRun,
	}
}

func NewWithExecutor(reg *registry.Registry, executor CommandExecutor, dryRun bool) *Runner {
	return &Runner{
		reg:      reg,
		sysInfo:  system.GetSystemInfo(),
		executor: executor,
		dryRun:   dryRun,
	}
}

func (r *Runner) execWithSudoIfNeeded(cmdName string, args ...string) error {
	if r.sysInfo.OS == system.OSLinux && r.sysInfo.NeedsSudo {
		allArgs := append([]string{cmdName}, args...)
		return r.executor.Execute("sudo", allArgs...)
	}
	return r.executor.Execute(cmdName, args...)
}

func (r *Runner) Install(tool catalog.Tool) error {
	target := tool.TargetForCurrentOS()
	if target == nil {
		return fmt.Errorf("ferramenta %s (%s) não é compatível com %s", tool.Name, tool.ID, r.sysInfo.OS)
	}

	LogOp("install %s inicio", tool.ID)
	start := time.Now()
	fmt.Printf("=== Instalando %s (%s) ===\n", tool.Name, tool.ID)

	if r.dryRun {
		fmt.Printf("[DRY-RUN] Simulando instalação de %s via %s (%s)\n", tool.Name, target.Type, target.PackageID)
		LogOp("install %s fim rc=0 dry-run", tool.ID)
		return r.reg.MarkInstalled(tool.ID)
	}

	var err error
	switch target.Type {
	case catalog.TypeWinget:
		err = r.executor.Execute("winget", "install", "--exact", "--id", target.PackageID,
			"--accept-source-agreements", "--accept-package-agreements", "--silent")

	case catalog.TypeNative:
		err = r.installNative(target.PackageID)

	case catalog.TypeNpm:
		err = r.executor.Execute("npm", "install", "-g", target.PackageID)

	case catalog.TypePipx:
		err = r.executor.Execute("pipx", "install", target.PackageID)

	case catalog.TypeCargo:
		err = r.executor.Execute("cargo", "install", target.PackageID)

	case catalog.TypeFlatpak:
		err = r.executor.Execute("flatpak", "install", "-y", "flathub", target.PackageID)

	case catalog.TypeSkill:
		err = r.installGitSkill(tool.ID, target.PackageID)

	case catalog.TypeDirect:
		if tool.ID == "orca" {
			err = r.executor.Execute("powershell", "-NoProfile", "-Command", "Invoke-WebRequest -Uri https://github.com/stablyai/orca/releases/latest/download/orca-windows-setup.exe -OutFile $env:TEMP\\orca-setup.exe; Start-Process -FilePath $env:TEMP\\orca-setup.exe -ArgumentList '/S' -Wait")
		} else {
			err = r.installGitSkill(tool.ID, target.PackageID)
		}

	case catalog.TypeScript:
		err = r.installScript(tool.ID, target)

	default:
		err = fmt.Errorf("método de instalação não suportado: %s", target.Type)
	}

	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		fmt.Printf("✖ Falha ao instalar %s (%s): %v\n", tool.Name, elapsed, err)
		LogOp("install %s fim rc=1 erro=%v", tool.ID, err)
		return err
	}

	fmt.Printf("✔ %s instalado com sucesso em %s.\n", tool.Name, elapsed)
	LogOp("install %s fim rc=0 duracao=%s", tool.ID, elapsed)
	return r.reg.MarkInstalled(tool.ID)
}

func (r *Runner) installNative(packageID string) error {
	switch r.sysInfo.PrimaryManager {
	case system.MgrApt:
		return r.execWithSudoIfNeeded("apt-get", "install", "-y", packageID)
	case system.MgrDnf:
		return r.execWithSudoIfNeeded("dnf", "install", "-y", packageID)
	case system.MgrPacman:
		return r.execWithSudoIfNeeded("pacman", "-S", "--noconfirm", "--needed", packageID)
	case system.MgrZypper:
		return r.execWithSudoIfNeeded("zypper", "install", "-y", packageID)
	case system.MgrEopkg:
		return r.execWithSudoIfNeeded("eopkg", "install", "-y", packageID)
	case system.MgrBrew:
		return r.executor.Execute("brew", "install", packageID)
	default:
		return fmt.Errorf("nenhum gerenciador nativo suportado encontrado para instalar %s", packageID)
	}
}

func (r *Runner) uninstallNative(packageID string) error {
	switch r.sysInfo.PrimaryManager {
	case system.MgrApt:
		return r.execWithSudoIfNeeded("apt-get", "remove", "-y", packageID)
	case system.MgrDnf:
		return r.execWithSudoIfNeeded("dnf", "remove", "-y", packageID)
	case system.MgrPacman:
		return r.execWithSudoIfNeeded("pacman", "-R", "--noconfirm", packageID)
	case system.MgrZypper:
		return r.execWithSudoIfNeeded("zypper", "remove", "-y", packageID)
	case system.MgrEopkg:
		return r.execWithSudoIfNeeded("eopkg", "remove", "-y", packageID)
	case system.MgrBrew:
		return r.executor.Execute("brew", "uninstall", packageID)
	default:
		return fmt.Errorf("nenhum gerenciador nativo suportado encontrado para remover %s", packageID)
	}
}

func (r *Runner) updateNative(packageID string) error {
	switch r.sysInfo.PrimaryManager {
	case system.MgrApt:
		return r.execWithSudoIfNeeded("apt-get", "install", "--only-upgrade", "-y", packageID)
	case system.MgrDnf:
		return r.execWithSudoIfNeeded("dnf", "upgrade", "-y", packageID)
	case system.MgrPacman:
		return r.execWithSudoIfNeeded("pacman", "-S", "--noconfirm", packageID)
	case system.MgrZypper:
		return r.execWithSudoIfNeeded("zypper", "update", "-y", packageID)
	case system.MgrEopkg:
		return r.execWithSudoIfNeeded("eopkg", "upgrade", "-y", packageID)
	case system.MgrBrew:
		return r.executor.Execute("brew", "upgrade", packageID)
	default:
		return fmt.Errorf("nenhum gerenciador nativo suportado encontrado para atualizar %s", packageID)
	}
}

func (r *Runner) installScript(id string, target *catalog.Target) error {
	switch id {
	case "ollama":
		if r.sysInfo.OS == system.OSLinux {
			return r.executor.Execute("sh", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
		}
	case "hermes":
		if r.sysInfo.OS == system.OSLinux {
			return r.executor.Execute("bash", "-c", "curl -fsSL https://hermes-agent.nousresearch.com/install.sh | bash")
		}
	case "rustup":
		if r.sysInfo.OS == system.OSLinux {
			return r.executor.Execute("sh", "-c", "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y")
		}
	case "uv":
		if r.sysInfo.OS == system.OSLinux {
			return r.executor.Execute("sh", "-c", "curl -LsSf https://astral.sh/uv/install.sh | sh")
		}
	case "flathub":
		return r.execWithSudoIfNeeded("flatpak", "remote-add", "--if-not-exists", "flathub", "https://dl.flathub.org/repo/flathub.flatpakrepo")
	case "orca":
		if r.sysInfo.OS == system.OSLinux {
			return r.executor.Execute("sh", "-c", "mkdir -p $HOME/.local/bin && curl -fsSL https://github.com/stablyai/orca/releases/latest/download/orca-linux.AppImage -o $HOME/.local/bin/orca && chmod +x $HOME/.local/bin/orca")
		}
	case "docker":
		if r.sysInfo.OS == system.OSLinux {
			return r.executor.Execute("sh", "-c", "curl -fsSL https://get.docker.com | sh")
		}
	case "opengym", "openstock", "openmaicapp":
		if r.sysInfo.OS == system.OSLinux {
			repo := target.PackageID
			return r.executor.Execute("sh", "-c", fmt.Sprintf("mkdir -p $HOME/.local/share/apps && git clone --depth 1 https://github.com/%s $HOME/.local/share/apps/%s", repo, id))
		}
	}

	return fmt.Errorf("script de instalação direta não implementado para: %s", id)
}

func (r *Runner) Uninstall(tool catalog.Tool) error {
	target := tool.TargetForCurrentOS()
	if target == nil {
		return fmt.Errorf("ferramenta %s (%s) não é compatível com %s", tool.Name, tool.ID, r.sysInfo.OS)
	}

	LogOp("uninstall %s inicio", tool.ID)
	start := time.Now()
	fmt.Printf("=== Desinstalando %s (%s) ===\n", tool.Name, tool.ID)

	if r.dryRun {
		fmt.Printf("[DRY-RUN] Simulando desinstalação de %s via %s (%s)\n", tool.Name, target.Type, target.PackageID)
		LogOp("uninstall %s fim rc=0 dry-run", tool.ID)
		return r.reg.MarkUninstalled(tool.ID)
	}

	var err error
	switch target.Type {
	case catalog.TypeWinget:
		err = r.executor.Execute("winget", "uninstall", "--exact", "--id", target.PackageID, "--silent")

	case catalog.TypeNative:
		err = r.uninstallNative(target.PackageID)

	case catalog.TypeNpm:
		err = r.executor.Execute("npm", "uninstall", "-g", target.PackageID)

	case catalog.TypePipx:
		err = r.executor.Execute("pipx", "uninstall", target.PackageID)

	case catalog.TypeCargo:
		err = r.executor.Execute("cargo", "uninstall", target.PackageID)

	case catalog.TypeFlatpak:
		err = r.executor.Execute("flatpak", "uninstall", "-y", target.PackageID)

	case catalog.TypeSkill, catalog.TypeDirect:
		err = r.uninstallGitSkill(tool.ID)

	case catalog.TypeScript:
		if tool.ID == "orca" {
			_ = os.Remove(filepath.Join(os.Getenv("HOME"), ".local", "bin", "orca"))
		} else if tool.ID == "opengym" || tool.ID == "openstock" || tool.ID == "openmaicapp" {
			_ = os.RemoveAll(filepath.Join(os.Getenv("HOME"), ".local", "share", "apps", tool.ID))
		}
		err = r.reg.MarkUninstalled(tool.ID)

	default:
		err = fmt.Errorf("método de desinstalação não suportado: %s", target.Type)
	}

	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		fmt.Printf("✖ Falha ao desinstalar %s (%s): %v\n", tool.Name, elapsed, err)
		LogOp("uninstall %s fim rc=1 erro=%v", tool.ID, err)
		return err
	}

	fmt.Printf("✔ %s removido com sucesso em %s.\n", tool.Name, elapsed)
	LogOp("uninstall %s fim rc=0 duracao=%s", tool.ID, elapsed)
	return r.reg.MarkUninstalled(tool.ID)
}

func (r *Runner) Update(tool catalog.Tool) error {
	target := tool.TargetForCurrentOS()
	if target == nil {
		return fmt.Errorf("ferramenta %s (%s) não é compatível com %s", tool.Name, tool.ID, r.sysInfo.OS)
	}

	LogOp("update %s inicio", tool.ID)
	start := time.Now()
	fmt.Printf("=== Atualizando %s (%s) ===\n", tool.Name, tool.ID)

	if r.dryRun {
		fmt.Printf("[DRY-RUN] Simulando atualização de %s via %s (%s)\n", tool.Name, target.Type, target.PackageID)
		LogOp("update %s fim rc=0 dry-run", tool.ID)
		return nil
	}

	var err error
	switch target.Type {
	case catalog.TypeWinget:
		err = r.executor.Execute("winget", "upgrade", "--exact", "--id", target.PackageID, "--silent")

	case catalog.TypeNative:
		err = r.updateNative(target.PackageID)

	case catalog.TypeNpm:
		err = r.executor.Execute("npm", "update", "-g", target.PackageID)

	case catalog.TypePipx:
		err = r.executor.Execute("pipx", "upgrade", target.PackageID)

	case catalog.TypeCargo:
		err = r.executor.Execute("cargo", "install", target.PackageID)

	case catalog.TypeFlatpak:
		err = r.executor.Execute("flatpak", "update", "-y", target.PackageID)

	case catalog.TypeSkill:
		err = r.installGitSkill(tool.ID, target.PackageID)

	case catalog.TypeDirect:
		if tool.ID == "orca" {
			err = r.executor.Execute("powershell", "-NoProfile", "-Command", "Invoke-WebRequest -Uri https://github.com/stablyai/orca/releases/latest/download/orca-windows-setup.exe -OutFile $env:TEMP\\orca-setup.exe; Start-Process -FilePath $env:TEMP\\orca-setup.exe -ArgumentList '/S' -Wait")
		} else {
			err = r.installGitSkill(tool.ID, target.PackageID)
		}

	case catalog.TypeScript:
		err = r.installScript(tool.ID, target)

	default:
		err = fmt.Errorf("método de atualização não suportado: %s", target.Type)
	}

	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		fmt.Printf("✖ Falha ao atualizar %s (%s): %v\n", tool.Name, elapsed, err)
		LogOp("update %s fim rc=1 erro=%v", tool.ID, err)
		return err
	}

	fmt.Printf("✔ %s atualizado com sucesso em %s.\n", tool.Name, elapsed)
	LogOp("update %s fim rc=0 duracao=%s", tool.ID, elapsed)
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
	repoURL := fmt.Sprintf("https://github.com/%s.git", packageID)
	dest := r.getSkillDir(id)

	if _, err := os.Stat(dest); err == nil {
		if err := r.executor.Execute("git", "-C", dest, "pull", "--ff-only"); err != nil {
			return err
		}
	} else {
		if err := r.executor.Execute("git", "clone", "--depth", "1", repoURL, dest); err != nil {
			return err
		}
	}

	targets := r.sysInfo.SkillDirs
	if len(targets) > 0 {
		linked, _ := LinkSkillToTargets(dest, targets)
		if linked > 0 {
			fmt.Printf("  ↳ %d skill(s) linkada(s) para agentes locais.\n", linked)
		}
	}
	return nil
}

func (r *Runner) uninstallGitSkill(id string) error {
	dest := r.getSkillDir(id)
	targets := r.sysInfo.SkillDirs
	UnlinkSkillFromTargets(id, targets)
	return os.RemoveAll(dest)
}

func (r *Runner) RunBatchInstall(tools []catalog.Tool) (int, int) {
	ok, fail := 0, 0
	total := len(tools)
	overallStart := time.Now()

	for i, t := range tools {
		fmt.Printf("\n--- [%d/%d] ---\n", i+1, total)
		if err := r.Install(t); err != nil {
			fail++
		} else {
			ok++
		}
	}
	fmt.Printf("\nResumo: %d instalado(s), %d falha(s) em %s.\n", ok, fail, time.Since(overallStart).Round(time.Millisecond))
	return ok, fail
}

func (r *Runner) RunBatchUninstall(tools []catalog.Tool) (int, int) {
	ok, fail := 0, 0
	total := len(tools)
	overallStart := time.Now()

	for i, t := range tools {
		fmt.Printf("\n--- [%d/%d] ---\n", i+1, total)
		if err := r.Uninstall(t); err != nil {
			fail++
		} else {
			ok++
		}
	}
	fmt.Printf("\nResumo: %d removido(s), %d falha(s) em %s.\n", ok, fail, time.Since(overallStart).Round(time.Millisecond))
	return ok, fail
}

func (r *Runner) RunBatchUpdate(tools []catalog.Tool) (int, int) {
	ok, fail := 0, 0
	total := len(tools)
	overallStart := time.Now()

	for i, t := range tools {
		fmt.Printf("\n--- [%d/%d] ---\n", i+1, total)
		if err := r.Update(t); err != nil {
			fail++
		} else {
			ok++
		}
	}
	fmt.Printf("\nResumo: %d atualizado(s), %d falha(s) em %s.\n", ok, fail, time.Since(overallStart).Round(time.Millisecond))
	return ok, fail
}
