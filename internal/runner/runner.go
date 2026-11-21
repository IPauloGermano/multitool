package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
	"github.com/IPauloGermano/multitool/internal/system"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type CommandExecutor interface {
	Execute(name string, args ...string) error
	LookPath(file string) (string, error)
}

type TimeoutExecutor interface {
	ExecuteWithTimeout(timeout time.Duration, name string, args ...string) error
}

// SilentExecutor executa comandos descartando stdout/stderr (ex: sondas de status).
type SilentExecutor interface {
	ExecuteSilent(name string, args ...string) error
}

// ReleaseTagResolver mantém consultas HTTP substituíveis junto com a execução.
type ReleaseTagResolver interface {
	ResolveLatestTag(repo, fallback string) string
}

// CommandError encapsula o erro de um comando externo junto às últimas linhas de saída
// capturadas, permitindo que o Runner decida se e quando exibir o diagnóstico.
type CommandError struct {
	Command     string
	Err         error
	RecentLines []string
}

func (e *CommandError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *CommandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type OSExecutor struct{}

func (o *OSExecutor) ResolveLatestTag(repo, fallback string) string {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       4 * time.Second,
	}
	return resolveGitHubLatestTag(client, repo, fallback)
}

func (o *OSExecutor) Execute(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (o *OSExecutor) ExecuteSilent(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func (o *OSExecutor) ExecuteWithTimeout(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (o *OSExecutor) ExecuteWithActivity(activityFn func(line string), name string, args ...string) error {
	return o.ExecuteWithActivityTimeout(0, activityFn, name, args...)
}

const maxRecentOutputLines = 10

func (o *OSExecutor) ExecuteWithActivityTimeout(timeout time.Duration, activityFn func(line string), name string, args ...string) error {
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin

	if activityFn == nil {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	var recentLines []string
	var bufMu sync.Mutex
	done := make(chan struct{})

	go func() {
		defer close(done)
		scanner := bufio.NewScanner(pr)
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, 64*1024)
		for scanner.Scan() {
			clean := extractOutputLine(scanner.Text())
			if clean != "" {
				bufMu.Lock()
				if len(recentLines) >= maxRecentOutputLines {
					copy(recentLines, recentLines[1:])
					recentLines[len(recentLines)-1] = clean
				} else {
					recentLines = append(recentLines, clean)
				}
				bufMu.Unlock()
				activityFn(clean)
			}
		}
	}()

	err := cmd.Run()
	_ = pw.Close()
	<-done
	_ = pr.Close()

	if err != nil {
		bufMu.Lock()
		lines := append([]string(nil), recentLines...)
		bufMu.Unlock()
		if len(lines) > 0 {
			return &CommandError{
				Command:     name,
				Err:         err,
				RecentLines: lines,
			}
		}
	}
	return err
}

func (o *OSExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// -------------------------------------------------------------
// RUNNER CORE
// -------------------------------------------------------------
type Runner struct {
	reg      *registry.Registry
	sysInfo  *system.SystemInfo
	executor CommandExecutor
	dryRun   bool
	force    bool
	verbose  bool
	cacheMu  sync.RWMutex
	cmdCache map[string]bool
}

func (r *Runner) SetForce(force bool) {
	r.force = force
}

func (r *Runner) Force() bool {
	return r.force
}

func (r *Runner) SetVerbose(verbose bool) {
	r.verbose = verbose
}

func (r *Runner) Verbose() bool {
	return r.verbose
}

func New(reg *registry.Registry, dryRun bool) *Runner {
	info := *system.GetSystemInfo()
	if reg != nil {
		reg.SetDryRun(dryRun)
	}
	return &Runner{
		reg:      reg,
		sysInfo:  &info,
		executor: &OSExecutor{},
		dryRun:   dryRun,
		cmdCache: make(map[string]bool),
	}
}

func NewWithExecutor(reg *registry.Registry, executor CommandExecutor, dryRun bool) *Runner {
	info := *system.GetSystemInfo()
	if reg != nil {
		reg.SetDryRun(dryRun)
	}
	return &Runner{
		reg:      reg,
		sysInfo:  &info,
		executor: executor,
		dryRun:   dryRun,
		cmdCache: make(map[string]bool),
	}
}

func (r *Runner) execWithActivity(activityFn func(line string), name string, args ...string) error {
	if r.verbose {
		activityFn = nil
	}
	if ae, ok := r.executor.(interface {
		ExecuteWithActivity(func(string), string, ...string) error
	}); ok {
		return ae.ExecuteWithActivity(activityFn, name, args...)
	}
	return r.executor.Execute(name, args...)
}

func (r *Runner) execSilent(name string, args ...string) error {
	if se, ok := r.executor.(SilentExecutor); ok {
		return se.ExecuteSilent(name, args...)
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

func (r *Runner) execWithTimeout(timeout time.Duration, name string, args ...string) error {
	if te, ok := r.executor.(TimeoutExecutor); ok {
		return te.ExecuteWithTimeout(timeout, name, args...)
	}
	return r.executor.Execute(name, args...)
}

func (r *Runner) execWithActivityAndTimeout(timeout time.Duration, activityFn func(line string), name string, args ...string) error {
	if r.verbose {
		activityFn = nil
	}
	if ae, ok := r.executor.(interface {
		ExecuteWithActivityTimeout(time.Duration, func(string), string, ...string) error
	}); ok {
		return ae.ExecuteWithActivityTimeout(timeout, activityFn, name, args...)
	}
	return r.execWithActivity(activityFn, name, args...)
}

func (r *Runner) startSudoKeepAlive() func() {
	if r.sysInfo == nil || r.sysInfo.OS != system.OSLinux || !r.sysInfo.NeedsSudo || r.dryRun {
		return func() {}
	}

	_ = r.executor.Execute("sudo", "-v")

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(45 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = r.executor.Execute("sudo", "-n", "-v")
			}
		}
	}()

	return func() {
		close(done)
	}
}

func (r *Runner) toolNeedsSudo(tool catalog.Tool, isInstallOrUpdate bool) bool {
	if r.sysInfo == nil || r.sysInfo.OS != system.OSLinux || !r.sysInfo.NeedsSudo || r.dryRun {
		return false
	}
	target := r.targetForTool(tool)
	if target == nil {
		return false
	}
	if target.Type == catalog.TypeNative && r.sysInfo.PrimaryManager != system.MgrBrew {
		return true
	}
	if target.Type == catalog.TypeScript {
		switch tool.ID {
		case "docker", "tailscale", "antigravity", "dockerdesktop", "warp", "waydroid":
			return isInstallOrUpdate
		case "ollama":
			if isInstallOrUpdate {
				return true
			}
			for _, p := range []string{"/etc/systemd/system/ollama.service", "/usr/local/bin/ollama", "/usr/bin/ollama", "/usr/share/ollama"} {
				if _, err := os.Stat(p); err == nil {
					return true
				}
			}
			return false
		}
	}
	return false
}

func (r *Runner) batchNeedsSudo(tools []catalog.Tool, isInstallOrUpdate bool) bool {
	for _, t := range tools {
		if r.toolNeedsSudo(t, isInstallOrUpdate) {
			return true
		}
	}
	return false
}

// isDirectWindowsBinary identifica ferramentas TypeDirect que possuem receita
// própria de download/instalação de binário ou fonte no Windows (em contraste com skills Git).
func isDirectWindowsBinary(toolID string) bool {
	switch toolID {
	case "craftlauncher", "orca", "openmausbot", "apktool", "ghidra", "wabt",
		"goose", "ohmypi", "atkinson", "sourcesans3", "sourceserif4":
		return true
	default:
		return false
	}
}

// isDirectPortableDownload identifica ferramentas TypeDirect que apenas baixam e
// extraem arquivos portáteis em diretórios isolados do usuário (sem trava global de MSI/NSIS),
// sendo seguras para instalação concorrente em worker pool.
func isDirectPortableDownload(toolID string) bool {
	switch toolID {
	case "apktool", "ghidra", "wabt", "goose", "ohmypi", "atkinson", "sourcesans3", "sourceserif4":
		return true
	default:
		return false
	}
}

// isGitSkillTarget indica se o alvo representa uma skill clonada via Git
// (TypeSkill no Linux/macOS ou TypeDirect não-binário no Windows).
func isGitSkillTarget(toolID string, target *catalog.Target) bool {
	if target == nil {
		return false
	}
	if target.Type == catalog.TypeSkill {
		return true
	}
	return target.Type == catalog.TypeDirect && !isDirectWindowsBinary(toolID)
}

func (r *Runner) allDependenciesSatisfied(tool catalog.Tool) bool {
	for _, depID := range tool.Dependencies {
		depTool := catalog.FindTool(depID)
		if depTool != nil && depTool.IsSupportedOnCurrentOS() && !r.IsDependencySatisfied(depTool) {
			return false
		}
	}
	return true
}

func (r *Runner) isParallelDirectTarget(tool catalog.Tool, target *catalog.Target) bool {
	return target != nil && target.Type == catalog.TypeDirect && isDirectPortableDownload(tool.ID) && r.allDependenciesSatisfied(tool)
}

func (r *Runner) isToolPresentOnHost(tool catalog.Tool, target *catalog.Target) bool {
	if target == nil {
		return false
	}
	if isGitSkillTarget(tool.ID, target) {
		fi, err := os.Stat(r.getSkillDir(tool.ID))
		return err == nil && fi.IsDir()
	}
	switch target.Type {
	case catalog.TypeFlatpak:
		if !r.haveCommand("flatpak") {
			return false
		}
		return r.execSilent("flatpak", "info", target.PackageID) == nil
	default:
		if tool.ID == "temurin" && r.haveCommand("java") {
			return true
		}
		return r.findTargetBinaryPath(tool, target) != ""
	}
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

func diagnosticLines(err error) []string {
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) || len(cmdErr.RecentLines) == 0 {
		return nil
	}
	lines := cmdErr.RecentLines
	if len(lines) > 5 {
		lines = lines[len(lines)-5:]
	}
	return lines
}

func printCommandDiagnostics(err error) {
	for _, l := range diagnosticLines(err) {
		fmt.Printf("   %s %s\n", styleError.Render("│"), l)
	}
}

func printCommandDiagnosticsToSpinner(sp *Spinner, err error) {
	for _, l := range diagnosticLines(err) {
		line := fmt.Sprintf("   %s %s", styleError.Render("│"), l)
		if sp != nil {
			sp.PrintLine(line)
		} else {
			fmt.Println(line)
		}
	}
}

func (r *Runner) Install(tool catalog.Tool) error {
	return r.installInternalWithQuiet(tool, "", true, false)
}

func (r *Runner) installInternal(tool catalog.Tool, prefix string, showVisual bool) error {
	return r.installInternalWithQuiet(tool, prefix, showVisual, false)
}

func (r *Runner) installInternalWithQuiet(tool catalog.Tool, prefix string, showVisual bool, quiet bool) error {
	target := r.targetForTool(tool)
	if target == nil {
		return fmt.Errorf("ferramenta %s (%s) não é compatível com %s", tool.Name, tool.ID, r.sysInfo.OS)
	}

	LogOp("install %s inicio", tool.ID)
	start := time.Now()

	if r.dryRun {
		if !quiet {
			fmt.Printf("[DRY-RUN] Simulando instalação de %s via %s (%s)\n", tool.Name, target.Type, target.PackageID)
		}
		LogOp("install %s fim rc=0 dry-run", tool.ID)
		return r.reg.MarkInstalled(tool.ID)
	}

	// Fast-path: se a ferramenta já está presente no sistema host e --force não foi passado,
	// sincroniza o registro imediatamente sem acionar o gerenciador externo.
	if !r.force && r.isToolPresentOnHost(tool, target) {
		if isGitSkillTarget(tool.ID, target) && len(r.sysInfo.SkillDirs) > 0 {
			_, _ = LinkSkillToTargets(r.getSkillDir(tool.ID), r.sysInfo.SkillDirs)
		}
		elapsed := time.Since(start).Round(time.Millisecond)
		if !quiet {
			msg := fmt.Sprintf("%s já presente no sistema (%s) — use --force para reinstalar.", tool.Name, elapsed)
			if prefix != "" {
				msg = prefix + " " + msg
			}
			if showVisual {
				fmt.Printf("%s %s\n", styleSuccess.Render("✔"), msg)
			} else {
				fmt.Printf("✔ %s\n", msg)
			}
		}
		LogOp("install %s fim rc=0 duracao=%s fast_path=true", tool.ID, elapsed)
		return r.reg.MarkInstalled(tool.ID)
	}

	var sp *Spinner
	if showVisual && !r.verbose && !quiet {
		sp = NewSpinner(prefix, fmt.Sprintf("Instalando %s (%s)...", tool.Name, target.Type))
		sp.Start()
	} else if !quiet {
		if prefix != "" {
			fmt.Printf("➜ %s Instalando %s via %s (%s)...\n", prefix, tool.Name, target.Type, target.PackageID)
		} else {
			fmt.Printf("=== Instalando %s (%s) ===\n", tool.Name, tool.ID)
		}
	}

	// Auto-provisionamento de dependências de software
	for _, depID := range tool.Dependencies {
		depTool := catalog.FindTool(depID)
		if depTool != nil && depTool.IsSupportedOnCurrentOS() {
			if !r.IsDependencySatisfied(depTool) {
				if sp != nil {
					sp.Update(fmt.Sprintf("Instalando dependência '%s' para '%s'...", depTool.Name, tool.Name))
				} else if !quiet {
					fmt.Printf("➜ Instalando dependência obrigatória '%s' para '%s'...\n", depTool.Name, tool.Name)
				}
				if err := r.installInternalWithQuiet(*depTool, prefix, false, quiet); err != nil {
					if sp != nil {
						sp.Stop(false, fmt.Sprintf("Falha ao instalar dependência %s: %v", depID, err))
					}
					if !quiet {
						printCommandDiagnostics(err)
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
		} else if !quiet {
			fmt.Printf("✖ Falha ao provisionar runtime para %s: %v\n", tool.Name, err)
		}
		if !quiet {
			printCommandDiagnostics(err)
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
	} else if quiet {
		// No worker pool silencioso, captura saída em buffer sem poluir stdout
		actFn = func(string) {}
	}

	var err error
	var alreadyInstalled bool
	var rebootRequired bool
	switch target.Type {
	case catalog.TypeWinget:
		args := []string{"install", "--exact", "--id", target.PackageID,
			"--accept-source-agreements", "--accept-package-agreements", "--silent"}
		if r.force {
			args = append(args, "--force")
		}
		err = r.execWithActivity(actFn, "winget", args...)
		if isWingetAlreadyInstalled(err) {
			err = nil
			alreadyInstalled = true
		} else if isWingetRebootRequired(err) {
			err = nil
			rebootRequired = true
		}

	case catalog.TypeNative:
		err = r.installNativeWithActivity(actFn, target.PackageID)

	case catalog.TypeNpm:
		err = r.execWithActivity(actFn, "npm", "install", "-g", target.PackageID)

	case catalog.TypePipx:
		args := []string{"install"}
		if r.force {
			args = append(args, "--force")
		}
		args = append(args, target.PackageID)
		err = r.execWithActivity(actFn, "pipx", args...)

	case catalog.TypeCargo:
		args := []string{"install"}
		if r.force {
			args = append(args, "--force")
		}
		args = append(args, target.PackageID)
		err = r.execWithActivity(actFn, "cargo", args...)

	case catalog.TypeFlatpak:
		err = r.execWithActivity(actFn, "flatpak", "--noninteractive", "install", "-y", "flathub", target.PackageID)

	case catalog.TypeSkill:
		err = r.installGitSkillWithActivity(actFn, tool.ID, target.PackageID)

	case catalog.TypeDirect:
		if isDirectWindowsBinary(tool.ID) {
			err = r.installDirectWindowsTargetWithActivity(actFn, tool.ID, target)
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
		} else if !quiet {
			fmt.Printf("✖ Falha ao instalar %s (%s): %v\n", tool.Name, elapsed, err)
		}
		if !quiet {
			printCommandDiagnostics(err)
		}
		LogOp("install %s fim rc=1 erro=%v", tool.ID, err)
		return err
	}

	if sp != nil {
		if alreadyInstalled {
			sp.Stop(true, fmt.Sprintf("%s já está instalado na versão mais recente (%s).", tool.Name, elapsed))
		} else if rebootRequired {
			sp.Stop(true, fmt.Sprintf("%s instalado com sucesso em %s (reinicialização recomendada).", tool.Name, elapsed))
		} else {
			sp.Stop(true, fmt.Sprintf("%s instalado com sucesso em %s.", tool.Name, elapsed))
		}
	} else if !quiet {
		if alreadyInstalled {
			fmt.Printf("✔ %s já está instalado na versão mais recente (%s).\n", tool.Name, elapsed)
		} else if rebootRequired {
			fmt.Printf("✔ %s instalado com sucesso em %s (reinicialização recomendada).\n", tool.Name, elapsed)
		} else {
			fmt.Printf("✔ %s instalado com sucesso em %s.\n", tool.Name, elapsed)
		}
	}
	LogOp("install %s fim rc=0 duracao=%s ja_instalado=%v reboot=%v", tool.ID, elapsed, alreadyInstalled, rebootRequired)
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
	if (r.sysInfo != nil && r.sysInfo.OS == system.OSWindows || runtime.GOOS == "windows") && target.Binary != "" {
		if winPath := findWindowsTargetBinaryPath(tool, target); winPath != "" {
			return winPath
		}
	}
	if target.Binary != "" {
		if p, err := r.executor.LookPath(target.Binary); err == nil {
			return p
		}
	}
	return ""
}

func findWindowsTargetBinaryPath(tool catalog.Tool, target *catalog.Target) string {
	progFiles := os.Getenv("ProgramFiles")
	progFilesX86 := os.Getenv("ProgramFiles(x86)")
	localApp := os.Getenv("LOCALAPPDATA")
	appData := os.Getenv("APPDATA")
	winDir := os.Getenv("WINDIR")

	// 1. Fontes (.ttf / .otf ou categoria Fontes)
	lowerBin := strings.ToLower(target.Binary)
	if tool.Category == "Fontes" || strings.HasSuffix(lowerBin, ".ttf") || strings.HasSuffix(lowerBin, ".otf") {
		var fontDirs []string
		if localApp != "" {
			fontDirs = append(fontDirs, filepath.Join(localApp, "Microsoft", "Windows", "Fonts"))
		}
		if winDir != "" {
			fontDirs = append(fontDirs, filepath.Join(winDir, "Fonts"))
		}
		prefix := strings.TrimSuffix(target.Binary, filepath.Ext(target.Binary))
		for _, dir := range fontDirs {
			exact := filepath.Join(dir, target.Binary)
			if _, err := os.Stat(exact); err == nil {
				return exact
			}
			if prefix != "" {
				for _, ext := range []string{"*.ttf", "*.otf"} {
					if matches, err := filepath.Glob(filepath.Join(dir, prefix+ext)); err == nil && len(matches) > 0 {
						return matches[0]
					}
				}
			}
		}
	}

	// 2. Diretórios padrão de instalação Windows (Program Files, LocalAppData, AppData)
	var baseDirs []string
	for _, b := range []string{progFiles, progFilesX86} {
		if b != "" {
			baseDirs = append(baseDirs, b)
		}
	}
	if localApp != "" {
		baseDirs = append(baseDirs, filepath.Join(localApp, "Programs"), localApp)
	}
	if appData != "" {
		baseDirs = append(baseDirs, appData)
	}

	var vendor, product string
	if target.Type == catalog.TypeWinget && strings.Contains(target.PackageID, ".") {
		parts := strings.Split(target.PackageID, ".")
		vendor = parts[0]
		product = parts[1]
	}

	for _, base := range baseDirs {
		candidates := []string{
			filepath.Join(base, tool.Name, target.Binary),
			filepath.Join(base, tool.Name, "bin", target.Binary),
			filepath.Join(base, tool.ID, target.Binary),
			filepath.Join(base, tool.ID, "bin", target.Binary),
		}
		if vendor != "" && product != "" {
			candidates = append(candidates,
				filepath.Join(base, vendor, product, target.Binary),
				filepath.Join(base, vendor, product, "bin", target.Binary),
				filepath.Join(base, product, target.Binary),
				filepath.Join(base, product, "bin", target.Binary),
			)
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}

	// 3. Layouts específicos conhecidos de instaladores Windows
	var explicit []string
	var globs []string
	for _, pf := range []string{progFiles, progFilesX86} {
		if pf == "" {
			continue
		}
		switch tool.ID {
		case "clang":
			explicit = append(explicit, filepath.Join(pf, "LLVM", "bin", target.Binary))
		case "docker":
			explicit = append(explicit, filepath.Join(pf, "Docker", "Docker", "resources", "bin", target.Binary))
		case "dockerdesktop":
			explicit = append(explicit, filepath.Join(pf, "Docker", "Docker", target.Binary))
		case "obs":
			explicit = append(explicit, filepath.Join(pf, "obs-studio", "bin", "64bit", target.Binary))
		case "vlc":
			explicit = append(explicit, filepath.Join(pf, "VideoLAN", "VLC", target.Binary))
		case "git":
			explicit = append(explicit, filepath.Join(pf, "Git", "cmd", target.Binary), filepath.Join(pf, "Git", "bin", target.Binary))
		case "gh":
			explicit = append(explicit, filepath.Join(pf, "GitHub CLI", target.Binary))
		case "golang":
			explicit = append(explicit, filepath.Join(pf, "Go", "bin", target.Binary))
		case "nodejs":
			explicit = append(explicit, filepath.Join(pf, "nodejs", target.Binary))
		case "neovim":
			explicit = append(explicit, filepath.Join(pf, "Neovim", "bin", target.Binary))
		case "openssh":
			explicit = append(explicit, filepath.Join(pf, "OpenSSH", target.Binary))
		case "vscode":
			explicit = append(explicit, filepath.Join(pf, "Microsoft VS Code", "bin", target.Binary))
		case "temurin":
			globs = append(globs, filepath.Join(pf, "Eclipse Adoptium", "jdk-*", "bin", target.Binary))
		}
	}

	if localApp != "" {
		switch tool.ID {
		case "craftlauncher":
			explicit = append(explicit, filepath.Join(localApp, "Programs", "ArtCraft", "Launcher", "artcraft-launcher.exe"))
		case "dbeaver":
			explicit = append(explicit, filepath.Join(localApp, "DBeaver", target.Binary))
		case "discord":
			globs = append(globs, filepath.Join(localApp, "Discord", "app-*", target.Binary))
			explicit = append(explicit, filepath.Join(localApp, "Discord", "Update.exe"))
		case "jetbrainstoolbox":
			explicit = append(explicit, filepath.Join(localApp, "JetBrains", "Toolbox", "bin", target.Binary))
		case "vscode":
			explicit = append(explicit, filepath.Join(localApp, "Programs", "Microsoft VS Code", "bin", target.Binary))
		case "copilot":
			explicit = append(explicit, filepath.Join(localApp, "Programs", "Microsoft VS Code", "bin", "copilot-shim", target.Binary))
		}
	}

	if appData != "" {
		if tool.ID == "telegram" {
			explicit = append(explicit, filepath.Join(appData, "Telegram Desktop", target.Binary))
		}
		if target.Type == catalog.TypeNpm {
			explicit = append(explicit, filepath.Join(appData, "npm", target.Binary))
		}
	}

	for _, p := range explicit {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, pattern := range globs {
		if matches, err := filepath.Glob(pattern); err == nil && len(matches) > 0 {
			return matches[len(matches)-1]
		}
	}

	// 4. WinGet Links & Portable Packages
	if localApp != "" {
		wingetLink := filepath.Join(localApp, "Microsoft", "WinGet", "Links", target.Binary)
		if _, err := os.Stat(wingetLink); err == nil {
			return wingetLink
		}
		if target.Type == catalog.TypeWinget && target.PackageID != "" {
			pkgPattern := filepath.Join(localApp, "Microsoft", "WinGet", "Packages", target.PackageID+"_*")
			if pkgDirs, err := filepath.Glob(pkgPattern); err == nil {
				for _, pkgDir := range pkgDirs {
					for _, sub := range []string{
						filepath.Join(pkgDir, target.Binary),
						filepath.Join(pkgDir, "bin", target.Binary),
						filepath.Join(pkgDir, "mingw64", "bin", target.Binary),
					} {
						if _, err := os.Stat(sub); err == nil {
							return sub
						}
					}
					for _, subGlob := range []string{
						filepath.Join(pkgDir, "*", "bin", target.Binary),
						filepath.Join(pkgDir, "*", target.Binary),
					} {
						if matches, err := filepath.Glob(subGlob); err == nil && len(matches) > 0 {
							return matches[len(matches)-1]
						}
					}
				}
			}
		}
	}

	return ""
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

	if showVisual && target.Type == catalog.TypeNative && r.sysInfo.OS == system.OSLinux && r.sysInfo.NeedsSudo && !r.dryRun {
		_ = r.executor.Execute("sudo", "-v")
	}

	var sp *Spinner
	if showVisual && !r.verbose {
		sp = NewSpinner(prefix, fmt.Sprintf("Desinstalando %s...", tool.Name))
		sp.Start()
	} else if prefix != "" {
		fmt.Printf("➜ %s Desinstalando %s...\n", prefix, tool.Name)
	} else {
		fmt.Printf("=== Desinstalando %s (%s) ===\n", tool.Name, tool.ID)
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
		if r.haveCommand("winget") {
			err = r.execWithActivity(actFn, "winget", "uninstall", "--exact", "--id", target.PackageID, "--silent")
			if isWingetPackageNotFound(err) {
				LogOp("uninstall %s winget pacote ja ausente (0x8a150014/0x8a15002f), limpando registro", tool.ID)
				err = nil
			}
		} else {
			LogOp("uninstall %s winget ausente no PATH, limpando registro", tool.ID)
		}

	case catalog.TypeNative:
		err = r.uninstallNativeWithActivity(actFn, target.PackageID)

	case catalog.TypeNpm:
		if r.haveCommand("npm") {
			err = r.execWithActivity(actFn, "npm", "uninstall", "-g", target.PackageID)
		} else {
			LogOp("uninstall %s npm ausente no PATH, limpando registro", tool.ID)
		}

	case catalog.TypePipx:
		if r.haveCommand("pipx") {
			pkgName := target.PackageID
			if strings.HasPrefix(pkgName, "git+") || strings.HasPrefix(pkgName, "http") {
				pkgName = strings.TrimSuffix(target.Binary, ".exe")
			}
			err = r.execWithActivity(actFn, "pipx", "uninstall", pkgName)
		} else {
			LogOp("uninstall %s pipx ausente no PATH, limpando registro", tool.ID)
		}

	case catalog.TypeCargo:
		if r.haveCommand("cargo") {
			err = r.execWithActivity(actFn, "cargo", "uninstall", target.PackageID)
		} else {
			LogOp("uninstall %s cargo ausente no PATH, limpando registro", tool.ID)
		}

	case catalog.TypeFlatpak:
		if r.haveCommand("flatpak") {
			err = r.execWithActivity(actFn, "flatpak", "--noninteractive", "uninstall", "-y", target.PackageID)
		} else {
			LogOp("uninstall %s flatpak ausente no PATH, limpando registro", tool.ID)
		}

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
		} else if tool.ID == "craftlauncher" {
			localApp := os.Getenv("LOCALAPPDATA")
			if localApp != "" {
				_ = os.Remove(filepath.Join(localApp, "multitool", "bin", "artcraft-launcher.bat"))
				_ = os.Remove(filepath.Join(localApp, "multitool", "bin", "craft-launcher.bat"))
				_ = os.Remove(filepath.Join(localApp, "multitool", "bin", "craftlauncher.bat"))
			}
			err = r.reg.MarkUninstalled(tool.ID)
		} else if tool.ID == "orca" || tool.ID == "openmausbot" {
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
		} else if tool.ID == "goose" {
			localApp := os.Getenv("LOCALAPPDATA")
			if localApp != "" {
				_ = os.RemoveAll(filepath.Join(localApp, "multitool", "goose"))
				_ = os.Remove(filepath.Join(localApp, "multitool", "bin", "goose.exe"))
				_ = os.Remove(filepath.Join(localApp, "multitool", "bin", "goose.bat"))
			}
			err = r.reg.MarkUninstalled(tool.ID)
		} else if tool.ID == "ohmypi" {
			localApp := os.Getenv("LOCALAPPDATA")
			if localApp != "" {
				_ = os.Remove(filepath.Join(localApp, "multitool", "bin", "omp.exe"))
			}
			err = r.reg.MarkUninstalled(tool.ID)
		} else if tool.ID == "atkinson" || tool.ID == "sourcesans3" || tool.ID == "sourceserif4" {
			localApp := os.Getenv("LOCALAPPDATA")
			if localApp != "" {
				_ = os.RemoveAll(filepath.Join(localApp, "multitool", "fonts", tool.ID))
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
		if r.haveCommand("brew") {
			err = r.execWithActivity(actFn, "brew", "uninstall", target.PackageID)
		} else {
			LogOp("uninstall %s brew ausente no PATH, limpando registro", tool.ID)
		}

	default:
		err = fmt.Errorf("método de desinstalação não suportado: %s", target.Type)
	}

	// Se o gerenciador retornou erro mas a ferramenta já não está presente no host, considera já removida
	if err != nil && tool.ID != "rustup" && !r.isToolPresentOnHost(tool, target) {
		LogOp("uninstall %s ferramenta ja ausente no host apos erro (%v), limpando registro", tool.ID, err)
		err = nil
	}

	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		if sp != nil {
			sp.Stop(false, fmt.Sprintf("Falha ao desinstalar %s (%s): %v", tool.Name, elapsed, err))
		} else {
			fmt.Printf("✖ Falha ao desinstalar %s (%s): %v\n", tool.Name, elapsed, err)
		}
		printCommandDiagnostics(err)
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

	if showVisual && target.Type == catalog.TypeNative && r.sysInfo.OS == system.OSLinux && r.sysInfo.NeedsSudo && !r.dryRun {
		_ = r.executor.Execute("sudo", "-v")
	}

	var sp *Spinner
	if showVisual && !r.verbose {
		sp = NewSpinner(prefix, fmt.Sprintf("Atualizando %s via %s...", tool.Name, target.Type))
		sp.Start()
	} else if prefix != "" {
		fmt.Printf("➜ %s Atualizando %s via %s (%s)...\n", prefix, tool.Name, target.Type, target.PackageID)
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
		printCommandDiagnostics(err)
		LogOp("update %s fim rc=1 erro_runtime=%v", tool.ID, err)
		return err
	}

	if sp != nil {
		sp.Update(fmt.Sprintf("Atualizando %s via %s (%s)...", tool.Name, target.Type, target.PackageID))
	}

	var actFn func(string)
	if sp != nil {
		actFn = func(line string) {
			sp.UpdateActivity(line)
		}
	}

	var err error
	var alreadyUpToDate bool
	var rebootRequired bool
	switch target.Type {
	case catalog.TypeWinget:
		err = r.execWithActivity(actFn, "winget", "upgrade", "--exact", "--id", target.PackageID,
			"--accept-source-agreements", "--accept-package-agreements", "--silent")
		if isWingetAlreadyInstalled(err) {
			err = nil
			alreadyUpToDate = true
		} else if isWingetRebootRequired(err) {
			err = nil
			rebootRequired = true
		}

	case catalog.TypeNative:
		err = r.updateNativeWithActivity(actFn, target.PackageID)

	case catalog.TypeNpm:
		err = r.execWithActivity(actFn, "npm", "update", "-g", target.PackageID)

	case catalog.TypePipx:
		if strings.HasPrefix(target.PackageID, "git+") || strings.HasPrefix(target.PackageID, "http") {
			err = r.execWithActivity(actFn, "pipx", "install", "--force", target.PackageID)
		} else {
			err = r.execWithActivity(actFn, "pipx", "upgrade", target.PackageID)
		}

	case catalog.TypeCargo:
		err = r.execWithActivity(actFn, "cargo", "install", target.PackageID)

	case catalog.TypeFlatpak:
		err = r.execWithActivity(actFn, "flatpak", "--noninteractive", "update", "-y", target.PackageID)

	case catalog.TypeSkill:
		err = r.installGitSkillWithActivity(actFn, tool.ID, target.PackageID)

	case catalog.TypeDirect:
		if isDirectWindowsBinary(tool.ID) {
			err = r.installDirectWindowsTargetWithActivity(actFn, tool.ID, target)
		} else {
			err = r.installGitSkillWithActivity(actFn, tool.ID, target.PackageID)
		}

	case catalog.TypeScript:
		err = r.installScriptWithActivity(actFn, tool.ID, target)

	case catalog.TypeBrew:
		err = r.execWithActivity(actFn, "brew", "upgrade", target.PackageID)

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
		printCommandDiagnostics(err)
		LogOp("update %s fim rc=1 erro=%v", tool.ID, err)
		return err
	}

	if sp != nil {
		if alreadyUpToDate {
			sp.Stop(true, fmt.Sprintf("%s já está na versão mais recente (%s).", tool.Name, elapsed))
		} else if rebootRequired {
			sp.Stop(true, fmt.Sprintf("%s atualizado com sucesso em %s (reinicialização recomendada).", tool.Name, elapsed))
		} else {
			sp.Stop(true, fmt.Sprintf("%s atualizado com sucesso em %s.", tool.Name, elapsed))
		}
	} else {
		if alreadyUpToDate {
			fmt.Printf("✔ %s já está na versão mais recente (%s).\n", tool.Name, elapsed)
		} else if rebootRequired {
			fmt.Printf("✔ %s atualizado com sucesso em %s (reinicialização recomendada).\n", tool.Name, elapsed)
		} else {
			fmt.Printf("✔ %s atualizado com sucesso em %s.\n", tool.Name, elapsed)
		}
	}
	LogOp("update %s fim rc=0 duracao=%s ja_atualizado=%v reboot=%v", tool.ID, elapsed, alreadyUpToDate, rebootRequired)
	return nil
}

func (r *Runner) RunBatchInstall(tools []catalog.Tool) (int, int) {
	ok, fail, _ := r.RunBatchInstallWithErrors(tools)
	return ok, fail
}

func (r *Runner) RunBatchInstallWithErrors(tools []catalog.Tool) (int, int, error) {
	total := len(tools)
	if total == 0 {
		return 0, 0, nil
	}

	if !r.verbose && total > 1 {
		// Se o lote consistir puramente de skills Git (TypeSkill ou TypeDirect skill no Windows): executa em pool concorrente
		allSkills := true
		for _, t := range tools {
			tgt := r.targetForTool(t)
			if !isGitSkillTarget(t.ID, tgt) {
				allSkills = false
				break
			}
		}
		if allSkills {
			return r.runBatchInstallPool(tools, "skills", "Clonando", 4)
		}

		// Se o lote consistir puramente de downloads portáteis TypeDirect (sem trava MSI/NSIS): executa em pool concorrente
		allDirect := true
		for _, t := range tools {
			tgt := r.targetForTool(t)
			if !r.isParallelDirectTarget(t, tgt) {
				allDirect = false
				break
			}
		}
		if allDirect {
			return r.runBatchInstallPool(tools, "downloads diretos", "Baixando", 4)
		}
	}

	return r.runBatchInstallPartitioned(tools)
}

func (r *Runner) runBatchInstallPool(tools []catalog.Tool, label, verb string, maxWorkers int) (int, int, error) {
	var failures []error
	total := len(tools)
	overallStart := time.Now()
	if maxWorkers <= 0 {
		maxWorkers = 4
	}

	sp := NewSpinner("", fmt.Sprintf("%s concorrentemente %d %s (worker pool de %d)...", verb, total, label, maxWorkers))
	sp.Start()

	type poolResult struct {
		tool catalog.Tool
		err  error
		dur  time.Duration
	}

	results := make(chan poolResult, total)
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for _, t := range tools {
		wg.Add(1)
		go func(tool catalog.Tool) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			err := r.installInternalWithQuiet(tool, "", false, true)
			results <- poolResult{
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
		sp.Update(fmt.Sprintf("%s [%d/%d %s] %s concorrentemente...", bar, count, total, label, verb))

		if res.err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", res.tool.ID, res.err))
			fail++
			sp.PrintLine(fmt.Sprintf("  ↳ %s [%d/%d] %s (%s): %v",
				styleError.Render("✖"), count, total, res.tool.Name, res.dur, res.err))
			printCommandDiagnosticsToSpinner(sp, res.err)
		} else {
			ok++
			sp.PrintLine(fmt.Sprintf("  ↳ %s [%d/%d] %s (%s)",
				styleSuccess.Render("✔"), count, total, res.tool.Name, res.dur))
		}
	}

	overallDur := time.Since(overallStart).Round(time.Millisecond)
	sp.Stop(fail == 0, fmt.Sprintf("Resumo (%s): %d instalado(s), %d falha(s) em %s.", label, ok, fail, overallDur))
	return ok, fail, errors.Join(failures...)
}

func (r *Runner) runBatchInstallPartitioned(tools []catalog.Tool) (int, int, error) {
	var failures []error
	ok, fail := 0, 0
	total := len(tools)
	overallStart := time.Now()

	stopSudo := func() {}
	if r.batchNeedsSudo(tools, true) {
		stopSudo = r.startSudoKeepAlive()
	}
	defer stopSudo()

	i := 0
	for i < total {
		t := tools[i]
		tgt := r.targetForTool(t)

		if !r.verbose {
			// Bloco contíguo de skills Git (> 1)
			if isGitSkillTarget(t.ID, tgt) {
				j := i
				for j < total {
					curTgt := r.targetForTool(tools[j])
					if !isGitSkillTarget(tools[j].ID, curTgt) {
						break
					}
					j++
				}

				skillChunk := tools[i:j]
				if len(skillChunk) > 1 {
					chunkOk, chunkFail, chunkErr := r.runBatchInstallPool(skillChunk, "skills", "Clonando", 4)
					if chunkErr != nil {
						failures = append(failures, chunkErr)
					}
					ok += chunkOk
					fail += chunkFail
					i = j
					continue
				}
			}

			// Bloco contíguo de downloads portáteis TypeDirect (> 1) com dependências satisfeitas
			if r.isParallelDirectTarget(t, tgt) {
				j := i
				for j < total {
					curTgt := r.targetForTool(tools[j])
					if !r.isParallelDirectTarget(tools[j], curTgt) {
						break
					}
					j++
				}

				directChunk := tools[i:j]
				if len(directChunk) > 1 {
					chunkOk, chunkFail, chunkErr := r.runBatchInstallPool(directChunk, "downloads diretos", "Baixando", 4)
					if chunkErr != nil {
						failures = append(failures, chunkErr)
					}
					ok += chunkOk
					fail += chunkFail
					i = j
					continue
				}
			}
		}

		pct := float64(i) / float64(total)
		prefix := fmt.Sprintf("[%d/%d] %s", i+1, total, RenderProgressBar(14, pct))
		if err := r.installInternal(t, prefix, true); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", t.ID, err))
			fail++
		} else {
			ok++
		}
		i++
	}

	overallDur := time.Since(overallStart).Round(time.Millisecond)
	fmt.Printf("\nResumo do Lote: %s %d instalado(s), %d falha(s) em %s.\n",
		RenderProgressBar(14, 1.0), ok, fail, overallDur)
	return ok, fail, errors.Join(failures...)
}

func (r *Runner) RunBatchUninstall(tools []catalog.Tool) (int, int) {
	ok, fail, _ := r.RunBatchUninstallWithErrors(tools)
	return ok, fail
}

func (r *Runner) RunBatchUninstallWithErrors(tools []catalog.Tool) (int, int, error) {
	var failures []error
	ok, fail := 0, 0
	total := len(tools)
	overallStart := time.Now()

	stopSudo := func() {}
	if r.batchNeedsSudo(tools, false) {
		stopSudo = r.startSudoKeepAlive()
	}
	defer stopSudo()

	for i, t := range tools {
		pct := float64(i) / float64(total)
		prefix := fmt.Sprintf("[%d/%d] %s", i+1, total, RenderProgressBar(14, pct))
		if err := r.uninstallInternal(t, prefix, true); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", t.ID, err))
			fail++
		} else {
			ok++
		}
	}
	overallDur := time.Since(overallStart).Round(time.Millisecond)
	fmt.Printf("\nResumo da Remoção: %s %d removido(s), %d falha(s) em %s.\n",
		RenderProgressBar(14, 1.0), ok, fail, overallDur)
	return ok, fail, errors.Join(failures...)
}

func (r *Runner) RunBatchUpdate(tools []catalog.Tool) (int, int) {
	ok, fail, _ := r.RunBatchUpdateWithErrors(tools)
	return ok, fail
}

func (r *Runner) RunBatchUpdateWithErrors(tools []catalog.Tool) (int, int, error) {
	var failures []error
	ok, fail := 0, 0
	total := len(tools)
	overallStart := time.Now()

	stopSudo := func() {}
	if r.batchNeedsSudo(tools, true) {
		stopSudo = r.startSudoKeepAlive()
	}
	defer stopSudo()

	for i, t := range tools {
		pct := float64(i) / float64(total)
		prefix := fmt.Sprintf("[%d/%d] %s", i+1, total, RenderProgressBar(14, pct))
		if err := r.updateInternal(t, prefix, true); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", t.ID, err))
			fail++
		} else {
			ok++
		}
	}
	overallDur := time.Since(overallStart).Round(time.Millisecond)
	fmt.Printf("\nResumo da Atualização: %s %d atualizado(s), %d falha(s) em %s.\n",
		RenderProgressBar(14, 1.0), ok, fail, overallDur)
	return ok, fail, errors.Join(failures...)
}
