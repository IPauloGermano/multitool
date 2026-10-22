package catalog

import (
	"strings"
	"testing"
)

func TestCatalogIntegrity(t *testing.T) {
	tools := DefaultCatalog()
	if len(tools) == 0 {
		t.Fatalf("Catálogo padrão não pode estar vazio")
	}

	seen := make(map[string]bool)
	for _, tool := range tools {
		if tool.ID == "" {
			t.Errorf("Ferramenta com ID vazio encontrada: %+v", tool)
		}
		if seen[tool.ID] {
			t.Errorf("ID duplicado no catálogo: %s", tool.ID)
		}
		seen[tool.ID] = true

		if tool.Name == "" {
			t.Errorf("Ferramenta %s sem nome", tool.ID)
		}
		if tool.Category == "" {
			t.Errorf("Ferramenta %s sem categoria", tool.ID)
		}
		if tool.Description == "" {
			t.Errorf("Ferramenta %s sem descrição", tool.ID)
		}
	}
}

func TestFindToolAndAliases(t *testing.T) {
	testCases := []struct {
		query  string
		wantID string
	}{
		{"git", "git"},
		{"Git", "git"},
		{"diagram-design", "diagramdesign"},
		{"cathrynlavery/diagram-design", "diagramdesign"},
		{"https://github.com/cathrynlavery/diagram-design.git", "diagramdesign"},
		{"universal-modder", "universalmodder"},
		{"rehan-remade/universal-modder", "universalmodder"},
		{"https://github.com/rehan-remade/universal-modder", "universalmodder"},
		{"rea-agents", "rea"},
		{"decolua/9router", "9router"},
		{"diegosouzapw/omniroute", "omniroute"},
		{"hermes-agent", "hermes"},
		{"nousresearch/hermes", "hermes"},
		{"orca", "orca"},
		{"stablyai/orca", "orca"},
		{"stably-orca", "orca"},
		{"https://github.com/stablyai/orca", "orca"},
	}

	for _, tc := range testCases {
		tool := FindTool(tc.query)
		if tool == nil {
			t.Errorf("FindTool(%q) retornou nil, esperado ID %q", tc.query, tc.wantID)
			continue
		}
		if tool.ID != tc.wantID {
			t.Errorf("FindTool(%q) = %q, esperado %q", tc.query, tool.ID, tc.wantID)
		}
	}
}

func TestExpandSuites(t *testing.T) {
	expanded, err := Expand([]string{"@vibecode", "git"})
	if err != nil {
		t.Fatalf("Erro ao expandir: %v", err)
	}

	hasGit := false
	for _, id := range expanded {
		if id == "git" {
			hasGit = true
			break
		}
	}
	if !hasGit {
		t.Errorf("Esperado 'git' na lista expandida")
	}

	// Verifica se pacote inválido retorna erro
	if _, err := Expand([]string{"@inexistente_xyz"}); err == nil {
		t.Errorf("Esperava erro ao expandir pacote inexistente")
	}
}

func TestToolsByTopo(t *testing.T) {
	topos := Topos()
	if len(topos) != 5 {
		t.Errorf("Esperava 5 topos, veio %d", len(topos))
	}

	for _, topo := range topos {
		tools := ToolsByTopo(topo)
		for _, tool := range tools {
			gotTopo := TopoDaSub(tool.Category)
			if gotTopo != topo {
				t.Errorf("Ferramenta %s na categoria %s mapeou para topo %s, esperado %s",
					tool.ID, tool.Category, gotTopo, topo)
			}
		}
	}
}

func TestSearch(t *testing.T) {
	results := Search("terminal")
	if len(results) == 0 {
		t.Errorf("Busca por 'terminal' não deveria vir vazia")
	}

	found := false
	for _, tool := range results {
		if strings.Contains(strings.ToLower(tool.Name+tool.Description+tool.Category), "terminal") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Resultados de busca não continham o termo buscado")
	}
}
