package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/IPauloGermano/multitool/internal/registry"
	tea "github.com/charmbracelet/bubbletea"
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

func TestTUIModalDetails(t *testing.T) {
	tempDir := t.TempDir()
	reg, _ := registry.New(filepath.Join(tempDir, "installed.json"))
	m := NewModel(reg, true)

	// 1. Carrinho começa vazio
	if m.SelectedCount() != 0 {
		t.Fatalf("Esperava carrinho vazio, obteve: %d", m.SelectedCount())
	}

	// 2. Enter com carrinho vazio deve abrir modal de detalhes
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.IsModalOpen() {
		t.Fatalf("Esperava modal aberto após Enter com carrinho vazio")
	}
	if m.GetModalTool() == nil {
		t.Fatalf("Esperava ferramenta carregada no modal")
	}

	// 3. View deve renderizar modal com título e detalhes
	view := m.View()
	if !strings.Contains(view, "DETALHES") && !strings.Contains(view, m.GetModalTool().Name) {
		t.Errorf("View do modal não contém cabeçalho esperado: %s", view)
	}

	// 4. Barra de espaço dentro do modal deve adicionar ao carrinho
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)
	if m.SelectedCount() != 1 {
		t.Errorf("Esperava 1 item no carrinho após pressionar espaço no modal, obteve %d", m.SelectedCount())
	}

	// 5. Esc fecha o modal
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.IsModalOpen() {
		t.Errorf("Esperava modal fechado após Esc")
	}

	// 6. Com item no carrinho, Enter deve executar (tea.Quit)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.executed {
		t.Errorf("Esperava m.executed == true após Enter com itens no carrinho")
	}
	if cmd == nil {
		t.Errorf("Esperava tea.Quit cmd após Enter com itens no carrinho")
	}
}

func TestTUISuiteModalDetails(t *testing.T) {
	tempDir := t.TempDir()
	reg, _ := registry.New(filepath.Join(tempDir, "installed.json"))
	m := NewModel(reg, true)

	// Muda para aba Pacotes
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updated.(Model)

	// Enter com carrinho vazio abre modal da suíte
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.IsModalOpen() {
		t.Fatalf("Esperava modal aberto para suíte")
	}
	if m.GetModalSuite() == nil {
		t.Fatalf("Esperava suíte definida no modal")
	}

	view := m.View()
	if !strings.Contains(view, "PACOTE:") {
		t.Errorf("View do modal de pacote não contém cabeçalho de pacote: %s", view)
	}

	// Fechar modal com 'q'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(Model)
	if m.IsModalOpen() {
		t.Errorf("Esperava modal fechado após 'q'")
	}
}

func TestTUIBatchShortcuts(t *testing.T) {
	tempDir := t.TempDir()
	reg, _ := registry.New(filepath.Join(tempDir, "installed.json"))
	m := NewModel(reg, true)

	// 1. Tecla 'a' na aba TabAll deve selecionar todas as ferramentas visíveis
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(Model)
	totalTools := len(m.GetVisibleTools())
	if m.SelectedCount() != totalTools {
		t.Errorf("Esperava %d itens selecionados após 'a', obteve %d", totalTools, m.SelectedCount())
	}

	// 2. Tecla 'A' deve limpar todo o carrinho
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	m = updated.(Model)
	if m.SelectedCount() != 0 {
		t.Errorf("Esperava carrinho limpo após 'A', obteve %d", m.SelectedCount())
	}

	// 3. Muda para aba Pacotes e testa 'a'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(Model)
	if m.SelectedCount() == 0 {
		t.Errorf("Esperava itens selecionados de todos os pacotes após 'a'")
	}

	// 4. Limpa novamente com 'A'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	m = updated.(Model)
	if m.SelectedCount() != 0 {
		t.Errorf("Esperava carrinho limpo após 'A', obteve %d", m.SelectedCount())
	}
}

func TestTUIDynamicViewport(t *testing.T) {
	tempDir := t.TempDir()
	reg, _ := registry.New(filepath.Join(tempDir, "installed.json"))
	m := NewModel(reg, true)

	// 1. Simula terminal com altura reduzida (split de 15 linhas)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 15})
	m = updated.(Model)

	if m.GetViewportHeight() <= 0 || m.GetViewportHeight() >= 15 {
		t.Errorf("Altura da viewport deve ser adaptativa (< 15), obteve: %d", m.GetViewportHeight())
	}

	// View não deve entrar em pânico nem gerar tela vazia
	view := m.View()
	if len(view) == 0 {
		t.Errorf("View gerou string vazia em tela reduzida")
	}

	// 2. Simula redimensionamento para terminal amplo
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 45})
	m = updated.(Model)
	if m.GetViewportHeight() < 30 {
		t.Errorf("Altura da viewport deveria expandir em tela de 45 linhas, obteve %d", m.GetViewportHeight())
	}

	// 3. Testa navegação para baixo ajustando cursor e viewport
	for i := 0; i < 10; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
	}
	if m.GetCursor() != 10 {
		t.Errorf("Esperava cursor em 10, obteve %d", m.GetCursor())
	}
}
