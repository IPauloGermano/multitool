package tui

import (
	"fmt"
	"strings"

	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
	"github.com/IPauloGermano/multitool/internal/runner"
	"github.com/IPauloGermano/multitool/internal/system"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	TabPackages
	TabInstalled
	TabCart
)

type Model struct {
	reg           *registry.Registry
	sysInfo       *system.SystemInfo
	dryRun        bool
	allTools      []catalog.Tool
	visibleTools  []catalog.Tool
	allSuites     []catalog.Suite
	visibleSuites []catalog.Suite
	cursor        int
	selected      map[string]bool
	mode          Mode
	tab           Tab
	activeTopo    string
	searching     bool
	searchQuery   string
	quitting      bool
	executed      bool

	// Viewport e layout responsivo
	viewport      viewport.Model
	width         int
	height        int
	ready         bool

	// Modal de detalhes
	showModal     bool
	modalTool     *catalog.Tool
	modalSuite    *catalog.Suite
	modalViewport viewport.Model
}

func NewModel(reg *registry.Registry, dryRun bool) Model {
	tools := catalog.SupportedCatalog()
	suites := catalog.SupportedSuites()
	m := Model{
		reg:           reg,
		sysInfo:       system.GetSystemInfo(),
		dryRun:        dryRun,
		allTools:      tools,
		visibleTools:  tools,
		allSuites:     suites,
		visibleSuites: suites,
		cursor:        0,
		selected:      make(map[string]bool),
		mode:          ModeInstall,
		tab:           TabAll,
		width:         100,
		height:        24,
	}
	m.updateViewportDimensions()
	m.refreshVisible()
	return m
}

func (m *Model) updateViewportDimensions() {
	if m.width <= 0 {
		m.width = 100
	}
	if m.height <= 0 {
		m.height = 24
	}

	headerLines := 6
	if m.height < 22 {
		headerLines = 4 // layout compacto para telas reduzidas (splits < 20 linhas)
	}
	if m.searching || m.searchQuery != "" {
		headerLines++
	}

	footerLines := 2
	if m.height < 20 {
		footerLines = 1
	}

	bodyHeight := m.height - headerLines - footerLines
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	vpWidth := m.width
	if vpWidth <= 0 {
		vpWidth = 100
	}
	if vpWidth > 130 {
		vpWidth = 130
	}
	if vpWidth < 40 {
		vpWidth = 40
	}

	modalW := m.width - 6
	if modalW > 86 {
		modalW = 86
	}
	if modalW < 36 {
		modalW = 36
	}
	modalH := m.height - 4
	if modalH < 6 {
		modalH = 6
	}

	if !m.ready {
		m.viewport = viewport.New(vpWidth, bodyHeight)
		m.modalViewport = viewport.New(modalW-4, modalH-2)
		m.ready = true
	} else {
		m.viewport.Width = vpWidth
		m.viewport.Height = bodyHeight
		m.modalViewport.Width = modalW - 4
		m.modalViewport.Height = modalH - 2
	}

	m.syncViewport()
	if m.showModal {
		m.syncModalViewport()
	}
}

// truncateText limita visualmente uma string a maxWidth colunas no terminal, adicionando reticências se necessário.
func truncateText(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		r := []rune(s)
		if len(r) > maxWidth {
			return string(r[:maxWidth])
		}
		return s
	}
	target := maxWidth - 3
	r := []rune(s)
	var b strings.Builder
	curW := 0
	for _, ch := range r {
		w := lipgloss.Width(string(ch))
		if curW+w > target {
			break
		}
		b.WriteRune(ch)
		curW += w
	}
	b.WriteString("...")
	return b.String()
}

// wrapText quebra linhas de texto respeitando palavras para exibição em modais ou caixas formatadas.
func wrapText(s string, maxWidth int, indent string) string {
	if maxWidth <= 0 {
		return s
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	var cur strings.Builder
	curW := 0
	for _, w := range words {
		ww := lipgloss.Width(w)
		if curW > 0 && curW+1+ww > maxWidth {
			lines = append(lines, cur.String())
			cur.Reset()
			curW = 0
		}
		if curW > 0 {
			cur.WriteString(" ")
			curW++
		}
		cur.WriteString(w)
		curW += ww
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return strings.Join(lines, "\n"+indent)
}

func (m *Model) refreshVisible() {
	if m.tab == TabPackages {
		var list []catalog.Suite
		if m.searchQuery != "" {
			q := strings.ToLower(m.searchQuery)
			cleanQ := strings.TrimPrefix(q, "@")
			for _, s := range m.allSuites {
				matchesTools := false
				for _, tid := range s.Tools {
					if strings.Contains(strings.ToLower(tid), cleanQ) {
						matchesTools = true
						break
					}
					t := catalog.FindTool(tid)
					if t != nil && strings.Contains(strings.ToLower(t.Name), cleanQ) {
						matchesTools = true
						break
					}
				}
				if strings.Contains(strings.ToLower(s.ID), cleanQ) ||
					strings.Contains(strings.ToLower(s.Name), cleanQ) ||
					strings.Contains(strings.ToLower(s.Description), cleanQ) ||
					matchesTools {
					list = append(list, s)
				}
			}
		} else {
			list = m.allSuites
		}
		m.visibleSuites = list
		if m.cursor >= len(m.visibleSuites) {
			m.cursor = len(m.visibleSuites) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.syncViewport()
		return
	}

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
		cleanQ := strings.TrimPrefix(q, "@")
		isSuiteSearch := strings.HasPrefix(q, "@")

		suite := catalog.GetSuite(cleanQ)
		if isSuiteSearch && suite != nil {
			for _, t := range list {
				for _, tid := range suite.Tools {
					if tid == t.ID {
						filtered = append(filtered, t)
						break
					}
				}
			}
		} else {
			for _, t := range list {
				if strings.Contains(strings.ToLower(t.ID), q) ||
					strings.Contains(strings.ToLower(t.Name), q) ||
					strings.Contains(strings.ToLower(t.Description), q) ||
					strings.Contains(strings.ToLower(t.Category), q) {
					filtered = append(filtered, t)
				}
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
	m.syncViewport()
}

func (m *Model) syncViewport() {
	if m.tab == TabPackages {
		availWidth := m.viewport.Width
		if availWidth > m.width && m.width > 0 {
			availWidth = m.width
		}
		if availWidth > 1 {
			availWidth--
		}

		suiteIDWidth := 13
		suiteNameWidth := 28
		statsWidth := 18
		if availWidth < 95 {
			suiteNameWidth = 22
			statsWidth = 16
		}
		if availWidth < 80 {
			suiteNameWidth = 18
			statsWidth = 14
		}

		prefixW := 2 + 3 + 1 + suiteIDWidth + 1 + suiteNameWidth + 1 + statsWidth + 1

		var lines []string
		suiteOffsets := make([]int, len(m.visibleSuites))
		currLine := 0

		for i, suite := range m.visibleSuites {
			suiteOffsets[i] = currLine
			cursor := "  "
			if m.cursor == i {
				cursor = selectedStyle.Render("▶ ")
			}

			totalTools := len(suite.Tools)
			selectedTools := 0
			installedTools := 0
			for _, tid := range suite.Tools {
				if m.selected[tid] {
					selectedTools++
				}
				if m.reg.IsInstalled(tid) {
					installedTools++
				}
			}

			checked := "[ ]"
			if totalTools > 0 && selectedTools == totalTools {
				checked = selectedStyle.Render("[✓]")
			} else if selectedTools > 0 {
				checked = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true).Render("[-]")
			}

			suiteIDFormatted := fmt.Sprintf("@%-12s", truncateText(suite.ID, 12))
			if m.cursor == i {
				suiteIDFormatted = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#60A5FA")).Render(suiteIDFormatted)
			}

			statsBadge := fmt.Sprintf("(%d/%d instalados)", installedTools, totalTools)
			if installedTools == totalTools && totalTools > 0 {
				statsBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render(statsBadge)
			} else {
				statsBadge = dimStyle.Render(statsBadge)
			}

			availDesc := availWidth - prefixW
			if availDesc < 0 {
				availDesc = 0
			}

			sName := truncateText(suite.Name, suiteNameWidth)
			sDesc := truncateText(suite.Description, availDesc)

			var line string
			if sDesc != "" {
				line = fmt.Sprintf("%s%s %s %-*s %-*s %s",
					cursor, checked, suiteIDFormatted, suiteNameWidth, sName, statsWidth, statsBadge, dimStyle.Render(sDesc))
			} else {
				line = fmt.Sprintf("%s%s %s %-*s %-*s",
					cursor, checked, suiteIDFormatted, suiteNameWidth, sName, statsWidth, statsBadge)
			}
			lines = append(lines, line)
			currLine++

			if m.cursor == i {
				var memberTokens []string
				for _, tid := range suite.Tools {
					token := tid
					if m.selected[tid] {
						token = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true).Render("●" + tid)
					} else if m.reg.IsInstalled(tid) {
						token = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("✓" + tid)
					} else {
						token = dimStyle.Render(tid)
					}
					memberTokens = append(memberTokens, token)
				}

				prefixHeader := "       " + lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")).Render("↳ Membros:") + " "
				headerW := 18
				indent := strings.Repeat(" ", headerW)

				var cur strings.Builder
				cur.WriteString(prefixHeader)
				curW := headerW
				first := true

				for _, token := range memberTokens {
					tokW := lipgloss.Width(token)
					if !first && curW+2+tokW > availWidth {
						lines = append(lines, cur.String())
						currLine++
						cur.Reset()
						cur.WriteString(indent)
						curW = headerW
						first = true
					}
					if !first {
						cur.WriteString("  ")
						curW += 2
					}
					cur.WriteString(token)
					curW += tokW
					first = false
				}
				if cur.Len() > 0 {
					lines = append(lines, cur.String())
					currLine++
				}
				lines = append(lines, "")
				currLine++
			}
		}

		if len(lines) == 0 {
			lines = append(lines, dimStyle.Render("  Nenhum pacote encontrado para o filtro."))
		}

		m.viewport.SetContent(strings.Join(lines, "\n"))

		if len(m.visibleSuites) > 0 && m.cursor >= 0 && m.cursor < len(suiteOffsets) {
			targetLine := suiteOffsets[m.cursor]
			if targetLine < m.viewport.YOffset {
				m.viewport.SetYOffset(targetLine)
			} else if targetLine+2 >= m.viewport.YOffset+m.viewport.Height {
				m.viewport.SetYOffset(targetLine + 2 - m.viewport.Height + 1)
			}
			if m.viewport.YOffset < 0 {
				m.viewport.SetYOffset(0)
			}
		}
	} else {
		availWidth := m.viewport.Width
		if availWidth > m.width && m.width > 0 {
			availWidth = m.width
		}
		if availWidth > 1 {
			availWidth--
		}

		idWidth := 16
		nameWidth := 22
		methodWidth := 10

		if availWidth < 95 {
			idWidth = 14
			nameWidth = 18
			methodWidth = 9
		}
		if availWidth < 80 {
			idWidth = 13
			nameWidth = 15
			methodWidth = 8
		}

		prefixW := 2 + 3 + 1 + idWidth + 1 + nameWidth + 1 + methodWidth + 1

		var lines []string
		for i, tool := range m.visibleTools {
			cursor := "  "
			if m.cursor == i {
				cursor = selectedStyle.Render("▶ ")
			}

			checked := "[ ]"
			if m.selected[tool.ID] {
				checked = selectedStyle.Render("[✓]")
			}

			tagW := 0
			installedTag := ""
			if m.reg.IsInstalled(tool.ID) {
				installedTag = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render(" [instalado]")
				tagW = 12
			}

			method := ""
			target := tool.TargetForCurrentOS()
			if target != nil {
				method = fmt.Sprintf("(%s)", target.Type)
			}

			availDesc := availWidth - prefixW - tagW
			if availDesc < 0 {
				availDesc = 0
			}

			name := truncateText(tool.Name, nameWidth)
			idStr := truncateText(tool.ID, idWidth)
			methodStr := truncateText(method, methodWidth)
			desc := truncateText(tool.Description, availDesc)

			var line string
			if desc != "" {
				line = fmt.Sprintf("%s%s %-*s %-*s %-*s %s%s",
					cursor, checked, idWidth, idStr, nameWidth, name, methodWidth, methodStr, dimStyle.Render(desc), installedTag)
			} else if installedTag != "" {
				line = fmt.Sprintf("%s%s %-*s %-*s %-*s%s",
					cursor, checked, idWidth, idStr, nameWidth, name, methodWidth, methodStr, installedTag)
			} else {
				line = fmt.Sprintf("%s%s %-*s %-*s %-*s",
					cursor, checked, idWidth, idStr, nameWidth, name, methodWidth, methodStr)
			}
			lines = append(lines, line)
		}

		if len(lines) == 0 {
			lines = append(lines, dimStyle.Render("  Nenhuma ferramenta para exibir nesta categoria ou filtro."))
		}

		m.viewport.SetContent(strings.Join(lines, "\n"))

		if len(m.visibleTools) > 0 {
			if m.cursor < m.viewport.YOffset {
				m.viewport.SetYOffset(m.cursor)
			} else if m.cursor >= m.viewport.YOffset+m.viewport.Height {
				m.viewport.SetYOffset(m.cursor - m.viewport.Height + 1)
			}
			if m.viewport.YOffset < 0 {
				m.viewport.SetYOffset(0)
			}
		}
	}
}

func (m *Model) syncModalViewport() {
	var content string
	if m.modalTool != nil {
		content = m.renderToolModalContent(*m.modalTool)
	} else if m.modalSuite != nil {
		content = m.renderSuiteModalContent(*m.modalSuite)
	}
	m.modalViewport.SetContent(content)
}

func (m *Model) renderToolModalContent(tool catalog.Tool) string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#2563EB")).
		Padding(0, 1).
		Render(fmt.Sprintf(" %s (%s) ", tool.Name, tool.ID))

	b.WriteString(title + "\n\n")

	wrapW := m.modalViewport.Width - 16
	if wrapW < 20 {
		wrapW = 20
	}
	b.WriteString(fmt.Sprintf("• Descrição:    %s\n", wrapText(tool.Description, wrapW, "                ")))
	b.WriteString(fmt.Sprintf("• Categoria:    %s (%s)\n", tool.Category, catalog.TopoDaSub(tool.Category)))

	installed := dimStyle.Render("Não")
	if m.reg.IsInstalled(tool.ID) {
		installed = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true).Render("Sim (Instalado)")
	}
	b.WriteString(fmt.Sprintf("• Instalado:    %s\n", installed))

	inCart := dimStyle.Render("Não")
	if m.selected[tool.ID] {
		inCart = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true).Render("Sim (Marcado no carrinho)")
	}
	b.WriteString(fmt.Sprintf("• No Carrinho:  %s\n", inCart))

	upstream := catalog.GetUpstreamRepo(tool.ID)
	if upstream == "" {
		upstream = dimStyle.Render("—")
	} else {
		upstream = lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Render(upstream)
	}
	b.WriteString(fmt.Sprintf("• Repositório:  %s\n", upstream))

	deps := dimStyle.Render("nenhuma")
	if len(tool.Dependencies) > 0 {
		deps = lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA")).Render(strings.Join(tool.Dependencies, ", "))
	}
	b.WriteString(fmt.Sprintf("• Dependências: %s\n", deps))

	target := tool.TargetForCurrentOS()
	if target != nil {
		bin := target.Binary
		if bin == "" {
			bin = tool.ID
		}
		b.WriteString(fmt.Sprintf("• Executável:   %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true).Render(bin)))
		b.WriteString(fmt.Sprintf("• Alvo Atual:   %s (%s)\n", lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA")).Render(target.PackageID), target.Type))
		if target.SHA256 != "" {
			b.WriteString(fmt.Sprintf("• SHA-256:      %s\n", dimStyle.Render(target.SHA256)))
		}
	} else {
		b.WriteString(fmt.Sprintf("• Alvo Atual:   %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("Incompatível com o SO atual")))
	}

	b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")).Bold(true).Render("Compatibilidade Multiplataforma:") + "\n")
	if tool.Windows != nil {
		b.WriteString(fmt.Sprintf("  • Windows: %-30s [%s]\n", tool.Windows.PackageID, tool.Windows.Type))
	} else {
		b.WriteString(fmt.Sprintf("  • Windows: %s\n", dimStyle.Render("não suportado")))
	}
	if tool.Linux != nil {
		b.WriteString(fmt.Sprintf("  • Linux:   %-30s [%s]\n", tool.Linux.PackageID, tool.Linux.Type))
	} else {
		b.WriteString(fmt.Sprintf("  • Linux:   %s\n", dimStyle.Render("não suportado")))
	}
	if tool.Darwin != nil {
		b.WriteString(fmt.Sprintf("  • macOS:   %-30s [%s]\n", tool.Darwin.PackageID, tool.Darwin.Type))
	} else {
		b.WriteString(fmt.Sprintf("  • macOS:   %s\n", dimStyle.Render("não suportado")))
	}

	b.WriteString("\n" + dimStyle.Render("Ações: [Espaço] Alternar Carrinho • [Esc/Enter/q] Fechar Detalhes"))

	return b.String()
}

func (m *Model) renderSuiteModalContent(suite catalog.Suite) string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#2563EB")).
		Padding(0, 1).
		Render(fmt.Sprintf(" PACOTE: @%s - %s ", suite.ID, suite.Name))

	b.WriteString(title + "\n\n")
	wrapW := m.modalViewport.Width - 16
	if wrapW < 20 {
		wrapW = 20
	}
	b.WriteString(fmt.Sprintf("• Descrição:    %s\n", wrapText(suite.Description, wrapW, "                ")))
	b.WriteString(fmt.Sprintf("• Categoria:    %s\n", suite.Category))

	installedCount := 0
	selectedCount := 0
	for _, tid := range suite.Tools {
		if m.reg.IsInstalled(tid) {
			installedCount++
		}
		if m.selected[tid] {
			selectedCount++
		}
	}
	b.WriteString(fmt.Sprintf("• Progresso:    %d/%d instalados no sistema\n", installedCount, len(suite.Tools)))
	b.WriteString(fmt.Sprintf("• No Carrinho:  %d/%d marcados para ação\n\n", selectedCount, len(suite.Tools)))

	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")).Bold(true).Render("Ferramentas Integrantes:") + "\n")
	for _, tid := range suite.Tools {
		status := dimStyle.Render("○")
		if m.selected[tid] {
			status = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true).Render("●")
		} else if m.reg.IsInstalled(tid) {
			status = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("✓")
		}

		t := catalog.FindTool(tid)
		desc := ""
		if t != nil {
			availDesc := m.modalViewport.Width - 22
			if availDesc > 0 {
				desc = dimStyle.Render(" - " + truncateText(t.Description, availDesc))
			}
		}
		b.WriteString(fmt.Sprintf("  %s %-16s%s\n", status, tid, desc))
	}

	b.WriteString("\n" + dimStyle.Render("Ações: [Espaço] Marcar/Desmarcar Pacote • [Esc/Enter/q] Fechar Detalhes"))

	return b.String()
}

func (m Model) renderModalView() string {
	modalBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#38BDF8")).
		Padding(0, 1)

	box := modalBoxStyle.Render(m.modalViewport.View())

	var b strings.Builder
	isCompact := m.height > 0 && m.height < 22

	b.WriteString(titleStyle.Render(" MULTITOOL "))
	b.WriteString(fmt.Sprintf(" %s", badgeStyle.Render("DETALHES")))
	cartBadge := cartStyle.Render(fmt.Sprintf("Carrinho: %d item(ns)", len(m.selected)))
	b.WriteString(fmt.Sprintf("    %s", cartBadge))
	if isCompact {
		b.WriteString("\n")
	} else {
		b.WriteString("\n\n")
	}

	b.WriteString(box)
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Dica: [Esc/Enter/q] Fechar • [Espaço] Alternar Carrinho • [↑/↓/j/k] Rolar\n"))
	return b.String()
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

func (m Model) GetVisibleSuites() []catalog.Suite {
	return m.visibleSuites
}

func (m Model) IsModalOpen() bool {
	return m.showModal
}

func (m Model) GetModalTool() *catalog.Tool {
	return m.modalTool
}

func (m Model) GetModalSuite() *catalog.Suite {
	return m.modalSuite
}

func (m Model) GetViewportHeight() int {
	return m.viewport.Height
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewportDimensions()
		return m, nil

	case tea.KeyMsg:
		if m.showModal {
			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc", "enter", "q":
				m.showModal = false
				m.syncViewport()
				return m, nil
			case "up", "k":
				m.modalViewport.LineUp(1)
				return m, nil
			case "down", "j":
				m.modalViewport.LineDown(1)
				return m, nil
			case "pgup":
				m.modalViewport.HalfViewUp()
				return m, nil
			case "pgdown":
				m.modalViewport.HalfViewDown()
				return m, nil
			case " ":
				if m.modalTool != nil {
					toolID := m.modalTool.ID
					m.selected[toolID] = !m.selected[toolID]
					if !m.selected[toolID] {
						delete(m.selected, toolID)
					}
					if m.tab == TabCart {
						m.refreshVisible()
					}
				} else if m.modalSuite != nil {
					suite := m.modalSuite
					allSelected := len(suite.Tools) > 0
					for _, tid := range suite.Tools {
						if !m.selected[tid] {
							allSelected = false
							break
						}
					}
					if allSelected {
						for _, tid := range suite.Tools {
							delete(m.selected, tid)
						}
					} else {
						for _, tid := range suite.Tools {
							m.selected[tid] = true
						}
					}
				}
				m.syncModalViewport()
				return m, nil
			}
			return m, nil
		}

		if m.searching {
			switch msg.String() {
			case "enter", "esc":
				m.searching = false
				m.syncViewport()
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
				m.syncViewport()
			}

		case "down", "j":
			maxItems := len(m.visibleTools)
			if m.tab == TabPackages {
				maxItems = len(m.visibleSuites)
			}
			if m.cursor < maxItems-1 {
				m.cursor++
				m.syncViewport()
			}

		case "pgup":
			m.viewport.HalfViewUp()
			m.cursor = m.viewport.YOffset
			m.syncViewport()

		case "pgdown":
			m.viewport.HalfViewDown()
			maxItems := len(m.visibleTools)
			if m.tab == TabPackages {
				maxItems = len(m.visibleSuites)
			}
			m.cursor = m.viewport.YOffset + m.viewport.Height - 1
			if m.cursor >= maxItems {
				m.cursor = maxItems - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.syncViewport()

		case "a":
			if m.tab == TabPackages {
				for _, s := range m.visibleSuites {
					for _, tid := range s.Tools {
						m.selected[tid] = true
					}
				}
			} else {
				for _, t := range m.visibleTools {
					m.selected[t.ID] = true
				}
			}
			m.syncViewport()

		case "A":
			m.selected = make(map[string]bool)
			if m.tab == TabCart {
				m.refreshVisible()
			} else {
				m.syncViewport()
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
			m.syncViewport()

		case "t":
			m.tab = TabAll
			m.cursor = 0
			m.refreshVisible()

		case "p":
			m.tab = TabPackages
			m.cursor = 0
			m.refreshVisible()

		case "i":
			m.tab = TabInstalled
			m.cursor = 0
			m.refreshVisible()

		case "c":
			m.tab = TabCart
			m.cursor = 0
			m.refreshVisible()

		case "1":
			m.tab = TabTopo
			m.activeTopo = "IA"
			m.cursor = 0
			m.refreshVisible()

		case "2":
			m.tab = TabTopo
			m.activeTopo = "Código"
			m.cursor = 0
			m.refreshVisible()

		case "3":
			m.tab = TabTopo
			m.activeTopo = "Sistema"
			m.cursor = 0
			m.refreshVisible()

		case "4":
			m.tab = TabTopo
			m.activeTopo = "Apps"
			m.cursor = 0
			m.refreshVisible()

		case "5":
			m.tab = TabTopo
			m.activeTopo = "Segurança"
			m.cursor = 0
			m.refreshVisible()

		case " ":
			if m.tab == TabPackages {
				if len(m.visibleSuites) > 0 && m.cursor >= 0 && m.cursor < len(m.visibleSuites) {
					suite := m.visibleSuites[m.cursor]
					allSelected := len(suite.Tools) > 0
					for _, tid := range suite.Tools {
						if !m.selected[tid] {
							allSelected = false
							break
						}
					}
					if allSelected {
						for _, tid := range suite.Tools {
							delete(m.selected, tid)
						}
					} else {
						for _, tid := range suite.Tools {
							m.selected[tid] = true
						}
					}
				}
			} else {
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
			}
			m.syncViewport()

		case "enter":
			if len(m.selected) > 0 {
				m.executed = true
				return m, tea.Quit
			}
			// Se o carrinho estiver vazio, abre o modal de detalhes do item no cursor
			if m.tab == TabPackages {
				if len(m.visibleSuites) > 0 && m.cursor >= 0 && m.cursor < len(m.visibleSuites) {
					suite := m.visibleSuites[m.cursor]
					m.modalSuite = &suite
					m.modalTool = nil
					m.showModal = true
					m.syncModalViewport()
				}
			} else {
				if len(m.visibleTools) > 0 && m.cursor >= 0 && m.cursor < len(m.visibleTools) {
					tool := m.visibleTools[m.cursor]
					m.modalTool = &tool
					m.modalSuite = nil
					m.showModal = true
					m.syncModalViewport()
				}
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return "Operação cancelada pelo usuário.\n"
	}

	if m.showModal {
		return m.renderModalView()
	}

	var b strings.Builder

	osLabel := string(m.sysInfo.OS)
	if m.sysInfo.DistroPretty != "" {
		osLabel = fmt.Sprintf("%s (%s / %s)", m.sysInfo.OS, m.sysInfo.Distro, m.sysInfo.PrimaryManager)
	} else if m.sysInfo.OS == system.OSWindows {
		osLabel = fmt.Sprintf("Windows (%s / %s)", m.sysInfo.Arch, m.sysInfo.PrimaryManager)
	}

	isCompact := m.height > 0 && m.height < 22

	b.WriteString(titleStyle.Render(" MULTITOOL "))
	b.WriteString(fmt.Sprintf(" %s", badgeStyle.Render(osLabel)))
	if m.dryRun {
		b.WriteString(fmt.Sprintf(" %s", lipgloss.NewStyle().Background(lipgloss.Color("#D97706")).Padding(0, 1).Bold(true).Render("DRY-RUN")))
	}
	if isCompact {
		b.WriteString("\n")
	} else {
		b.WriteString("\n\n")
	}

	modeBadge := modeInstallStyle.Render("[ MODO: INSTALAR ]")
	if m.mode == ModeUninstall {
		modeBadge = modeUninstallStyle.Render("[ MODO: DESINSTALAR ]")
	}
	cartBadge := cartStyle.Render(fmt.Sprintf("Carrinho: %d item(ns)", len(m.selected)))
	b.WriteString(fmt.Sprintf("%s    %s", modeBadge, cartBadge))
	if isCompact {
		b.WriteString("\n")
	} else {
		b.WriteString("\n\n")
	}

	renderTab := func(key, label string, active bool) string {
		txt := fmt.Sprintf("[%s] %s", key, label)
		if active {
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#2563EB")).Padding(0, 1).Render(txt)
		}
		return dimStyle.Render(txt)
	}

	labelCod := "Código"
	labelSis := "Sistema"
	labelSeg := "Segurança"
	labelAll := "Todas"
	labelPkg := "Pacotes"
	labelIns := "Instalados"
	labelCrt := "Carrinho"
	prefix := headerStyle.Render("Abas: ")

	if m.width < 118 {
		labelCod = "Cód"
		labelSis = "Sis"
		labelSeg = "Seg"
		labelPkg = "Pacotes"
		labelIns = "Inst"
		labelCrt = "Carrinho"
	}
	if m.width < 100 {
		labelPkg = "Pkgs"
		labelCrt = "Car"
	}
	if m.width < 85 {
		prefix = ""
		labelAll = "Todo"
		labelPkg = "Pkg"
		labelIns = "Inst"
		labelCrt = "Car"
	}

	tabIA := renderTab("1", "IA", m.tab == TabTopo && m.activeTopo == "IA")
	tabCod := renderTab("2", labelCod, m.tab == TabTopo && m.activeTopo == "Código")
	tabSis := renderTab("3", labelSis, m.tab == TabTopo && m.activeTopo == "Sistema")
	tabApp := renderTab("4", "Apps", m.tab == TabTopo && m.activeTopo == "Apps")
	tabSeg := renderTab("5", labelSeg, m.tab == TabTopo && m.activeTopo == "Segurança")
	tabAll := renderTab("t", labelAll, m.tab == TabAll)
	tabPkg := renderTab("p", labelPkg, m.tab == TabPackages)
	tabIns := renderTab("i", labelIns, m.tab == TabInstalled)
	tabCrt := renderTab("c", labelCrt, m.tab == TabCart)

	b.WriteString(fmt.Sprintf("%s%s %s %s %s %s | %s %s %s %s\n",
		prefix, tabIA, tabCod, tabSis, tabApp, tabSeg, tabAll, tabPkg, tabIns, tabCrt))

	if m.searching {
		b.WriteString(fmt.Sprintf("%s %s█\n", selectedStyle.Render("Buscar:"), m.searchQuery))
	} else if m.searchQuery != "" {
		b.WriteString(fmt.Sprintf("%s %s (pressione '/' para editar ou 'Esc' para limpar)\n", dimStyle.Render("Filtro:"), m.searchQuery))
	} else if !isCompact {
		b.WriteString("\n")
	}

	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	totalItems := len(m.visibleTools)
	if m.tab == TabPackages {
		totalItems = len(m.visibleSuites)
	}
	if totalItems > m.viewport.Height && m.viewport.Height > 0 {
		scrollPct := m.viewport.ScrollPercent() * 100
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Item %d de %d (rolagem: %.0f%% • use ↑/↓)\n", m.cursor+1, totalItems, scrollPct)))
	}

	shortcutText := "Atalhos: [↑/↓/j/k] Navegar • [Espaço] Marcar • [Enter] Detalhes/Executar • [a/A] Marcar Tudo/Limpar • [r] Modo • [/] Buscar • [q] Sair"
	if m.width < 135 {
		shortcutText = "Atalhos: [↑/↓] Navegar • [Espaço] Marcar • [Enter] Detalhes/Executar • [a/A] Tudo/Limpar • [r] Modo • [/] Buscar • [q] Sair"
	}
	if m.width < 125 {
		shortcutText = "Atalhos: [↑/↓] Navegar • [Espaço] Marcar • [Enter] Detalhes/Executar • [r] Modo • [/] Buscar • [q] Sair"
	}
	if m.width < 105 {
		shortcutText = "Atalhos: [↑/↓] Navegar • [Espaço] Marcar • [Enter] Ação • [r] Modo • [/] Busca • [q] Sair"
	}
	if m.width < 90 {
		shortcutText = "[↑/↓] Navegar • [Espaço] Marcar • [Enter] Ação • [r] Modo • [/] Busca • [q] Sair"
	}
	b.WriteString(dimStyle.Render(shortcutText + "\n"))

	return b.String()
}

func Run(reg *registry.Registry, dryRun bool) error {
	m := NewModel(reg, dryRun)
	p := tea.NewProgram(m, tea.WithAltScreen())
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
			var ids []string
			for _, t := range toRun {
				ids = append(ids, t.ID)
			}
			resolved, err := catalog.ResolveDependencies(ids)
			if err == nil {
				var eligible []catalog.Tool
				for _, t := range resolved {
					if !res.selected[t.ID] && r.IsDependencySatisfied(&t) {
						continue
					}
					eligible = append(eligible, t)
				}
				toRun = eligible
			}
			r.RunBatchInstall(toRun)
		} else {
			r.RunBatchUninstall(toRun)
		}
	}
	return nil
}
