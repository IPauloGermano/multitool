package runner

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
	"github.com/IPauloGermano/multitool/internal/system"
)

// -------------------------------------------------------------
// VERIFICAÇÃO DE INTEGRIDADE (SHA-256)
// -------------------------------------------------------------
func VerifyFileSHA256(filePath, expectedSHA string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("falha ao abrir arquivo para verificação de SHA-256: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("falha ao calcular hash SHA-256: %w", err)
	}

	computed := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(computed, expectedSHA) {
		return fmt.Errorf("checksum mismatch: esperado %s, obtido %s", expectedSHA, computed)
	}
	return nil
}

type CommandExecutor interface {
	Execute(name string, args ...string) error
	LookPath(file string) (string, error)
}

type OSExecutor struct{}

func (o *OSExecutor) Execute(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (o *OSExecutor) ExecuteWithActivity(activityFn func(line string), name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin

	if activityFn == nil {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	var errBuf strings.Builder
	var errMu sync.Mutex

	go func() {
		scanner := bufio.NewScanner(pr)
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, 64*1024)
		for scanner.Scan() {
			text := scanner.Text()
			clean := strings.TrimSpace(text)
			if clean != "" {
				errMu.Lock()
				if errBuf.Len() < 8192 {
					errBuf.WriteString(clean + "\n")
				}
				errMu.Unlock()
				activityFn(clean)
			}
		}
	}()

	err := cmd.Run()
	_ = pw.Close()
	_ = pr.Close()

	if err != nil {
		errMu.Lock()
		recentLog := errBuf.String()
		errMu.Unlock()
		if recentLog != "" {
			lines := strings.Split(strings.TrimSpace(recentLog), "\n")
			startIdx := 0
			if len(lines) > 5 {
				startIdx = len(lines) - 5
			}
			for _, l := range lines[startIdx:] {
				fmt.Printf("   %s %s\n", styleError.Render("│"), l)
			}
		}
	}
	return err
}

func (o *OSExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
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
	cacheMu  sync.RWMutex
	cmdCache map[string]bool
}

func New(reg *registry.Registry, dryRun bool) *Runner {
	return &Runner{
		reg:      reg,
		sysInfo:  system.GetSystemInfo(),
		executor: &OSExecutor{},
		dryRun:   dryRun,
		cmdCache: make(map[string]bool),
	}
}

func NewWithExecutor(reg *registry.Registry, executor CommandExecutor, dryRun bool) *Runner {
	return &Runner{
		reg:      reg,
		sysInfo:  system.GetSystemInfo(),
		executor: executor,
		dryRun:   dryRun,
		cmdCache: make(map[string]bool),
	}
}

func (r *Runner) execWithActivity(activityFn func(line string), name string, args ...string) error {
	if ae, ok := r.executor.(interface {
		ExecuteWithActivity(func(string), string, ...string) error
	}); ok {
		return ae.ExecuteWithActivity(activityFn, name, args...)
	}
	return r.executor.Execute(name, args...)
}

func (r *Runner) execWithSudoIfNeeded(cmdName string, args ...string) error {
	return r.execWithSudoAndActivity(nil, cmdName, args...)
}

func (r *Runner) execWithSudoAndActivity(activityFn func(line string), cmdName string, args ...string) error {
	if r.sysInfo.OS == system.OSLinux && r.sysInfo.NeedsSudo {
		allArgs := append([]string{cmdName}, args...)
		return r.execWithActivity(activityFn, "sudo", allArgs...)
	}
	return r.execWithActivity(activityFn, cmdName, args...)
}

func (r *Runner) haveCommand(cmd string) bool {
	r.cacheMu.RLock()
	if r.cmdCache != nil {
		if val, ok := r.cmdCache[cmd]; ok {
			r.cacheMu.RUnlock()
			return val
		}
	}
	r.cacheMu.RUnlock()

	_, err := r.executor.LookPath(cmd)
	found := err == nil

	r.cacheMu.Lock()
	if r.cmdCache == nil {
		r.cmdCache = make(map[string]bool)
	}
	r.cmdCache[cmd] = found
	r.cacheMu.Unlock()

	return found
}

func (r *Runner) targetForTool(tool catalog.Tool) *catalog.Target {
	if r.sysInfo != nil && r.sysInfo.OS != "" {
		return tool.TargetForOS(string(r.sysInfo.OS))
	}
	return tool.TargetForCurrentOS()
}

func (r *Runner) IsDependencySatisfied(depTool *catalog.Tool) bool {
	if depTool == nil {
		return true
	}
	if r.reg.IsInstalled(depTool.ID) {
		return true
	}
	target := r.targetForTool(*depTool)
	if target == nil {
		return true
	}
	if target.Binary != "" && r.haveCommand(target.Binary) {
		return true
	}
	if depTool.ID == "temurin" && r.haveCommand("java") {
		return true
	}
	return false
}

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
		if toolID != "orca" && toolID != "openmausbot" && toolID != "apktool" && toolID != "ghidra" && toolID != "artcraft" && toolID != "wabt" {
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
		_ = r.executor.Execute("flatpak", "remote-add", "--if-not-exists", "flathub", "https://dl.flathub.org/repo/flathub.flatpakrepo")
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

func (r *Runner) Install(tool catalog.Tool) error {
	return r.installInternal(tool, "", true)
}

func (r *Runner) installInternal(tool catalog.Tool, prefix string, showVisual bool) error {
	target := r.targetForTool(tool)
	if target == nil {
		return fmt.Errorf("ferramenta %s (%s) não é compatível com %s", tool.Name, tool.ID, r.sysInfo.OS)
	}

	LogOp("install %s inicio", tool.ID)
	start := time.Now()

	if r.dryRun {
		fmt.Printf("[DRY-RUN] Simulando instalação de %s via %s (%s)\n", tool.Name, target.Type, target.PackageID)
		LogOp("install %s fim rc=0 dry-run", tool.ID)
		return r.reg.MarkInstalled(tool.ID)
	}

	var sp *Spinner
	if showVisual {
		sp = NewSpinner(prefix, fmt.Sprintf("Instalando %s (%s)...", tool.Name, target.Type))
		sp.Start()
	} else if prefix != "" {
		fmt.Printf("➜ %s Instalando %s...\n", prefix, tool.Name)
	} else {
		fmt.Printf("=== Instalando %s (%s) ===\n", tool.Name, tool.ID)
	}

	// Auto-provisionamento de dependências de software
	for _, depID := range tool.Dependencies {
		depTool := catalog.FindTool(depID)
		if depTool != nil && depTool.IsSupportedOnCurrentOS() {
			if !r.IsDependencySatisfied(depTool) {
				if sp != nil {
					sp.Update(fmt.Sprintf("Instalando dependência '%s' para '%s'...", depTool.Name, tool.Name))
				} else {
					fmt.Printf("➜ Instalando dependência obrigatória '%s' para '%s'...\n", depTool.Name, tool.Name)
				}
				if err := r.installInternal(*depTool, prefix, false); err != nil {
					if sp != nil {
						sp.Stop(false, fmt.Sprintf("Falha ao instalar dependência %s: %v", depID, err))
					}
					return fmt.Errorf("falha ao instalar dependência %s: %w", depID, err)
				}
				if sp != nil {
					sp.Update(fmt.Sprintf("Instalando %s (%s)...", tool.Name, target.Type))
				}
			}
		}
	}

	// Auto-provisionamento de runtime
	if sp != nil {
		sp.Update(fmt.Sprintf("Verificando runtime para %s...", tool.Name))
	}
	if err := r.ensureRuntime(tool.ID, target); err != nil {
		if sp != nil {
			sp.Stop(false, fmt.Sprintf("Falha ao provisionar runtime para %s: %v", tool.Name, err))
		} else {
			fmt.Printf("✖ Falha ao provisionar runtime para %s: %v\n", tool.Name, err)
		}
		LogOp("install %s fim rc=1 erro_runtime=%v", tool.ID, err)
		return err
	}

	if sp != nil {
		sp.Update(fmt.Sprintf("Instalando %s via %s (%s)...", tool.Name, target.Type, target.PackageID))
	}

	var actFn func(string)
	if sp != nil {
		actFn = func(line string) {
			sp.UpdateActivity(line)
		}
	}

	var err error
	switch target.Type {
	case catalog.TypeWinget:
		err = r.execWithActivity(actFn, "winget", "install", "--exact", "--id", target.PackageID,
			"--accept-source-agreements", "--accept-package-agreements", "--silent")

	case catalog.TypeNative:
		err = r.installNativeWithActivity(actFn, target.PackageID)

	case catalog.TypeNpm:
		err = r.execWithActivity(actFn, "npm", "install", "-g", target.PackageID)

	case catalog.TypePipx:
		err = r.execWithActivity(actFn, "pipx", "install", target.PackageID)

	case catalog.TypeCargo:
		err = r.execWithActivity(actFn, "cargo", "install", target.PackageID)

	case catalog.TypeFlatpak:
		err = r.execWithActivity(actFn, "flatpak", "install", "-y", "flathub", target.PackageID)

	case catalog.TypeSkill:
		err = r.installGitSkillWithActivity(actFn, tool.ID, target.PackageID)

	case catalog.TypeDirect:
		if tool.ID == "artcraft" || tool.ID == "orca" || tool.ID == "openmausbot" || tool.ID == "apktool" || tool.ID == "ghidra" || tool.ID == "wabt" {
			err = r.installDirectWindowsWithActivity(actFn, tool.ID)
		} else {
			err = r.installGitSkillWithActivity(actFn, tool.ID, target.PackageID)
		}

	case catalog.TypeScript:
		err = r.installScriptWithActivity(actFn, tool.ID, target)

	case catalog.TypeBrew:
		err = r.execWithActivity(actFn, "brew", "install", target.PackageID)

	default:
		err = fmt.Errorf("método de instalação não suportado: %s", target.Type)
	}

	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		if sp != nil {
			sp.Stop(false, fmt.Sprintf("Falha ao instalar %s (%s): %v", tool.Name, elapsed, err))
		} else {
			fmt.Printf("✖ Falha ao instalar %s (%s): %v\n", tool.Name, elapsed, err)
		}
		LogOp("install %s fim rc=1 erro=%v", tool.ID, err)
		return err
	}

	// Validação de integridade SHA-256 opcional pós-instalação
	if target.SHA256 != "" {
		if sp != nil {
			sp.Update(fmt.Sprintf("Validando integridade SHA-256 de %s...", tool.Name))
		}
		binPath := r.findTargetBinaryPath(tool, target)
		if binPath != "" {
			if vErr := VerifyFileSHA256(binPath, target.SHA256); vErr != nil {
				_ = os.Remove(binPath)
				if sp != nil {
					sp.Stop(false, fmt.Sprintf("Falha na integridade SHA-256 de %s: %v", tool.Name, vErr))
				}
				LogOp("install %s fim rc=1 erro_sha256=%v", tool.ID, vErr)
				return fmt.Errorf("falha na integridade SHA-256 de %s: %w", tool.Name, vErr)
			}
			if sp == nil {
				fmt.Printf("✔ Integridade SHA-256 verificada com sucesso para %s.\n", tool.Name)
			}
		}
	}

	if sp != nil {
		sp.Stop(true, fmt.Sprintf("%s instalado com sucesso em %s.", tool.Name, elapsed))
	} else {
		fmt.Printf("✔ %s instalado com sucesso em %s.\n", tool.Name, elapsed)
	}
	LogOp("install %s fim rc=0 duracao=%s", tool.ID, elapsed)
	return r.reg.MarkInstalled(tool.ID)
}

func (r *Runner) findTargetBinaryPath(tool catalog.Tool, target *catalog.Target) string {
	if target == nil {
		return ""
	}
	home, _ := os.UserHomeDir()
	if target.Binary != "" && home != "" {
		p1 := filepath.Join(home, ".local", "bin", target.Binary)
		if _, err := os.Stat(p1); err == nil {
			return p1
		}
		p2 := filepath.Join(home, ".local", "bin", tool.ID)
		if _, err := os.Stat(p2); err == nil {
			return p2
		}
	}
	if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" && target.Binary != "" {
		pWin1 := filepath.Join(localApp, "multitool", "bin", target.Binary)
		if _, err := os.Stat(pWin1); err == nil {
			return pWin1
		}
		pWin2 := filepath.Join(localApp, "multitool", "bin", tool.ID+".bat")
		if _, err := os.Stat(pWin2); err == nil {
			return pWin2
		}
	}
	if target.Binary != "" {
		if p, err := r.executor.LookPath(target.Binary); err == nil {
			return p
		}
	}
	return ""
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
	switch id {
	case "ollama":
		if r.sysInfo.OS == system.OSLinux {
			return r.execWithActivity(actFn, "sh", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
		}
	case "hermes":
		if r.sysInfo.OS == system.OSLinux {
			return r.execWithActivity(actFn, "bash", "-c", "curl -fsSL https://hermes-agent.nousresearch.com/install.sh | bash")
		}
	case "rustup":
		if r.sysInfo.OS == system.OSLinux {
			return r.execWithActivity(actFn, "sh", "-c", "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y")
		}
	case "uv":
		if r.sysInfo.OS == system.OSLinux {
			return r.execWithActivity(actFn, "sh", "-c", "curl -LsSf https://astral.sh/uv/install.sh | sh")
		}
	case "flathub":
		return r.execWithSudoAndActivity(actFn, "flatpak", "remote-add", "--if-not-exists", "flathub", "https://dl.flathub.org/repo/flathub.flatpakrepo")
	case "orca":
		if r.sysInfo.OS == system.OSLinux {
			return r.execWithActivity(actFn, "sh", "-c", "mkdir -p $HOME/.local/bin && curl -fsSL https://github.com/stablyai/orca/releases/latest/download/orca-linux.AppImage -o $HOME/.local/bin/orca && chmod +x $HOME/.local/bin/orca")
		}
	case "openmausbot":
		if r.sysInfo.OS == system.OSLinux {
			return r.execWithActivity(actFn, "sh", "-c", "mkdir -p $HOME/.local/bin && curl -fsSL https://github.com/milind-soni/OpenMausBot/releases/latest/download/OpenMausBot.AppImage -o $HOME/.local/bin/openmausbot && chmod +x $HOME/.local/bin/openmausbot")
		}
	case "gbrain":
		if r.sysInfo.OS == system.OSLinux {
			return r.execWithActivity(actFn, "sh", "-c", "mkdir -p $HOME/.local/bin && curl -fsSL https://github.com/garrytan/gbrain/releases/latest/download/gbrain-linux-x64 -o $HOME/.local/bin/gbrain && chmod +x $HOME/.local/bin/gbrain")
		}
		if r.sysInfo.OS == system.OSDarwin {
			return r.execWithActivity(actFn, "sh", "-c", "mkdir -p $HOME/.local/bin && curl -fsSL https://github.com/garrytan/gbrain/releases/latest/download/gbrain-darwin-arm64 -o $HOME/.local/bin/gbrain && chmod +x $HOME/.local/bin/gbrain")
		}
	case "docker":
		if r.sysInfo.OS == system.OSLinux {
			return r.execWithActivity(actFn, "sh", "-c", "curl -fsSL https://get.docker.com | sh")
		}
	case "opengym", "openstock", "openmaicapp", "artcraft":
		if r.sysInfo.OS == system.OSLinux {
			repo := target.PackageID
			return r.execWithActivity(actFn, "sh", "-c", fmt.Sprintf("mkdir -p $HOME/.local/share/apps && git clone --depth 1 https://github.com/%s $HOME/.local/share/apps/%s", repo, id))
		}
	case "temurin":
		if r.sysInfo.OS == system.OSLinux {
			arch := r.adoptiumArch()
			cmd := fmt.Sprintf("mkdir -p $HOME/.local/share/adoptium && curl -fsSL 'https://api.adoptium.net/v3/binary/latest/21/ga/linux/%s/jdk/hotspot/normal/eclipse' | tar -xz -C $HOME/.local/share/adoptium --strip-components=1 && mkdir -p $HOME/.local/bin && ln -sf $HOME/.local/share/adoptium/bin/java $HOME/.local/bin/java", arch)
			return r.execWithActivity(actFn, "sh", "-c", cmd)
		}
	case "ghidra":
		if r.sysInfo.OS == system.OSLinux {
			cmd := "mkdir -p $HOME/.local/share/ghidra && " +
				"curl -fsSL 'https://github.com/NationalSecurityAgency/ghidra/releases/download/Ghidra_11.1.2_build/ghidra_11.1.2_PUBLIC_20240709.zip' -o /tmp/ghidra.zip && " +
				"if command -v unzip >/dev/null 2>&1; then " +
				"  unzip -q /tmp/ghidra.zip -d /tmp; " +
				"elif command -v python3 >/dev/null 2>&1; then " +
				"  python3 -m zipfile -e /tmp/ghidra.zip /tmp; " +
				"elif [ -x \"$HOME/.local/share/adoptium/bin/jar\" ]; then " +
				"  (cd /tmp && \"$HOME/.local/share/adoptium/bin/jar\" xf /tmp/ghidra.zip); " +
				"elif command -v jar >/dev/null 2>&1; then " +
				"  (cd /tmp && jar xf /tmp/ghidra.zip); " +
				"else " +
				"  unzip -q /tmp/ghidra.zip -d /tmp; " +
				"fi && " +
				"mv /tmp/ghidra_*/* $HOME/.local/share/ghidra && " +
				"rm -rf /tmp/ghidra_* /tmp/ghidra.zip && " +
				"chmod +x $HOME/.local/share/ghidra/ghidraRun && " +
				"mkdir -p $HOME/.local/bin && ln -sf $HOME/.local/share/ghidra/ghidraRun $HOME/.local/bin/ghidra"
			return r.execWithActivity(actFn, "sh", "-c", cmd)
		}
	case "ohmyzsh":
		if r.sysInfo.OS == system.OSLinux {
			return r.execWithActivity(actFn, "sh", "-c", "RUNZSH=no CHSH=no KEEP_ZSHRC=yes sh -c \"$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)\"")
		}
	case "zed":
		if r.sysInfo.OS == system.OSLinux {
			return r.execWithActivity(actFn, "sh", "-c", "curl -f https://zed.dev/install.sh | sh")
		}
	case "tailscale":
		if r.sysInfo.OS == system.OSLinux {
			return r.execWithActivity(actFn, "sh", "-c", "curl -fsSL https://tailscale.com/install.sh | sh")
		}
	case "trivy":
		if r.sysInfo.OS == system.OSLinux {
			return r.execWithActivity(actFn, "sh", "-c", "mkdir -p $HOME/.local/bin && curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b $HOME/.local/bin")
		}
	case "goose":
		if r.sysInfo.OS == system.OSLinux {
			return r.execWithActivity(actFn, "bash", "-c", "curl -fsSL https://github.com/block/goose/releases/download/stable/download_cli.sh | bash")
		}
	case "cherrystudio":
		if r.sysInfo.OS == system.OSLinux {
			return r.execWithActivity(actFn, "sh", "-c", "mkdir -p $HOME/.local/bin && curl -fsSL https://github.com/CherryHQ/cherry-studio/releases/latest/download/Cherry-Studio-2.1.7-x86_64.AppImage -o $HOME/.local/bin/cherrystudio && chmod +x $HOME/.local/bin/cherrystudio")
		}
	}

	return fmt.Errorf("script de instalação direta não implementado para: %s", id)
}

// adoptiumArch mapeia a arquitetura do sistema (r.sysInfo.Arch) para o identificador da API do Adoptium.
// amd64 -> x64, arm64 -> aarch64, arm -> arm, default -> x64
func (r *Runner) adoptiumArch() string {
	switch r.sysInfo.Arch {
	case "arm64":
		return "aarch64"
	case "arm":
		return "arm"
	default:
		return "x64"
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
		if r.haveCommand("rustup") {
			_ = r.executor.Execute("rustup", "self", "uninstall", "-y")
		} else if home != "" {
			cargoRustup := filepath.Join(home, ".cargo", "bin", "rustup")
			if _, err := os.Stat(cargoRustup); err == nil {
				_ = r.executor.Execute(cargoRustup, "self", "uninstall", "-y")
			}
		}
		if home != "" {
			_ = os.RemoveAll(filepath.Join(home, ".rustup"))
			_ = os.RemoveAll(filepath.Join(home, ".cargo"))
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

	case "opengym", "openstock", "openmaicapp", "artcraft":
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
		_ = r.execWithSudoIfNeeded("flatpak", "remote-delete", "--force", "flathub")

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

func (r *Runner) Uninstall(tool catalog.Tool) error {
	return r.uninstallInternal(tool, "", true)
}

func (r *Runner) uninstallInternal(tool catalog.Tool, prefix string, showVisual bool) error {
	target := r.targetForTool(tool)
	if target == nil {
		return fmt.Errorf("ferramenta %s (%s) não é compatível com %s", tool.Name, tool.ID, r.sysInfo.OS)
	}

	LogOp("uninstall %s inicio", tool.ID)
	start := time.Now()

	if r.dryRun {
		fmt.Printf("[DRY-RUN] Simulando desinstalação de %s via %s (%s)\n", tool.Name, target.Type, target.PackageID)
		LogOp("uninstall %s fim rc=0 dry-run", tool.ID)
		return r.reg.MarkUninstalled(tool.ID)
	}

	var sp *Spinner
	if showVisual {
		sp = NewSpinner(prefix, fmt.Sprintf("Desinstalando %s...", tool.Name))
		sp.Start()
	} else if prefix != "" {
		fmt.Printf("➜ %s Desinstalando %s...\n", prefix, tool.Name)
	} else {
		fmt.Printf("=== Desinstalando %s (%s) ===\n", tool.Name, tool.ID)
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
		pkgName := target.PackageID
		if strings.HasPrefix(pkgName, "git+") || strings.HasPrefix(pkgName, "http") {
			pkgName = strings.TrimSuffix(target.Binary, ".exe")
		}
		err = r.executor.Execute("pipx", "uninstall", pkgName)

	case catalog.TypeCargo:
		err = r.executor.Execute("cargo", "uninstall", target.PackageID)

	case catalog.TypeFlatpak:
		err = r.executor.Execute("flatpak", "uninstall", "-y", target.PackageID)

	case catalog.TypeSkill, catalog.TypeDirect:
		if tool.ID == "apktool" {
			localApp := os.Getenv("LOCALAPPDATA")
			if localApp != "" {
				_ = os.Remove(filepath.Join(localApp, "multitool", "bin", "apktool.bat"))
				_ = os.Remove(filepath.Join(localApp, "multitool", "bin", "apktool.jar"))
			}
			err = r.reg.MarkUninstalled(tool.ID)
		} else if tool.ID == "ghidra" {
			localApp := os.Getenv("LOCALAPPDATA")
			if localApp != "" {
				_ = os.RemoveAll(filepath.Join(localApp, "multitool", "ghidra"))
				_ = os.Remove(filepath.Join(localApp, "multitool", "bin", "ghidra.bat"))
				_ = os.Remove(filepath.Join(localApp, "multitool", "bin", "ghidraRun.bat"))
			}
			err = r.reg.MarkUninstalled(tool.ID)
		} else if tool.ID == "artcraft" || tool.ID == "orca" || tool.ID == "openmausbot" {
			err = r.reg.MarkUninstalled(tool.ID)
		} else if tool.ID == "wabt" {
			localApp := os.Getenv("LOCALAPPDATA")
			if localApp != "" {
				_ = os.RemoveAll(filepath.Join(localApp, "multitool", "wabt"))
				entries := []string{"wat2wasm.exe", "wasm2wat.exe", "wasm-interp.exe", "wasm-objdump.exe", "wasm-validate.exe", "wasm-stats.exe", "wasm-strip.exe", "wasm2c.exe", "wast2json.exe", "wat-desugar.exe", "spectest-interp.exe"}
				for _, bin := range entries {
					_ = os.Remove(filepath.Join(localApp, "multitool", "bin", bin))
				}
			}
			err = r.reg.MarkUninstalled(tool.ID)
		} else {
			err = r.uninstallGitSkill(tool.ID)
		}

	case catalog.TypeScript:
		err = r.uninstallScript(tool, target)
		if err == nil {
			err = r.reg.MarkUninstalled(tool.ID)
		}

	case catalog.TypeBrew:
		err = r.executor.Execute("brew", "uninstall", target.PackageID)

	default:
		err = fmt.Errorf("método de desinstalação não suportado: %s", target.Type)
	}

	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		if sp != nil {
			sp.Stop(false, fmt.Sprintf("Falha ao desinstalar %s (%s): %v", tool.Name, elapsed, err))
		} else {
			fmt.Printf("✖ Falha ao desinstalar %s (%s): %v\n", tool.Name, elapsed, err)
		}
		LogOp("uninstall %s fim rc=1 erro=%v", tool.ID, err)
		return err
	}

	if sp != nil {
		sp.Stop(true, fmt.Sprintf("%s removido com sucesso em %s.", tool.Name, elapsed))
	} else {
		fmt.Printf("✔ %s removido com sucesso em %s.\n", tool.Name, elapsed)
	}
	LogOp("uninstall %s fim rc=0 duracao=%s", tool.ID, elapsed)
	return r.reg.MarkUninstalled(tool.ID)
}

func (r *Runner) Update(tool catalog.Tool) error {
	return r.updateInternal(tool, "", true)
}

func (r *Runner) updateInternal(tool catalog.Tool, prefix string, showVisual bool) error {
	target := r.targetForTool(tool)
	if target == nil {
		return fmt.Errorf("ferramenta %s (%s) não é compatível com %s", tool.Name, tool.ID, r.sysInfo.OS)
	}

	LogOp("update %s inicio", tool.ID)
	start := time.Now()

	if r.dryRun {
		fmt.Printf("[DRY-RUN] Simulando atualização de %s via %s (%s)\n", tool.Name, target.Type, target.PackageID)
		LogOp("update %s fim rc=0 dry-run", tool.ID)
		return nil
	}

	var sp *Spinner
	if showVisual {
		sp = NewSpinner(prefix, fmt.Sprintf("Atualizando %s via %s...", tool.Name, target.Type))
		sp.Start()
	} else if prefix != "" {
		fmt.Printf("➜ %s Atualizando %s...\n", prefix, tool.Name)
	} else {
		fmt.Printf("=== Atualizando %s (%s) ===\n", tool.Name, tool.ID)
	}

	// Auto-provisionamento de runtime
	if sp != nil {
		sp.Update(fmt.Sprintf("Verificando runtime para %s...", tool.Name))
	}
	if err := r.ensureRuntime(tool.ID, target); err != nil {
		if sp != nil {
			sp.Stop(false, fmt.Sprintf("Falha ao provisionar runtime para %s: %v", tool.Name, err))
		} else {
			fmt.Printf("✖ Falha ao provisionar runtime para %s: %v\n", tool.Name, err)
		}
		LogOp("update %s fim rc=1 erro_runtime=%v", tool.ID, err)
		return err
	}

	if sp != nil {
		sp.Update(fmt.Sprintf("Atualizando %s via %s (%s)...", tool.Name, target.Type, target.PackageID))
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
		if strings.HasPrefix(target.PackageID, "git+") || strings.HasPrefix(target.PackageID, "http") {
			err = r.executor.Execute("pipx", "install", "--force", target.PackageID)
		} else {
			err = r.executor.Execute("pipx", "upgrade", target.PackageID)
		}

	case catalog.TypeCargo:
		err = r.executor.Execute("cargo", "install", target.PackageID)

	case catalog.TypeFlatpak:
		err = r.executor.Execute("flatpak", "update", "-y", target.PackageID)

	case catalog.TypeSkill:
		err = r.installGitSkill(tool.ID, target.PackageID)

	case catalog.TypeDirect:
		if tool.ID == "artcraft" || tool.ID == "orca" || tool.ID == "openmausbot" || tool.ID == "apktool" || tool.ID == "ghidra" || tool.ID == "wabt" {
			err = r.installDirectWindows(tool.ID)
		} else {
			err = r.installGitSkill(tool.ID, target.PackageID)
		}

	case catalog.TypeScript:
		err = r.installScript(tool.ID, target)

	case catalog.TypeBrew:
		err = r.executor.Execute("brew", "upgrade", target.PackageID)

	default:
		err = fmt.Errorf("método de atualização não suportado: %s", target.Type)
	}

	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		if sp != nil {
			sp.Stop(false, fmt.Sprintf("Falha ao atualizar %s (%s): %v", tool.Name, elapsed, err))
		} else {
			fmt.Printf("✖ Falha ao atualizar %s (%s): %v\n", tool.Name, elapsed, err)
		}
		LogOp("update %s fim rc=1 erro=%v", tool.ID, err)
		return err
	}

	if sp != nil {
		sp.Stop(true, fmt.Sprintf("%s atualizado com sucesso em %s.", tool.Name, elapsed))
	} else {
		fmt.Printf("✔ %s atualizado com sucesso em %s.\n", tool.Name, elapsed)
	}
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
	return r.installGitSkillWithActivity(nil, id, packageID)
}

func extractZipStripRoot(zipPath, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		cleanName := filepath.ToSlash(f.Name)
		parts := strings.Split(cleanName, "/")
		if len(parts) <= 1 {
			continue
		}
		subPath := filepath.Join(parts[1:]...)
		destPath := filepath.Join(destDir, subPath)

		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) && destPath != filepath.Clean(destDir) {
			continue
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(destPath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) installDirectWindows(toolID string) error {
	return r.installDirectWindowsWithActivity(nil, toolID)
}

func (r *Runner) installDirectWindowsWithActivity(actFn func(string), toolID string) error {
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
		zipPath := filepath.Join(os.TempDir(), "ghidra_dl.zip")
		_ = os.Remove(zipPath)
		defer os.Remove(zipPath)

		zipURL := "https://github.com/NationalSecurityAgency/ghidra/releases/download/Ghidra_11.1.2_build/ghidra_11.1.2_PUBLIC_20240709.zip"
		if err := r.execWithActivity(actFn, "curl.exe", "-fsSL", zipURL, "-o", zipPath); err != nil {
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
		batPath := filepath.Join(binDir, "apktool.bat")
		jarPath := filepath.Join(binDir, "apktool.jar")

		if err := r.execWithActivity(actFn, "curl.exe", "-fsSL", batURL, "-o", batPath); err != nil {
			return err
		}
		if err := r.execWithActivity(actFn, "curl.exe", "-fsSL", jarURL, "-o", jarPath); err != nil {
			return err
		}
		return nil

	case "orca":
		setupURL := "https://github.com/stablyai/orca/releases/latest/download/orca-windows-setup.exe"
		setupPath := filepath.Join(os.TempDir(), "orca-setup.exe")
		defer os.Remove(setupPath)

		if err := r.execWithActivity(actFn, "curl.exe", "-fsSL", setupURL, "-o", setupPath); err != nil {
			return err
		}
		return r.execWithActivity(actFn, setupPath, "/S")

	case "openmausbot":
		setupURL := "https://github.com/milind-soni/OpenMausBot/releases/latest/download/OpenMausBot-setup.exe"
		setupPath := filepath.Join(os.TempDir(), "OpenMausBot-setup.exe")
		defer os.Remove(setupPath)

		if err := r.execWithActivity(actFn, "curl.exe", "-fsSL", setupURL, "-o", setupPath); err != nil {
			return err
		}
		return r.execWithActivity(actFn, setupPath, "/S")

	case "artcraft":
		setupURL := "https://github.com/storytold/artcraft/releases/download/artcraft-v0.41.0/ArtCraft_0.41.0_x64-setup.exe"
		setupPath := filepath.Join(os.TempDir(), "ArtCraft-setup.exe")
		defer os.Remove(setupPath)

		if err := r.execWithActivity(actFn, "curl.exe", "-fsSL", setupURL, "-o", setupPath); err != nil {
			return err
		}
		return r.execWithActivity(actFn, setupPath, "/S")

	case "wabt":
		wabtDir := filepath.Join(localApp, "multitool", "wabt")
		if err := os.MkdirAll(wabtDir, 0755); err != nil {
			return err
		}
		tarURL := "https://github.com/WebAssembly/wabt/releases/latest/download/wabt-1.0.42-windows-x64.tar.gz"
		tarPath := filepath.Join(os.TempDir(), "wabt.tar.gz")
		defer os.Remove(tarPath)

		if err := r.execWithActivity(actFn, "curl.exe", "-fsSL", tarURL, "-o", tarPath); err != nil {
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
	repoURL := fmt.Sprintf("https://github.com/%s.git", packageID)
	dest := r.getSkillDir(id)

	if _, err := os.Stat(dest); err == nil {
		if err := r.execWithActivity(actFn, "git", "-C", dest, "pull", "--ff-only"); err != nil {
			return err
		}
	} else {
		if err := r.execWithActivity(actFn, "git", "clone", "--depth", "1", repoURL, dest); err != nil {
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
	total := len(tools)
	if total == 0 {
		return 0, 0
	}

	// Se o lote consistir puramente de ferramentas TypeSkill e total > 1: executa em pool concorrente
	allSkills := true
	for _, t := range tools {
		tgt := r.targetForTool(t)
		if tgt == nil || tgt.Type != catalog.TypeSkill {
			allSkills = false
			break
		}
	}

	if allSkills && total > 1 {
		return r.runBatchInstallSkillsParallel(tools)
	}

	return r.runBatchInstallPartitioned(tools)
}

func (r *Runner) runBatchInstallSkillsParallel(tools []catalog.Tool) (int, int) {
	total := len(tools)
	overallStart := time.Now()

	sp := NewSpinner("", fmt.Sprintf("Clonando concorrentemente %d skills (worker pool de 4)...", total))
	sp.Start()

	type skillResult struct {
		tool catalog.Tool
		err  error
		dur  time.Duration
	}

	results := make(chan skillResult, total)
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup

	for _, t := range tools {
		wg.Add(1)
		go func(tool catalog.Tool) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			err := r.installInternal(tool, "", false)
			results <- skillResult{
				tool: tool,
				err:  err,
				dur:  time.Since(start).Round(time.Millisecond),
			}
		}(t)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	ok, fail := 0, 0
	count := 0
	for res := range results {
		count++
		pct := float64(count) / float64(total)
		bar := RenderProgressBar(14, pct)
		sp.Update(fmt.Sprintf("%s [%d/%d skills] Clonando concorrentemente...", bar, count, total))

		if res.err != nil {
			fail++
			fmt.Printf("  ↳ %s [%d/%d] %s (%s): %v\n",
				styleError.Render("✖"), count, total, res.tool.Name, res.dur, res.err)
		} else {
			ok++
			fmt.Printf("  ↳ %s [%d/%d] %s (%s)\n",
				styleSuccess.Render("✔"), count, total, res.tool.Name, res.dur)
		}
	}

	overallDur := time.Since(overallStart).Round(time.Millisecond)
	sp.Stop(fail == 0, fmt.Sprintf("Resumo das Skills: %d instalada(s), %d falha(s) em %s.", ok, fail, overallDur))
	return ok, fail
}

func (r *Runner) runBatchInstallPartitioned(tools []catalog.Tool) (int, int) {
	ok, fail := 0, 0
	total := len(tools)
	overallStart := time.Now()

	if r.sysInfo.OS == system.OSLinux && r.sysInfo.NeedsSudo && !r.dryRun {
		_ = r.executor.Execute("sudo", "-v")
	}

	i := 0
	for i < total {
		t := tools[i]
		tgt := r.targetForTool(t)

		// Se encontrar um bloco contíguo de skills > 1
		if tgt != nil && tgt.Type == catalog.TypeSkill {
			j := i
			for j < total {
				curTgt := r.targetForTool(tools[j])
				if curTgt == nil || curTgt.Type != catalog.TypeSkill {
					break
				}
				j++
			}

			skillChunk := tools[i:j]
			if len(skillChunk) > 1 {
				chunkOk, chunkFail := r.runBatchInstallSkillsParallel(skillChunk)
				ok += chunkOk
				fail += chunkFail
				i = j
				continue
			}
		}

		pct := float64(i) / float64(total)
		prefix := fmt.Sprintf("[%d/%d] %s", i+1, total, RenderProgressBar(14, pct))
		if err := r.installInternal(t, prefix, true); err != nil {
			fail++
		} else {
			ok++
		}
		i++
	}

	overallDur := time.Since(overallStart).Round(time.Millisecond)
	fmt.Printf("\nResumo do Lote: %s %d instalado(s), %d falha(s) em %s.\n",
		RenderProgressBar(14, 1.0), ok, fail, overallDur)
	return ok, fail
}

func (r *Runner) RunBatchUninstall(tools []catalog.Tool) (int, int) {
	ok, fail := 0, 0
	total := len(tools)
	overallStart := time.Now()

	for i, t := range tools {
		pct := float64(i) / float64(total)
		prefix := fmt.Sprintf("[%d/%d] %s", i+1, total, RenderProgressBar(14, pct))
		if err := r.uninstallInternal(t, prefix, true); err != nil {
			fail++
		} else {
			ok++
		}
	}
	overallDur := time.Since(overallStart).Round(time.Millisecond)
	fmt.Printf("\nResumo da Remoção: %s %d removido(s), %d falha(s) em %s.\n",
		RenderProgressBar(14, 1.0), ok, fail, overallDur)
	return ok, fail
}

func (r *Runner) RunBatchUpdate(tools []catalog.Tool) (int, int) {
	ok, fail := 0, 0
	total := len(tools)
	overallStart := time.Now()

	for i, t := range tools {
		pct := float64(i) / float64(total)
		prefix := fmt.Sprintf("[%d/%d] %s", i+1, total, RenderProgressBar(14, pct))
		if err := r.updateInternal(t, prefix, true); err != nil {
			fail++
		} else {
			ok++
		}
	}
	overallDur := time.Since(overallStart).Round(time.Millisecond)
	fmt.Printf("\nResumo da Atualização: %s %d atualizado(s), %d falha(s) em %s.\n",
		RenderProgressBar(14, 1.0), ok, fail, overallDur)
	return ok, fail
}

// -------------------------------------------------------------
// SINCRONIZAÇÃO E DETECÇÃO DE FERRAMENTAS DO HOST
// -------------------------------------------------------------
type SyncResult struct {
	ToolID   string `json:"id"`
	ToolName string `json:"name"`
	Binary   string `json:"binary,omitempty"`
	Method   string `json:"method"`
}

func (r *Runner) SyncInstalled() ([]SyncResult, int, error) {
	supported := catalog.SupportedCatalog()
	var newlyDetected []SyncResult
	alreadyCount := 0

	for _, tool := range supported {
		if r.reg.IsInstalled(tool.ID) {
			alreadyCount++
			continue
		}

		target := r.targetForTool(tool)
		if target == nil {
			continue
		}

		detected := false
		detectMethod := ""

		if tool.ID == "temurin" && r.haveCommand("java") {
			detected = true
			detectMethod = "PATH (java)"
		} else if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" && target.Binary != "" && (func() bool {
			_, err1 := os.Stat(filepath.Join(localApp, "multitool", "bin", target.Binary))
			_, err2 := os.Stat(filepath.Join(localApp, "multitool", "bin", tool.ID+".bat"))
			return err1 == nil || err2 == nil
		})() {
			detected = true
			detectMethod = "LOCALAPPDATA/multitool/bin"
		} else if target.Binary != "" && r.haveCommand(target.Binary) {
			detected = true
			detectMethod = fmt.Sprintf("PATH (%s)", target.Binary)
		} else if target.Type == catalog.TypeSkill {
			skillDir := r.getSkillDir(tool.ID)
			if fi, err := os.Stat(skillDir); err == nil && fi.IsDir() {
				detected = true
				detectMethod = "diretório local de skills"
			}
		}

		if detected {
			newlyDetected = append(newlyDetected, SyncResult{
				ToolID:   tool.ID,
				ToolName: tool.Name,
				Binary:   target.Binary,
				Method:   detectMethod,
			})

			if !r.dryRun {
				_ = r.reg.MarkInstalled(tool.ID)
			}
		}
	}

	if !r.dryRun {
		r.RelinkAllSkills()
	}

	return newlyDetected, alreadyCount, nil
}

func (r *Runner) RelinkAllSkills() int {
	targets := r.sysInfo.SkillDirs
	if len(targets) == 0 {
		return 0
	}

	relinked := 0
	skillsBase := filepath.Dir(r.getSkillDir("dummy"))
	entries, err := os.ReadDir(skillsBase)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		if entry.IsDir() {
			skillPath := filepath.Join(skillsBase, entry.Name())
			n, _ := LinkSkillToTargets(skillPath, targets)
			relinked += n
		}
	}
	return relinked
}
