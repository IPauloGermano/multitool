package tui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/IPauloGermano/multitool/internal/registry"
)

func TestTUIStateTransitions(t *testing.T) {
	tempDir := t.TempDir()
	reg, _ := registry.New(filepath.Join(tempDir, "installed.json"))

	m := NewModel(reg, true)
	if m.GetMode() != ModeInstall {
		t.Errorf("Modo inicial esperado: %s, veio: %s", ModeInstall, m.GetMode())
	}

	// 1. Alterna modo com 'r'
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(Model)
	if m.GetMode() != ModeUninstall {
		t.Errorf("Esperava modo %s, veio: %s", ModeUninstall, m.GetMode())
	}

	// 2. Marca ferramenta no carrinho com espaço
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)
	if m.SelectedCount() != 1 {
		t.Errorf("Esperava 1 item no carrinho, veio: %d", m.SelectedCount())
	}

	// 3. Muda para aba Carrinho com 'c'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)
	if m.GetTab() != TabCart {
		t.Errorf("Esperava TabCart, veio: %v", m.GetTab())
	}
	if len(m.GetVisibleTools()) != 1 {
		t.Errorf("Aba carrinho deveria ter 1 ferramenta visível, veio %d", len(m.GetVisibleTools()))
	}

	// 4. Muda para aba Pacotes com 'p'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updated.(Model)
	if m.GetTab() != TabPackages {
		t.Errorf("Esperava TabPackages, veio: %v", m.GetTab())
	}
	if len(m.GetVisibleSuites()) == 0 {
		t.Errorf("Aba pacotes não deveria estar vazia")
	}

	// 5. Marca o primeiro pacote (@vibecode) com espaço
	firstSuite := m.GetVisibleSuites()[0]
	prevSelected := m.SelectedCount()
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)

	// Carrinho deve ter aumentado com todos os membros do pacote
	if m.SelectedCount() <= prevSelected {
		t.Errorf("Esperava aumento no carrinho após selecionar suíte %s, count=%d", firstSuite.ID, m.SelectedCount())
	}

	// 6. View deve renderizar sem pânico
	viewOutput := m.View()
	if len(viewOutput) == 0 {
		t.Errorf("View() da aba pacotes retornou string vazia")
	}

	// 7. Desmarca o pacote com espaço
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)
	for _, tid := range firstSuite.Tools {
		if m.selected[tid] {
			t.Errorf("Ferramenta %s do pacote %s deveria ter sido desmarcada", tid, firstSuite.ID)
		}
	}
}

func TestTUISearch(t *testing.T) {
	tempDir := t.TempDir()
	reg, _ := registry.New(filepath.Join(tempDir, "installed.json"))
	m := NewModel(reg, true)

	// 1. Ativa busca com '/'
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(Model)
	if !m.searching {
		t.Errorf("Esperava modo de busca ativo")
	}

	// 2. Digita 'docker'
	for _, r := range "docker" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}

	if len(m.GetVisibleTools()) == 0 {
		t.Errorf("Busca por 'docker' não deveria ser vazia")
	}

	// 3. Limpa busca com Esc
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // sai do modo input
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // limpa query
	m = updated.(Model)
	if m.searchQuery != "" {
		t.Errorf("Esperava busca limpa após Esc")
	}

	// 4. Busca por pacote @vibecode em TabAll
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(Model)
	for _, r := range "@vibecode" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	if len(m.GetVisibleTools()) != 15 {
		t.Errorf("Busca por @vibecode deveria retornar 15 ferramentas, obteve %d", len(m.GetVisibleTools()))
	}
}
