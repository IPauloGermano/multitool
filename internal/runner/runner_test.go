package runner

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
)

type MockExecutor struct {
	executed [][]string
}

func (m *MockExecutor) Execute(name string, args ...string) error {
	cmd := append([]string{name}, args...)
	m.executed = append(m.executed, cmd)
	return nil
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
