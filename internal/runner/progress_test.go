package runner

import (
	"strings"
	"testing"
	"time"
)

func TestRenderProgressBar(t *testing.T) {
	// Teste limites
	bar0 := RenderProgressBar(10, 0.0)
	if !strings.Contains(bar0, "0%") {
		t.Errorf("Esperava 0%%, obteve: %s", bar0)
	}

	bar50 := RenderProgressBar(10, 0.5)
	if !strings.Contains(bar50, "50%") {
		t.Errorf("Esperava 50%%, obteve: %s", bar50)
	}

	bar100 := RenderProgressBar(10, 1.0)
	if !strings.Contains(bar100, "100%") {
		t.Errorf("Esperava 100%%, obteve: %s", bar100)
	}

	// Teste overflow e underflow
	barNeg := RenderProgressBar(10, -0.2)
	if !strings.Contains(barNeg, "0%") {
		t.Errorf("Esperava clamp para 0%%, obteve: %s", barNeg)
	}

	barOver := RenderProgressBar(10, 1.5)
	if !strings.Contains(barOver, "100%") {
		t.Errorf("Esperava clamp para 100%%, obteve: %s", barOver)
	}
}

func TestSpinnerLifecycle(t *testing.T) {
	sp := NewSpinner("teste", "Executando teste unitário")
	sp.Start()
	time.Sleep(50 * time.Millisecond)
	sp.Update("Atualizando etapa")
	sp.UpdatePrefix("[1/1]")
	sp.Stop(true, "Finalizado com sucesso")

	// Segundo Stop não deve causar pânico (idempotente)
	sp.Stop(true, "Ignorado")
}
