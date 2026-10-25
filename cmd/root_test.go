package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IPauloGermano/multitool/internal/registry"
)

func TestRootCommands(t *testing.T) {
	tempDir := t.TempDir()
	regPath := filepath.Join(tempDir, "installed.json")
	reg, err := registry.New(regPath)
	if err != nil {
		t.Fatalf("Erro ao criar registro de teste: %v", err)
	}

	// 1. Teste 'version'
	buf := new(bytes.Buffer)
	rootCmd := NewRootCmd(reg)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Erro no version: %v", err)
	}
	if !strings.Contains(buf.String(), "multitool v") {
		t.Errorf("Saída inesperada do version: %s", buf.String())
	}

	// 2. Teste 'system'
	buf.Reset()
	rootCmd = NewRootCmd(reg)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"system"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Erro no system: %v", err)
	}
	if !strings.Contains(buf.String(), "DIAGNÓSTICO DO SISTEMA") {
		t.Errorf("Saída inesperada do system: %s", buf.String())
	}

	// 3. Teste 'suites'
	buf.Reset()
	rootCmd = NewRootCmd(reg)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"suites"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Erro no suites: %v", err)
	}
	if !strings.Contains(buf.String(), "@vibecode") {
		t.Errorf("Esperava @vibecode na lista de suítes: %s", buf.String())
	}

	// 4. Teste 'list'
	buf.Reset()
	rootCmd = NewRootCmd(reg)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Erro no list: %v", err)
	}
	if !strings.Contains(buf.String(), "git") {
		t.Errorf("Esperava 'git' na listagem: %s", buf.String())
	}

	// 5. Teste 'search'
	buf.Reset()
	rootCmd = NewRootCmd(reg)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"search", "ripgrep"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Erro no search: %v", err)
	}
	if !strings.Contains(buf.String(), "ripgrep") {
		t.Errorf("Esperava 'ripgrep' no resultado da busca: %s", buf.String())
	}

	// 6. Teste 'info'
	buf.Reset()
	rootCmd = NewRootCmd(reg)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"info", "ollama"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Erro no info: %v", err)
	}
	if !strings.Contains(buf.String(), "Ollama") {
		t.Errorf("Esperava 'Ollama' no info: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "Repositório:") || !strings.Contains(buf.String(), "https://github.com/ollama/ollama") {
		t.Errorf("Esperava link do repositório no info ollama: %s", buf.String())
	}

	// 7. Teste 'install --dry-run'
	buf.Reset()
	rootCmd = NewRootCmd(reg)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"install", "btop", "--dry-run"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Erro no install dry-run: %v", err)
	}
	if !reg.IsInstalled("btop") {
		t.Errorf("btop deveria estar registrado após install dry-run")
	}

	// 8. Teste 'info' com dependências
	buf.Reset()
	rootCmd = NewRootCmd(reg)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"info", "ghidra"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Erro no info ghidra: %v", err)
	}
	if !strings.Contains(buf.String(), "temurin") {
		t.Errorf("Esperava 'temurin' listado nas dependências de ghidra: %s", buf.String())
	}

	// 9. Teste 'install --dry-run' com resolução de dependências
	buf.Reset()
	rootCmd = NewRootCmd(reg)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"install", "ghidra", "--dry-run"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Erro no install ghidra dry-run: %v", err)
	}
	if !reg.IsInstalled("ghidra") {
		t.Errorf("ghidra deveria estar registrado após install dry-run")
	}
	if !reg.IsInstalled("temurin") && !strings.Contains(buf.String(), "já está disponível no sistema") {
		t.Errorf("dependência temurin deveria estar registrada após install dry-run de ghidra ou reportada como já disponível: %s", buf.String())
	}

	// 10. Teste 'info git' com alvo macOS
	buf.Reset()
	rootCmd = NewRootCmd(reg)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"info", "git"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Erro no info git: %v", err)
	}
	if !strings.Contains(buf.String(), "macOS:") {
		t.Errorf("Esperava exibição do alvo macOS no info git: %s", buf.String())
	}

	// 11. Teste 'sync --dry-run'
	buf.Reset()
	rootCmd = NewRootCmd(reg)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"sync", "--dry-run"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Erro no sync dry-run: %v", err)
	}
	if !strings.Contains(buf.String(), "SINCRONIZAÇÃO") {
		t.Errorf("Esperava cabeçalho de sincronização: %s", buf.String())
	}

	// 12. Teste 'sync --json'
	buf.Reset()
	rootCmd = NewRootCmd(reg)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"sync", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Erro no sync json: %v", err)
	}
	if !strings.Contains(buf.String(), "synced") {
		t.Errorf("Esperava formato JSON no sync --json: %s", buf.String())
	}
}
