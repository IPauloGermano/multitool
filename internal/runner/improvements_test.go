package runner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
	"github.com/IPauloGermano/multitool/internal/system"
)

type spyActivityExecutor struct {
	*MockExecutor
	activityCalls  int
	nilActivity    int
	nonNilActivity int
	errByCmd       map[string]error
}

func (s *spyActivityExecutor) ExecuteWithActivity(actFn func(string), name string, args ...string) error {
	s.mu.Lock()
	s.activityCalls++
	if actFn == nil {
		s.nilActivity++
	} else {
		s.nonNilActivity++
	}
	s.mu.Unlock()

	cmdKey := strings.Join(append([]string{name}, args...), " ")
	for k, err := range s.errByCmd {
		if strings.Contains(cmdKey, k) {
			s.mu.Lock()
			s.executed = append(s.executed, append([]string{name}, args...))
			s.mu.Unlock()
			return err
		}
	}
	return s.MockExecutor.Execute(name, args...)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	_ = w.Close()
	os.Stdout = old
	out := <-done
	_ = r.Close()
	return out
}

func TestOSExecutorReturnsCommandErrorWithoutPrinting(t *testing.T) {
	execOS := &OSExecutor{}
	var seenLines []string
	var runErr error

	printed := captureStdout(t, func() {
		// 'go' sempre existe no ambiente de teste e falha rapidamente com subcomando inexistente.
		runErr = execOS.ExecuteWithActivity(func(line string) {
			seenLines = append(seenLines, line)
		}, "go", "subcomando-inexistente-multitool-test")
	})

	if runErr == nil {
		t.Fatal("esperava erro ao executar subcomando inválido do go")
	}
	if printed != "" {
		t.Errorf("OSExecutor não deve imprimir diretamente no stdout; imprimiu: %q", printed)
	}

	var cmdErr *CommandError
	if !errors.As(runErr, &cmdErr) {
		t.Fatalf("esperava *CommandError, obteve %T: %v", runErr, runErr)
	}
	if len(cmdErr.RecentLines) == 0 {
		t.Errorf("esperava RecentLines preenchido no CommandError")
	}
	if len(seenLines) == 0 {
		t.Errorf("esperava activityFn acionada durante a execução")
	}

	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) {
		t.Errorf("CommandError.Unwrap deveria preservar *exec.ExitError")
	}
}

func TestWingetAlreadyInstalledSuppressesDiagnostics(t *testing.T) {
	reg, err := registry.New(filepath.Join(t.TempDir(), "installed.json"))
	if err != nil {
		t.Fatal(err)
	}

	tool := catalog.FindTool("cmake")
	if tool == nil {
		t.Fatal("ferramenta cmake não encontrada")
	}

	spy := &spyActivityExecutor{
		MockExecutor: &MockExecutor{},
		errByCmd: map[string]error{
			"winget install": &CommandError{
				Command: "winget",
				Err:     fmt.Errorf("exit status 0x8a15002b"),
				RecentLines: []string{
					"Found an existing package already installed. Trying to upgrade the installed package...",
					"No available upgrade found.",
					"No newer package versions are available from the configured sources",
				},
			},
		},
	}

	r := NewWithExecutor(reg, spy, false)
	r.sysInfo.OS = system.OSWindows

	var installErr error
	out := captureStdout(t, func() {
		installErr = r.Install(*tool)
	})

	if installErr != nil {
		t.Fatalf("esperava sucesso idempotente para 0x8a15002b, obteve: %v", installErr)
	}
	if strings.Contains(out, "No available upgrade found") || strings.Contains(out, "│") {
		t.Errorf("não deveria imprimir diagnóstico de erro para código benigno do winget; saída: %s", out)
	}
	if !reg.IsInstalled("cmake") {
		t.Errorf("cmake deveria constar no registro após 0x8a15002b")
	}
}

func TestRealErrorPrintsDiagnosticsAfterFailure(t *testing.T) {
	reg, err := registry.New(filepath.Join(t.TempDir(), "installed.json"))
	if err != nil {
		t.Fatal(err)
	}

	tool := catalog.FindTool("cmake")
	if tool == nil {
		t.Fatal("ferramenta cmake não encontrada")
	}

	spy := &spyActivityExecutor{
		MockExecutor: &MockExecutor{},
		errByCmd: map[string]error{
			"winget install": &CommandError{
				Command:     "winget",
				Err:         fmt.Errorf("exit status 1"),
				RecentLines: []string{"fatal: network unreachable"},
			},
		},
	}

	r := NewWithExecutor(reg, spy, false)
	r.sysInfo.OS = system.OSWindows

	var installErr error
	out := captureStdout(t, func() {
		installErr = r.Install(*tool)
	})

	if installErr == nil {
		t.Fatal("esperava falha na instalação")
	}
	failIdx := strings.Index(out, "Falha ao instalar")
	diagIdx := strings.Index(out, "fatal: network unreachable")
	if failIdx == -1 || diagIdx == -1 || diagIdx < failIdx {
		t.Errorf("diagnóstico deve ser exibido APÓS a mensagem de falha; saída:\n%s", out)
	}
}

func TestFastPathSkipAndForceReinstall(t *testing.T) {
	reg, err := registry.New(filepath.Join(t.TempDir(), "installed.json"))
	if err != nil {
		t.Fatal(err)
	}

	tool := catalog.FindTool("cmake")
	if tool == nil {
		t.Fatal("ferramenta cmake não encontrada")
	}

	mock := &MockExecutor{
		present: map[string]bool{"cmake.exe": true, "cmake": true},
	}
	r := NewWithExecutor(reg, mock, false)
	r.sysInfo.OS = system.OSWindows

	// 1. Sem --force: deve pular instantaneamente (0 comandos) e sincronizar o registro
	if err := r.Install(*tool); err != nil {
		t.Fatalf("erro inesperado no fast-path: %v", err)
	}
	if len(mock.executed) != 0 {
		t.Fatalf("fast-path não deveria invocar winget; executou: %v", mock.executed)
	}
	if !reg.IsInstalled("cmake") {
		t.Fatalf("fast-path deveria registrar cmake em installed.json")
	}

	// 2. Com --force: deve executar winget install com --force
	r.SetForce(true)
	if err := r.Install(*tool); err != nil {
		t.Fatalf("erro inesperado com --force: %v", err)
	}
	if len(mock.executed) == 0 {
		t.Fatalf("com --force deveria invocar winget install")
	}
	cmdStr := strings.Join(mock.executed[len(mock.executed)-1], " ")
	if !strings.Contains(cmdStr, "winget install") || !strings.Contains(cmdStr, "--force") {
		t.Errorf("esperava winget install com --force, obteve: %s", cmdStr)
	}
}

func TestFastPathWindowsProgramFilesBinDetection(t *testing.T) {
	root := t.TempDir()
	progFiles := filepath.Join(root, "Program Files")
	t.Setenv("ProgramFiles", progFiles)
	cmakeBin := filepath.Join(progFiles, "CMake", "bin", "cmake.exe")
	if err := os.MkdirAll(filepath.Dir(cmakeBin), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cmakeBin, []byte("stub"), 0755); err != nil {
		t.Fatal(err)
	}

	reg, _ := registry.New(filepath.Join(root, "installed.json"))
	mock := &MockExecutor{}
	r := NewWithExecutor(reg, mock, false)
	r.sysInfo.OS = system.OSWindows

	tool := catalog.FindTool("cmake")
	if err := r.Install(*tool); err != nil {
		t.Fatalf("erro no fast-path ProgramFiles\\bin: %v", err)
	}
	if len(mock.executed) != 0 {
		t.Errorf("deveria ter detectado CMake\\bin\\cmake.exe sem acionar winget; executou: %v", mock.executed)
	}
}

func TestVerboseModeStreamsDirectly(t *testing.T) {
	reg, _ := registry.New(filepath.Join(t.TempDir(), "installed.json"))
	tool := catalog.FindTool("repomix")

	spy := &spyActivityExecutor{MockExecutor: &MockExecutor{}}
	r := NewWithExecutor(reg, spy, false)
	r.SetVerbose(true)

	if err := r.Install(*tool); err != nil {
		t.Fatalf("erro inesperado em modo verbose: %v", err)
	}
	if spy.nilActivity == 0 {
		t.Errorf("em modo verbose, activityFn deve ser nil para streaming direto")
	}
	if spy.nonNilActivity != 0 {
		t.Errorf("em modo verbose, não deveria usar activityFn capturada")
	}
}

func TestParallelDirectPortableDownloadsAndWindowsSkills(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))
	reg, _ := registry.New(filepath.Join(root, "installed.json"))

	var inflight atomic.Int32
	var maxInflight atomic.Int32

	mock := &MockExecutor{
		beforeExecute: func(name string, args []string) error {
			if name == "curl.exe" {
				cur := inflight.Add(1)
				for {
					prev := maxInflight.Load()
					if cur <= prev || maxInflight.CompareAndSwap(prev, cur) {
						break
					}
				}
				time.Sleep(40 * time.Millisecond)
				inflight.Add(-1)
			}
			return nil
		},
	}
	// Desbloqueia concorrência fora do mutex do MockExecutor usando wrapper concorrente
	concExec := &concurrentDownloadExecutor{MockExecutor: mock, inflight: &inflight, maxInflight: &maxInflight}
	r := NewWithExecutor(reg, concExec, false)
	r.sysInfo.OS = system.OSWindows
	r.sysInfo.Arch = "amd64"

	apktool := catalog.FindTool("apktool")
	ghidra := catalog.FindTool("ghidra")
	wabt := catalog.FindTool("wabt")
	craft := catalog.FindTool("craftlauncher")
	if apktool == nil || ghidra == nil || wabt == nil || craft == nil {
		t.Fatal("ferramentas TypeDirect de teste não encontradas")
	}

	if r.isParallelDirectTarget(*craft, craft.Windows) {
		t.Errorf("craftlauncher (MSI) NÃO deve ser marcado como seguro para paralelismo")
	}
	if !r.isParallelDirectTarget(*wabt, wabt.Windows) {
		t.Errorf("wabt deveria ser elegível para download direto paralelo")
	}

	ok, fail, err := r.RunBatchInstallWithErrors([]catalog.Tool{*apktool, *ghidra, *wabt})
	if err != nil || ok != 3 || fail != 0 {
		t.Fatalf("esperava ok=3 fail=0 err=nil, obteve ok=%d fail=%d err=%v", ok, fail, err)
	}
	if maxInflight.Load() < 2 {
		t.Errorf("esperava execução concorrente (maxInflight >= 2) para downloads portáteis TypeDirect, obteve %d", maxInflight.Load())
	}

	// Verifica também que skills no Windows (TypeDirect) são reconhecidas como git skills
	sp := catalog.FindTool("superpowers")
	if sp == nil || !isGitSkillTarget(sp.ID, sp.Windows) {
		t.Errorf("superpowers no Windows deveria ser reconhecida por isGitSkillTarget")
	}
}

type concurrentDownloadExecutor struct {
	*MockExecutor
	inflight    *atomic.Int32
	maxInflight *atomic.Int32
}

func (c *concurrentDownloadExecutor) Execute(name string, args ...string) error {
	if name == "curl.exe" {
		cur := c.inflight.Add(1)
		for {
			prev := c.maxInflight.Load()
			if cur <= prev || c.maxInflight.CompareAndSwap(prev, cur) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		c.inflight.Add(-1)
	}
	return c.MockExecutor.Execute(name, args...)
}

func TestDoctorAuditReport(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	localApp := filepath.Join(root, "AppData", "Local")
	t.Setenv("LOCALAPPDATA", localApp)
	t.Setenv("PATH", filepath.Join(root, "empty-path"))

	reg, err := registry.New(filepath.Join(root, "installed.json"))
	if err != nil {
		t.Fatal(err)
	}
	_ = reg.MarkInstalled("git")
	_ = reg.MarkInstalled("cmake")
	_ = reg.MarkInstalled("ferramenta-inexistente")
	_ = reg.MarkInstalled("superpowers")

	// Cria local bin com arquivo mas fora do PATH -> deve gerar DoctorWarn
	localBin := filepath.Join(localApp, "multitool", "bin")
	_ = os.MkdirAll(localBin, 0755)
	_ = os.WriteFile(filepath.Join(localBin, "apktool.bat"), []byte("@echo off"), 0755)

	mock := &MockExecutor{
		present: map[string]bool{"git.exe": true, "git": true},
		missing: map[string]bool{"cmake.exe": true, "cmake": true},
	}
	r := NewWithExecutor(reg, mock, false)
	r.sysInfo.OS = system.OSWindows

	agentSkills := filepath.Join(root, ".claude", "skills")
	_ = os.MkdirAll(agentSkills, 0755)
	r.sysInfo.SkillDirs = []string{agentSkills}

	// Cria diretório da skill superpowers, mas sem link no agente -> deve gerar DoctorWarn
	skillSrc := r.getSkillDir("superpowers")
	_ = os.MkdirAll(skillSrc, 0755)
	_ = os.WriteFile(filepath.Join(skillSrc, "SKILL.md"), []byte("# test"), 0644)

	rep := r.Doctor()

	findCheck := func(list []DoctorCheck, name string) *DoctorCheck {
		for i := range list {
			if list[i].Name == name {
				return &list[i]
			}
		}
		return nil
	}

	if c := findCheck(rep.Tools, "git"); c == nil || c.Status != DoctorOK {
		t.Errorf("esperava git=ok no doctor, obteve: %+v", c)
	}
	if c := findCheck(rep.Tools, "cmake"); c == nil || c.Status != DoctorFail {
		t.Errorf("esperava cmake=fail no doctor (binário ausente), obteve: %+v", c)
	}
	if c := findCheck(rep.Tools, "ferramenta-inexistente"); c == nil || c.Status != DoctorWarn {
		t.Errorf("esperava ferramenta-inexistente=warn no doctor, obteve: %+v", c)
	}
	if c := findCheck(rep.Skills, "superpowers"); c == nil || c.Status != DoctorWarn {
		t.Errorf("esperava superpowers=warn (link ausente), obteve: %+v", c)
	}
	if c := findCheck(rep.Runtimes, "local-bin-path"); c == nil || c.Status != DoctorWarn {
		t.Errorf("esperava local-bin-path=warn (fora do PATH), obteve: %+v", c)
	}
	if rep.Summary.Fail == 0 || rep.Summary.Warn == 0 || rep.Summary.OK == 0 {
		t.Errorf("resumo do doctor inconsistente: %+v", rep.Summary)
	}
}

func TestFindWindowsTargetBinaryPaths(t *testing.T) {
	root := t.TempDir()
	progFiles := filepath.Join(root, "Program Files")
	localApp := filepath.Join(root, "AppData", "Local")
	appData := filepath.Join(root, "AppData", "Roaming")
	winDir := filepath.Join(root, "Windows")

	t.Setenv("ProgramFiles", progFiles)
	t.Setenv("ProgramFiles(x86)", filepath.Join(root, "Program Files (x86)"))
	t.Setenv("LOCALAPPDATA", localApp)
	t.Setenv("APPDATA", appData)
	t.Setenv("WINDIR", winDir)

	filesToCreate := []string{
		filepath.Join(progFiles, "LLVM", "bin", "clang.exe"),
		filepath.Join(localApp, "DBeaver", "dbeaver.exe"),
		filepath.Join(localApp, "Discord", "app-1.0.9269", "Discord.exe"),
		filepath.Join(progFiles, "Docker", "Docker", "Docker Desktop.exe"),
		filepath.Join(winDir, "Fonts", "JetBrainsMonoNerdFont-Regular.ttf"),
		filepath.Join(localApp, "JetBrains", "Toolbox", "bin", "jetbrains-toolbox.exe"),
		filepath.Join(progFiles, "obs-studio", "bin", "64bit", "obs64.exe"),
		filepath.Join(appData, "Telegram Desktop", "Telegram.exe"),
		filepath.Join(progFiles, "VideoLAN", "VLC", "vlc.exe"),
	}

	for _, f := range filesToCreate {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("stub"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	reg, err := registry.New(filepath.Join(root, "installed.json"))
	if err != nil {
		t.Fatal(err)
	}
	toolIDs := []string{
		"clang", "dbeaver", "discord", "dockerdesktop",
		"jetbrainsmono", "jetbrainstoolbox", "obs", "telegram", "vlc",
	}
	for _, id := range toolIDs {
		_ = reg.MarkInstalled(id)
	}

	mock := &MockExecutor{
		missing: map[string]bool{
			"clang.exe":             true,
			"dbeaver.exe":           true,
			"Discord.exe":           true,
			"Docker Desktop.exe":    true,
			"JetBrainsMono.ttf":     true,
			"jetbrains-toolbox.exe": true,
			"obs64.exe":             true,
			"Telegram.exe":          true,
			"vlc.exe":               true,
		},
	}
	r := NewWithExecutor(reg, mock, false)
	r.sysInfo.OS = system.OSWindows

	rep := r.Doctor()
	for _, check := range rep.Tools {
		if check.Status != DoctorOK {
			t.Errorf("esperava %s=ok no doctor Windows, obteve status=%s detail=%s", check.Name, check.Status, check.Detail)
		}
	}
}
