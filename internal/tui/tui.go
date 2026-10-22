package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
	"github.com/IPauloGermano/multitool/internal/runner"
	"github.com/IPauloGermano/multitool/internal/system"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#2563EB")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA")).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF"))

	cartStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Bold(true)

	modeInstallStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981")).
				Bold(true)

	modeUninstallStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#EF4444")).
				Bold(true)

	badgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#38BDF8")).
			Background(lipgloss.Color("#0F172A")).
			Padding(0, 1)
)

type Mode string

const (
	ModeInstall   Mode = "instalar"
	ModeUninstall Mode = "desinstalar"
)

type Tab int

const (
	TabAll Tab = iota
	TabTopo
	TabInstalled
	TabCart
)

type Model struct {
	reg          *registry.Registry
	sysInfo      *system.SystemInfo
	dryRun       bool
	allTools     []catalog.Tool
	visibleTools []catalog.Tool
	cursor       int
	selected     map[string]bool
	mode         Mode
	tab          Tab
	activeTopo   string
	searching    bool
	searchQuery  string
	quitting     bool
	executed     bool
}

func NewModel(reg *registry.Registry, dryRun bool) Model {
	tools := catalog.SupportedCatalog()
	m := Model{
		reg:          reg,
		sysInfo:      system.GetSystemInfo(),
		dryRun:       dryRun,
		allTools:     tools,
		visibleTools: tools,
		cursor:       0,
		selected:     make(map[string]bool),
		mode:         ModeInstall,
		tab:          TabAll,
	}
	m.refreshVisible()
	return m
}

func (m *Model) refreshVisible() {
	var list []catalog.Tool

	switch m.tab {
	case TabAll:
		list = m.allTools
	case TabTopo:
		list = catalog.ToolsByTopo(m.activeTopo)
	case TabInstalled:
		for _, t := range m.allTools {
			if m.reg.IsInstalled(t.ID) {
				list = append(list, t)
			}
		}
	case TabCart:
		for _, t := range m.allTools {
			if m.selected[t.ID] {
				list = append(list, t)
			}
		}
	}

	if m.searchQuery != "" {
		var filtered []catalog.Tool
		q := strings.ToLower(m.searchQuery)
		for _, t := range list {
			if strings.Contains(strings.ToLower(t.ID), q) ||
				strings.Contains(strings.ToLower(t.Name), q) ||
				strings.Contains(strings.ToLower(t.Description), q) ||
				strings.Contains(strings.ToLower(t.Category), q) {
				filtered = append(filtered, t)
			}
		}
		list = filtered
	}

	m.visibleTools = list
	if m.cursor >= len(m.visibleTools) {
		m.cursor = len(m.visibleTools) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) SelectedCount() int {
	return len(m.selected)
}

func (m Model) GetMode() Mode {
	return m.mode
}

func (m Model) GetCursor() int {
	return m.cursor
}

func (m Model) GetTab() Tab {
	return m.tab
}

func (m Model) GetVisibleTools() []catalog.Tool {
	return m.visibleTools
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.searching {
			switch msg.String() {
			case "enter", "esc":
				m.searching = false
				return m, nil
			case "backspace":
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
					m.refreshVisible()
				}
				return m, nil
			default:
				if len(msg.Runes) > 0 {
					m.searchQuery += string(msg.Runes)
					m.refreshVisible()
				}
				return m, nil
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.refreshVisible()
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.visibleTools)-1 {
				m.cursor++
			}

		case "/":
			m.searching = true
			return m, nil

		case "r":
			if m.mode == ModeInstall {
				m.mode = ModeUninstall
			} else {
				m.mode = ModeInstall
			}

		case "t":
			m.tab = TabAll
			m.refreshVisible()

		case "i":
			m.tab = TabInstalled
			m.refreshVisible()

		case "c":
			m.tab = TabCart
			m.refreshVisible()

		case "1":
			m.tab = TabTopo
			m.activeTopo = "IA"
			m.refreshVisible()

		case "2":
			m.tab = TabTopo
			m.activeTopo = "Código"
			m.refreshVisible()

		case "3":
			m.tab = TabTopo
			m.activeTopo = "Sistema"
			m.refreshVisible()

		case "4":
			m.tab = TabTopo
			m.activeTopo = "Apps"
			m.refreshVisible()

		case "5":
			m.tab = TabTopo
			m.activeTopo = "Segurança"
			m.refreshVisible()

		case " ":
			if len(m.visibleTools) > 0 && m.cursor >= 0 && m.cursor < len(m.visibleTools) {
				toolID := m.visibleTools[m.cursor].ID
				m.selected[toolID] = !m.selected[toolID]
				if !m.selected[toolID] {
					delete(m.selected, toolID)
				}
				if m.tab == TabCart {
					m.refreshVisible()
				}
			}

		case "enter":
			if len(m.selected) > 0 {
				m.executed = true
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return "Operação cancelada pelo usuário.\n"
	}

	var b strings.Builder

	// Header com info do sistema detectado
	osLabel := string(m.sysInfo.OS)
	if m.sysInfo.DistroPretty != "" {
		osLabel = fmt.Sprintf("%s (%s / %s)", m.sysInfo.OS, m.sysInfo.Distro, m.sysInfo.PrimaryManager)
	} else if m.sysInfo.OS == system.OSWindows {
		osLabel = fmt.Sprintf("Windows (%s / %s)", m.sysInfo.Arch, m.sysInfo.PrimaryManager)
	}

	b.WriteString(titleStyle.Render(" MULTITOOL "))
	b.WriteString(fmt.Sprintf(" %s", badgeStyle.Render(osLabel)))
	if m.dryRun {
		b.WriteString(fmt.Sprintf(" %s", lipgloss.NewStyle().Background(lipgloss.Color("#D97706")).Padding(0, 1).Bold(true).Render("DRY-RUN")))
	}
	b.WriteString("\n\n")

	// Modo e Status
	modeBadge := modeInstallStyle.Render("[ MODO: INSTALAR ]")
	if m.mode == ModeUninstall {
		modeBadge = modeUninstallStyle.Render("[ MODO: DESINSTALAR ]")
	}
	cartBadge := cartStyle.Render(fmt.Sprintf("Carrinho: %d item(ns)", len(m.selected)))
	b.WriteString(fmt.Sprintf("%s    %s\n\n", modeBadge, cartBadge))

	// Abas
	b.WriteString(headerStyle.Render("Abas: "))
	b.WriteString("[1] IA  [2] Código  [3] Sistema  [4] Apps  [5] Segurança  |  [t] Todas  [i] Instalados  [c] Carrinho\n")

	if m.searching {
		b.WriteString(fmt.Sprintf("\n%s %s█\n", selectedStyle.Render("Buscar:"), m.searchQuery))
	} else if m.searchQuery != "" {
		b.WriteString(fmt.Sprintf("\n%s %s (pressione '/' para editar ou 'Esc' para limpar)\n", dimStyle.Render("Filtro:"), m.searchQuery))
	} else {
		b.WriteString("\n")
	}

	// Lista de Ferramentas
	if len(m.visibleTools) == 0 {
		b.WriteString(dimStyle.Render("  Nenhuma ferramenta para exibir nesta categoria ou filtro.\n\n"))
	} else {
		maxItems := 14
		start := 0
		if m.cursor >= maxItems {
			start = m.cursor - maxItems + 1
		}
		end := start + maxItems
		if end > len(m.visibleTools) {
			end = len(m.visibleTools)
		}

		for i := start; i < end; i++ {
			tool := m.visibleTools[i]
			cursor := "  "
			if m.cursor == i {
				cursor = selectedStyle.Render("▶ ")
			}

			checked := "[ ]"
			if m.selected[tool.ID] {
				checked = selectedStyle.Render("[✓]")
			}

			installedTag := ""
			if m.reg.IsInstalled(tool.ID) {
				installedTag = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render(" [instalado]")
			}

			method := ""
			target := tool.TargetForCurrentOS()
			if target != nil {
				method = fmt.Sprintf("(%s)", target.Type)
			}

			line := fmt.Sprintf("%s%s %-14s %-22s %-10s %s%s\n",
				cursor, checked, tool.ID, tool.Name, method, dimStyle.Render(tool.Description), installedTag)

			b.WriteString(line)
		}

		if len(m.visibleTools) > maxItems {
			b.WriteString(dimStyle.Render(fmt.Sprintf("\n  ... exibindo %d de %d ferramentas (use ↑/↓ para rolar)\n", end-start, len(m.visibleTools))))
		}
	}

	// Rodapé de atalhos
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Atalhos: [↑/↓/j/k] Navegar • [Espaço] Marcar/Desmarcar • [Enter] Executar • [r] Alternar Modo • [/] Buscar • [q] Sair\n"))

	return b.String()
}

func Run(reg *registry.Registry, dryRun bool) error {
	m := NewModel(reg, dryRun)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	res := finalModel.(Model)
	if res.executed && len(res.selected) > 0 {
		var toRun []catalog.Tool
		for _, t := range res.allTools {
			if res.selected[t.ID] {
				toRun = append(toRun, t)
			}
		}

		r := runner.New(reg, dryRun)
		if res.mode == ModeInstall {
			r.RunBatchInstall(toRun)
		} else {
			r.RunBatchUninstall(toRun)
		}
	}
	return nil
}
