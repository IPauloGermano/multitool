package registry

import (
	"path/filepath"
	"testing"
)

func TestRegistryOperations(t *testing.T) {
	tempDir := t.TempDir()
	regPath := filepath.Join(tempDir, "installed.json")

	reg, err := New(regPath)
	if err != nil {
		t.Fatalf("Erro ao criar registro: %v", err)
	}

	if reg.IsInstalled("git") {
		t.Errorf("git não deveria estar instalado inicialmente")
	}

	if err := reg.MarkInstalled("git"); err != nil {
		t.Fatalf("Erro ao marcar git como instalado: %v", err)
	}

	if !reg.IsInstalled("git") {
		t.Errorf("git deveria constar como instalado")
	}

	if err := reg.MarkInstalled("ripgrep"); err != nil {
		t.Fatalf("Erro ao marcar ripgrep como instalado: %v", err)
	}

	list := reg.ListInstalled()
	if len(list) != 2 || list[0] != "git" || list[1] != "ripgrep" {
		t.Errorf("Lista de instalados inesperada: %v", list)
	}

	// Recarrega de disco
	reg2, err := New(regPath)
	if err != nil {
		t.Fatalf("Erro ao recarregar registro: %v", err)
	}
	if !reg2.IsInstalled("git") || !reg2.IsInstalled("ripgrep") {
		t.Errorf("Registro persistido falhou ao recarregar itens")
	}

	// Remove
	if err := reg2.MarkUninstalled("git"); err != nil {
		t.Fatalf("Erro ao desinstalar git: %v", err)
	}
	if reg2.IsInstalled("git") {
		t.Errorf("git ainda consta como instalado após remoção")
	}
	if len(reg2.ListInstalled()) != 1 {
		t.Errorf("Deveria restar apenas 1 item instalado")
	}
}
