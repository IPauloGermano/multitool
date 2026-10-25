package runner

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/mattn/go-isatty"
)

var (
	styleSpinner = lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Bold(true)
	styleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
	styleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true)
	styleDimTime = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	styleBarFull = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
	styleBarDim  = lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))
	styleBarPct  = lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA")).Bold(true)
	styleAct     = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
)

// getTerminalWidth obtém a largura visível do terminal ou fallback seguro.
func getTerminalWidth() int {
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// cleanActivity limpa caracteres de controle que poderiam desalinhar o terminal.
func cleanActivity(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.TrimSpace(s)
}

// truncateToWidth trunca uma string considerando caracteres unicode para não ultrapassar maxWidth.
func truncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w <= maxWidth {
		return s
	}

	target := maxWidth - 3
	if target <= 0 {
		runes := []rune(s)
		if len(runes) > maxWidth {
			return string(runes[:maxWidth])
		}
		return s
	}

	runes := []rune(s)
	var b strings.Builder
	cur := 0
	for _, r := range runes {
		rw := lipgloss.Width(string(r))
		if cur+rw > target {
			break
		}
		b.WriteRune(r)
		cur += rw
	}
	b.WriteString("...")
	return b.String()
}

// RenderProgressBar renderiza uma barra de progresso gráfica com Lip Gloss.
func RenderProgressBar(width int, pct float64) string {
	if width <= 0 {
		width = 12
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

	fullCount := int(float64(width) * pct)
	emptyCount := width - fullCount

	bar := styleBarFull.Render(strings.Repeat("━", fullCount)) + styleBarDim.Render(strings.Repeat("─", emptyCount))
	pctStr := styleBarPct.Render(fmt.Sprintf("%3.0f%%", pct*100))
	return fmt.Sprintf("[%s] %s", bar, pctStr)
}

// Spinner gerencia o feedback visual animado no terminal durante comandos longos.
type Spinner struct {
	frames    []string
	frameIdx  int
	prefix    string
	message   string
	activity  string
	start     time.Time
	stopChan  chan struct{}
	doneChan  chan struct{}
	stopped   bool
	mu        sync.Mutex
	isTTY     bool
}

// NewSpinner cria um novo indicador de progresso com detecção de TTY.
func NewSpinner(prefix, message string) *Spinner {
	isTerminal := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	return &Spinner{
		frames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		prefix:   prefix,
		message:  message,
		start:    time.Now(),
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
		isTTY:    isTerminal,
	}
}

// Start inicia a animação em background se for um terminal interativo.
func (s *Spinner) Start() {
	s.mu.Lock()
	if !s.isTTY {
		if s.prefix != "" {
			fmt.Printf("➜ %s %s...\n", s.prefix, s.message)
		} else {
			fmt.Printf("➜ %s...\n", s.message)
		}
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	go func() {
		defer close(s.doneChan)
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopChan:
				return
			case <-ticker.C:
				s.mu.Lock()
				frame := s.frames[s.frameIdx]
				s.frameIdx = (s.frameIdx + 1) % len(s.frames)
				prefix := s.prefix
				msg := s.message
				act := s.activity
				elapsed := time.Since(s.start).Round(time.Second)
				s.mu.Unlock()

				termWidth := getTerminalWidth()
				// Margem de segurança de 2 colunas para JAMAIS quebrar linha
				maxContent := termWidth - 2
				if maxContent < 20 {
					maxContent = 20
				}

				timeStr := styleDimTime.Render(fmt.Sprintf("[%s]", elapsed))
				timeWidth := lipgloss.Width(timeStr)
				frameStr := styleSpinner.Render(frame) + " "
				frameWidth := 2

				availForContent := maxContent - timeWidth - frameWidth - 1
				if availForContent < 10 {
					availForContent = 10
				}

				var body strings.Builder
				if prefix != "" {
					body.WriteString(prefix)
					body.WriteString(" ")
				}
				body.WriteString(msg)
				if act != "" {
					body.WriteString(" • ")
					body.WriteString(act)
				}

				lineBody := truncateToWidth(body.String(), availForContent)
				// Preenche com espaços para cobrir resíduos e evitar oscilação
				padding := availForContent - lipgloss.Width(lineBody)
				padStr := ""
				if padding > 0 {
					padStr = strings.Repeat(" ", padding)
				}

				fmt.Printf("\r\033[K%s%s%s %s", frameStr, lineBody, padStr, timeStr)
			}
		}
	}()
}

// Update altera a mensagem sendo exibida no spinner.
func (s *Spinner) Update(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = msg
}

// UpdatePrefix altera o prefixo (ex: barra de progresso do lote).
func (s *Spinner) UpdatePrefix(prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prefix = prefix
}

// UpdateActivity atualiza a linha de atividade real capturada do comando em execução.
func (s *Spinner) UpdateActivity(activity string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activity = cleanActivity(activity)
}

// Stop finaliza o spinner com um ícone de status e mensagem final.
func (s *Spinner) Stop(success bool, finalMsg string) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	isTTY := s.isTTY
	s.mu.Unlock()

	if isTTY {
		close(s.stopChan)
		<-s.doneChan
		fmt.Print("\r\033[K")
	}

	icon := styleSuccess.Render("✔")
	if !success {
		icon = styleError.Render("✖")
	}

	fmt.Printf("%s %s\n", icon, finalMsg)
}
