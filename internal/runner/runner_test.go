package runner

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
)

type MockExecutor struct {
	executed [][]string
	missing  map[string]bool
}

func (m *MockExecutor) Execute(name string, args ...string) error {
	cmd := append([]string{name}, args...)
	m.executed = append(m.executed, cmd)
	return nil
}

func (m *MockExecutor) LookPath(file string) (string, error) {
	if m.missing != nil && m.missing[file] {
		return "", exec.ErrNotFound
	}
	return "/usr/bin/" + file, nil
}

func TestRunnerDryRun(t *testing.T) {
	tempDir := t.TempDir()
	regPath := filepath.Join(tempDir, "installed.json")
	reg, err := registry.New(regPath)
	if err != nil {
		t.Fatalf("Erro ao inicializar registro: %v", err)
	}

	mock := &MockExecutor{}
	r := NewWithExecutor(reg, mock, true)

	tool := catalog.FindTool("git")
	if tool == nil {
		t.Fatalf("Ferramenta 'git' não encontrada")
	}

	// 1. Dry run install
	if err := r.Install(*tool); err != nil {
		t.Fatalf("Erro no install dry-run: %v", err)
	}
	if len(mock.executed) != 0 {
		t.Errorf("Dry-run não deveria ter executado comandos reais, mas executou: %v", mock.executed)
	}
	if !reg.IsInstalled("git") {
		t.Errorf("git deveria estar marcado como instalado mesmo em dry-run")
	}

	// 2. Dry run uninstall
	if err := r.Uninstall(*tool); err != nil {
		t.Fatalf("Erro no uninstall dry-run: %v", err)
	}
	if len(mock.executed) != 0 {
		t.Errorf("Dry-run não deveria ter executado comandos reais")
	}
	if reg.IsInstalled("git") {
		t.Errorf("git deveria estar desmarcado após uninstall")
	}
}

func TestRunnerExecution(t *testing.T) {
	tempDir := t.TempDir()
	regPath := filepath.Join(tempDir, "installed.json")
	reg, _ := registry.New(regPath)

	mock := &MockExecutor{}
	r := NewWithExecutor(reg, mock, false)

	tool := catalog.FindTool("repomix")
	if tool == nil {
		t.Fatalf("Ferramenta 'repomix' não encontrada")
	}

	// Install real via mock
	if err := r.Install(*tool); err != nil {
		t.Fatalf("Erro ao instalar: %v", err)
	}
	if len(mock.executed) == 0 {
		t.Fatalf("Nenhum comando foi executado")
	}

	lastCmd := strings.Join(mock.executed[len(mock.executed)-1], " ")
	if !strings.Contains(lastCmd, "npm") || !strings.Contains(lastCmd, "repomix") {
		t.Errorf("Comando inesperado para repomix: %s", lastCmd)
	}
}

func TestBatchRunner(t *testing.T) {
	tempDir := t.TempDir()
	regPath := filepath.Join(tempDir, "installed.json")
	reg, _ := registry.New(regPath)

	mock := &MockExecutor{}
	r := NewWithExecutor(reg, mock, true) // dry-run

	tools := []catalog.Tool{
		*catalog.FindTool("git"),
		*catalog.FindTool("jq"),
	}

	ok, fail := r.RunBatchInstall(tools)
	if ok != 2 || fail != 0 {
		t.Errorf("Esperava 2 ok e 0 falhas, veio ok=%d fail=%d", ok, fail)
	}
}

func TestAutoProvisionRuntime(t *testing.T) {
	tempDir := t.TempDir()
	regPath := filepath.Join(tempDir, "installed.json")
	reg, _ := registry.New(regPath)

	mock := &MockExecutor{
		missing: map[string]bool{"npm": true},
	}
	r := NewWithExecutor(reg, mock, false)

	tool := catalog.FindTool("repomix")
	if tool == nil {
		t.Fatalf("Ferramenta 'repomix' não encontrada")
	}

	if err := r.Install(*tool); err != nil {
		t.Fatalf("Erro ao instalar: %v", err)
	}

	if len(mock.executed) < 2 {
		t.Fatalf("Esperava pelo menos 2 comandos (auto-provisionamento do runtime e install), executou: %v", mock.executed)
	}

	firstCmd := strings.Join(mock.executed[0], " ")
	if !strings.Contains(firstCmd, "nodejs") && !strings.Contains(firstCmd, "npm") && !strings.Contains(firstCmd, "OpenJS.NodeJS.LTS") {
		t.Errorf("Primeiro comando deveria ser provisionamento do Node/npm, foi: %s", firstCmd)
	}

	lastCmd := strings.Join(mock.executed[len(mock.executed)-1], " ")
	if !strings.Contains(lastCmd, "npm") || !strings.Contains(lastCmd, "repomix") {
		t.Errorf("Último comando deveria ser a instalação do repomix, foi: %s", lastCmd)
	}
}

func TestSoftwareDependencyAutoInstall(t *testing.T) {
	tempDir := t.TempDir()
	regPath := filepath.Join(tempDir, "installed.json")
	reg, _ := registry.New(regPath)

	mock := &MockExecutor{
		missing: map[string]bool{"java": true, "temurin": true},
	}
	r := NewWithExecutor(reg, mock, false)

	tool := catalog.FindTool("ghidra")
	if tool == nil {
		t.Fatalf("Ferramenta 'ghidra' não encontrada")
	}

	if err := r.Install(*tool); err != nil {
		t.Fatalf("Erro ao instalar ghidra com dependência: %v", err)
	}

	if !reg.IsInstalled("temurin") {
		t.Errorf("Dependência 'temurin' deveria ter sido marcada como instalada no registro")
	}
	if !reg.IsInstalled("ghidra") {
		t.Errorf("'ghidra' deveria ter sido marcado como instalado no registro")
	}
}

func TestSoftwareDependencySkippedWhenSatisfied(t *testing.T) {
	tempDir := t.TempDir()
	regPath := filepath.Join(tempDir, "installed.json")
	reg, _ := registry.New(regPath)

	// java está disponível (não está em missing)
	mock := &MockExecutor{
		missing: map[string]bool{},
	}
	r := NewWithExecutor(reg, mock, false)

	tool := catalog.FindTool("ghidra")
	if tool == nil {
		t.Fatalf("Ferramenta 'ghidra' não encontrada")
	}

	if err := r.Install(*tool); err != nil {
		t.Fatalf("Erro ao instalar ghidra: %v", err)
	}

	// Como java estava satisfeito no host, temurin não deve ter gerado comando de install
	for _, cmd := range mock.executed {
		cmdStr := strings.Join(cmd, " ")
		if strings.Contains(cmdStr, "temurin") && !strings.Contains(cmdStr, "ghidra") {
			t.Errorf("Comando inesperado instalando temurin mesmo com java satisfeito: %s", cmdStr)
		}
	}
}
