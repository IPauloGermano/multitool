package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
	tea "github.com/charmbracelet/bubbletea"
)

func classifyTUIKind(tool catalog.Tool) string {
	if tool.Category == "Skills" {
		return "skill"
	}
	if tool.Category == "Desktop" {
		return "app"
	}
	return "tool"
}

func topoShortcutKey(topo string) rune {
	switch topo {
	case "IA":
		return '1'
	case "Código":
		return '2'
	case "Sistema":
		return '3'
	case "Apps":
		return '4'
	case "Segurança":
		return '5'
	default:
		return 't'
	}
}

func TestIndividualTUITools(t *testing.T) {
	tools := catalog.SupportedCatalog()
	if len(tools) == 0 {
		t.Fatal("nenhuma ferramenta suportada no SO atual")
	}

	// Reutiliza um único modelo base para encontrar índices e testar renderização individual
	regPath := filepath.Join(t.TempDir(), "installed.json")
	reg, err := registry.New(regPath)
	if err != nil {
		t.Fatalf("erro ao criar registro: %v", err)
	}
	baseModel := NewModel(reg, true)
	visibleAll := baseModel.GetVisibleTools()
	indexByID := make(map[string]int, len(visibleAll))
	for i, tool := range visibleAll {
		indexByID[tool.ID] = i
	}

	topoKeys := map[string]map[string]bool{}
	for _, topo := range catalog.Topos() {
		m := NewModel(reg, true)
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{topoShortcutKey(topo)}})
		m = updated.(Model)
		set := make(map[string]bool)
		for _, tool := range m.GetVisibleTools() {
			set[tool.ID] = true
		}
		topoKeys[topo] = set
	}

	for _, tool := range tools {
		tool := tool
		subName := classifyTUIKind(tool) + "/" + tool.ID
		t.Run(subName, func(t *testing.T) {
			idx, ok := indexByID[tool.ID]
			if !ok {
				t.Fatalf("ferramenta suportada %q não aparece na aba Todas (TabAll)", tool.ID)
			}

			topo := catalog.TopoDaSub(tool.Category)
			if !topoKeys[topo][tool.ID] {
				t.Errorf("ferramenta %q não aparece na aba de topo %q", tool.ID, topo)
			}

			// Posiciona o cursor na ferramenta e valida modal de detalhes ('d') e seleção (' ')
			m := NewModel(reg, true)
			m.cursor = idx

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
			m = updated.(Model)
			if !m.IsModalOpen() || m.GetModalTool() == nil || m.GetModalTool().ID != tool.ID {
				t.Fatalf("modal de detalhes não abriu para %q", tool.ID)
			}
			view := m.View()
			if !strings.Contains(view, tool.Name) {
				t.Errorf("view do modal de %q não contém o nome %q", tool.ID, tool.Name)
			}

			// Seleciona pelo modal com Espaço e fecha com Esc
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
			m = updated.(Model)
			if !m.selected[tool.ID] {
				t.Errorf("espaço no modal não selecionou %q no carrinho", tool.ID)
			}
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = updated.(Model)
			if m.IsModalOpen() {
				t.Errorf("Esc não fechou modal de %q", tool.ID)
			}
		})
	}
}
