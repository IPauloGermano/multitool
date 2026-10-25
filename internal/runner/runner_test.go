package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
	"github.com/IPauloGermano/multitool/internal/system"
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

func TestVerifyFileSHA256(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "sample.txt")
	content := []byte("antigravity multitool test payload")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Erro ao criar arquivo de teste: %v", err)
	}

	// echo -n "antigravity multitool test payload" | sha256sum
	// 5c41cae6db3f886d912ceba281895a9d8fa176df2c70030c23f2fbe317bc224d
	expectedSHA := "a70c58fa1fcda3235b5a9f9c38e9fd20c9b5b413d4ca48246f26b4af0b66c2c6"

	// 1. Sucesso com hash correto
	if err := VerifyFileSHA256(testFile, expectedSHA); err != nil {
		t.Errorf("Esperava validação bem sucedida, obteve erro: %v", err)
	}

	// 2. Falha com hash adulterado
	wrongSHA := "0000000000000000000000000000000000000000000000000000000000000000"
	if err := VerifyFileSHA256(testFile, wrongSHA); err == nil {
		t.Errorf("Esperava erro de mismatch com hash incorreto")
	}

	// 3. Falha com arquivo inexistente
	if err := VerifyFileSHA256(filepath.Join(tempDir, "nao_existe.txt"), expectedSHA); err == nil {
		t.Errorf("Esperava erro com arquivo inexistente")
	}
}

func TestSyncInstalled(t *testing.T) {
	tempDir := t.TempDir()
	regPath := filepath.Join(tempDir, "installed.json")
	reg, _ := registry.New(regPath)

	// Mock onde apenas 'git' e 'btop' existem no PATH
	mock := &MockExecutor{
		missing: map[string]bool{
			"ripgrep": true,
			"jq":      true,
			"bat":     true,
		},
	}

	// 1. Teste em modo DryRun
	rDry := NewWithExecutor(reg, mock, true)
	results, already, err := rDry.SyncInstalled()
	if err != nil {
		t.Fatalf("Erro no SyncInstalled dry-run: %v", err)
	}
	if already != 0 {
		t.Errorf("Esperava 0 ferramentas já registradas, obteve %d", already)
	}
	if len(results) == 0 {
		t.Errorf("Esperava ferramentas detectadas via LookPath")
	}
	if reg.IsInstalled("git") {
		t.Errorf("Dry-run não deveria registrar 'git' em installed.json")
	}

	// 2. Teste em modo Real
	rReal := NewWithExecutor(reg, mock, false)
	resultsReal, _, err := rReal.SyncInstalled()
	if err != nil {
		t.Fatalf("Erro no SyncInstalled real: %v", err)
	}
	if len(resultsReal) == 0 {
		t.Errorf("Esperava ferramentas detectadas")
	}
	if !reg.IsInstalled("git") {
		t.Errorf("Modo real deveria registrar 'git' no installed.json")
	}

	// 3. Segunda execução: ferramentas devem constar como already
	_, alreadySecond, _ := rReal.SyncInstalled()
	if alreadySecond == 0 {
		t.Errorf("Segunda execução deveria contabilizar ferramentas já instaladas")
	}
}

func TestParallelSkillsInstall(t *testing.T) {
	tempDir := t.TempDir()
	regPath := filepath.Join(tempDir, "installed.json")
	reg, _ := registry.New(regPath)

	mock := &MockExecutor{}
	r := NewWithExecutor(reg, mock, true) // dry-run

	skill1 := catalog.FindTool("superpowers")
	skill2 := catalog.FindTool("impeccable")
	skill3 := catalog.FindTool("archify")

	if skill1 == nil || skill2 == nil || skill3 == nil {
		t.Fatalf("Skills de teste não encontradas no catálogo")
	}

	tools := []catalog.Tool{*skill1, *skill2, *skill3}
	ok, fail := r.RunBatchInstall(tools)
	if ok != 3 || fail != 0 {
		t.Errorf("Esperava 3 sucessos em paralelo e 0 falhas, obteve ok=%d, fail=%d", ok, fail)
	}
	if !reg.IsInstalled("superpowers") || !reg.IsInstalled("impeccable") || !reg.IsInstalled("archify") {
		t.Errorf("Todas as skills deveriam estar marcadas no registro")
	}
}

func TestTypeBrewExecution(t *testing.T) {
	tempDir := t.TempDir()
	regPath := filepath.Join(tempDir, "installed.json")
	reg, _ := registry.New(regPath)

	mock := &MockExecutor{}
	r := NewWithExecutor(reg, mock, false)

	brewTool := catalog.Tool{
		ID:       "formula-test",
		Name:     "Formula Test",
		Category: "Infraestrutura",
		Linux:    &catalog.Target{Type: catalog.TypeBrew, PackageID: "formula-test", Binary: "formula-test"},
	}

	if err := r.Install(brewTool); err != nil {
		t.Fatalf("Erro ao instalar ferramenta TypeBrew: %v", err)
	}

	if len(mock.executed) == 0 {
		t.Fatalf("Nenhum comando foi executado")
	}
	lastCmd := strings.Join(mock.executed[len(mock.executed)-1], " ")
	if !strings.Contains(lastCmd, "brew install formula-test") {
		t.Errorf("Comando inesperado para brew install: %s", lastCmd)
	}
}

func TestTemurinDynamicArchDownload(t *testing.T) {
	tempDir := t.TempDir()
	regPath := filepath.Join(tempDir, "installed.json")
	reg, _ := registry.New(regPath)

	tool := catalog.FindTool("temurin")
	if tool == nil {
		t.Fatalf("Ferramenta 'temurin' não encontrada no catálogo")
	}

	tests := []struct {
		arch     string
		expected string
	}{
		{arch: "amd64", expected: "/linux/x64/"},
		{arch: "arm64", expected: "/linux/aarch64/"},
		{arch: "arm", expected: "/linux/arm/"},
		{arch: "unknown", expected: "/linux/x64/"},
	}

	for _, tt := range tests {
		mock := &MockExecutor{}
		r := NewWithExecutor(reg, mock, false)
		r.sysInfo.OS = system.OSLinux
		r.sysInfo.Arch = tt.arch

		if err := r.installScript("temurin", tool.Linux); err != nil {
			t.Fatalf("Erro ao executar installScript temurin com arch=%s: %v", tt.arch, err)
		}

		if len(mock.executed) == 0 {
			t.Fatalf("Nenhum comando executado para arch=%s", tt.arch)
		}
		cmdStr := strings.Join(mock.executed[len(mock.executed)-1], " ")
		if !strings.Contains(cmdStr, tt.expected) {
			t.Errorf("Para arch=%s esperava substring %q no comando, obteve: %s", tt.arch, tt.expected, cmdStr)
		}
	}
}
