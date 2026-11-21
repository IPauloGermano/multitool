package catalog

import (
	"encoding/hex"
	"strings"
	"testing"
)

func classifyCatalogKind(tool Tool) string {
	if tool.Category == "Skills" {
		return "skill"
	}
	if tool.Category == "Desktop" {
		return "app"
	}
	return "tool"
}

func isValidTargetTypeForOS(osName string, tt TargetType) bool {
	switch osName {
	case "windows":
		switch tt {
		case TypeWinget, TypeNpm, TypePipx, TypeCargo, TypeDirect, TypeBrew:
			return true
		}
	case "linux":
		switch tt {
		case TypeNative, TypeNpm, TypePipx, TypeCargo, TypeFlatpak, TypeSkill, TypeScript, TypeBrew:
			return true
		}
	case "darwin":
		switch tt {
		case TypeBrew, TypeNpm, TypePipx, TypeCargo, TypeSkill, TypeScript:
			return true
		}
	}
	return false
}

func isValidSHA256Hex(h string) bool {
	if len(h) != 64 {
		return false
	}
	_, err := hex.DecodeString(h)
	return err == nil
}

func isValidToolID(id string) bool {
	if id == "" || id != strings.ToLower(id) {
		return false
	}
	for _, r := range id {
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if !isLower && !isDigit && r != '_' {
			return false
		}
	}
	return true
}

func TestIndividualCatalogEntries(t *testing.T) {
	tools := DefaultCatalog()
	if len(tools) == 0 {
		t.Fatal("catálogo padrão vazio")
	}

	validTopos := make(map[string]bool)
	for _, topo := range Topos() {
		validTopos[topo] = true
	}

	aliasesByID := make(map[string][]string)
	for alias, targetID := range toolAliases {
		aliasesByID[targetID] = append(aliasesByID[targetID], alias)
	}

	for _, tool := range tools {
		tool := tool
		subName := classifyCatalogKind(tool) + "/" + tool.ID
		t.Run(subName, func(t *testing.T) {
			// 1. Metadados obrigatórios e formato de ID
			if !isValidToolID(tool.ID) {
				t.Errorf("ID inválido %q: deve ser minúsculo e alfanumérico (ou '_')", tool.ID)
			}
			if strings.TrimSpace(tool.Name) == "" {
				t.Errorf("tool %q sem Name", tool.ID)
			}
			if strings.TrimSpace(tool.Category) == "" {
				t.Errorf("tool %q sem Category", tool.ID)
			}
			if strings.TrimSpace(tool.Description) == "" {
				t.Errorf("tool %q sem Description", tool.ID)
			}

			topo := TopoDaSub(tool.Category)
			if !validTopos[topo] {
				t.Errorf("categoria %q de %q mapeou para topo inválido %q", tool.Category, tool.ID, topo)
			}

			if tool.Windows == nil && tool.Linux == nil && tool.Darwin == nil {
				t.Errorf("tool %q não possui nenhum alvo definido (Windows/Linux/Darwin)", tool.ID)
			}

			// 2. Resolução via FindTool (ID exato, maiúsculo e aliases)
			if got := FindTool(tool.ID); got == nil || got.ID != tool.ID {
				t.Errorf("FindTool(%q) = %+v, esperado ID %q", tool.ID, got, tool.ID)
			}
			if got := FindTool(strings.ToUpper(tool.ID)); got == nil || got.ID != tool.ID {
				t.Errorf("FindTool(%q) = %+v, esperado ID %q", strings.ToUpper(tool.ID), got, tool.ID)
			}
			for _, alias := range aliasesByID[tool.ID] {
				if got := FindTool(alias); got == nil || got.ID != tool.ID {
					t.Errorf("alias %q deveria resolver para %q, obteve %+v", alias, tool.ID, got)
				}
			}

			// 3. URL upstream quando disponível
			if repo := GetUpstreamRepo(tool.ID); repo != "" {
				if !strings.HasPrefix(repo, "https://") && !strings.HasPrefix(repo, "http://") {
					t.Errorf("GetUpstreamRepo(%q) retornou URL inválida: %q", tool.ID, repo)
				}
			}

			// 4. Validação de alvos multiplataforma (Windows, Linux, Darwin)
			targets := map[string]*Target{
				"windows": tool.Windows,
				"linux":   tool.Linux,
				"darwin":  tool.Darwin,
			}
			for osName, target := range targets {
				if target == nil {
					continue
				}
				if !isValidTargetTypeForOS(osName, target.Type) {
					t.Errorf("alvo %s de %q usa TargetType incompatível: %q", osName, tool.ID, target.Type)
				}
				if strings.TrimSpace(target.PackageID) == "" {
					t.Errorf("alvo %s de %q com PackageID vazio", osName, tool.ID)
				}
				if strings.TrimSpace(target.Binary) == "" {
					t.Errorf("alvo %s de %q com Binary vazio", osName, tool.ID)
				}
				if target.SHA256 != "" && len(target.ArtifactSHA256) > 0 {
					t.Errorf("alvo %s de %q define SHA256 e ArtifactSHA256 simultaneamente", osName, tool.ID)
				}
				if target.SHA256 != "" && !isValidSHA256Hex(target.SHA256) {
					t.Errorf("alvo %s de %q possui SHA256 inválido: %q", osName, tool.ID, target.SHA256)
				}
				for artName, artHash := range target.ArtifactSHA256 {
					if !isValidSHA256Hex(artHash) {
						t.Errorf("alvo %s de %q possui ArtifactSHA256 inválido para %q: %q", osName, tool.ID, artName, artHash)
					}
				}
			}

			// Consistência de TargetForOS
			if tool.TargetForOS("windows") != tool.Windows {
				t.Errorf("TargetForOS(windows) divergiu de tool.Windows para %q", tool.ID)
			}
			if tool.TargetForOS("linux") != tool.Linux {
				t.Errorf("TargetForOS(linux) divergiu de tool.Linux para %q", tool.ID)
			}
			darwinResolved := tool.TargetForOS("darwin")
			if tool.Darwin != nil && darwinResolved != tool.Darwin {
				t.Errorf("TargetForOS(darwin) deveria retornar tool.Darwin explícito para %q", tool.ID)
			}
			if tool.Darwin == nil && tool.Linux != nil {
				switch tool.Linux.Type {
				case TypeNpm, TypePipx, TypeCargo, TypeSkill:
					if darwinResolved != tool.Linux {
						t.Errorf("TargetForOS(darwin) deveria fazer fallback agnóstico para Linux (%s) em %q", tool.Linux.Type, tool.ID)
					}
				default:
					if darwinResolved != nil {
						t.Errorf("TargetForOS(darwin) não deveria fazer fallback para tipo %s em %q", tool.Linux.Type, tool.ID)
					}
				}
			}

			// 5. Regras específicas para Skills de IA
			if tool.Category == "Skills" {
				if tool.Linux == nil || tool.Linux.Type != TypeSkill {
					t.Errorf("skill %q deve ter alvo Linux com TypeSkill, obteve %+v", tool.ID, tool.Linux)
				} else {
					if tool.Linux.Binary != "SKILL.md" {
						t.Errorf("skill %q deve ter Linux.Binary == SKILL.md, obteve %q", tool.ID, tool.Linux.Binary)
					}
					if !strings.Contains(tool.Linux.PackageID, "/") {
						t.Errorf("skill %q deve ter Linux.PackageID no formato owner/repo, obteve %q", tool.ID, tool.Linux.PackageID)
					}
				}
				if tool.Windows != nil {
					if tool.Windows.Type != TypeDirect {
						t.Errorf("skill %q no Windows deve usar TypeDirect, obteve %q", tool.ID, tool.Windows.Type)
					}
					if tool.Windows.Binary != "SKILL.md" {
						t.Errorf("skill %q no Windows deve ter Binary == SKILL.md, obteve %q", tool.ID, tool.Windows.Binary)
					}
					if tool.Linux != nil && tool.Windows.PackageID != tool.Linux.PackageID {
						t.Errorf("skill %q possui PackageID divergente entre Windows (%q) e Linux (%q)", tool.ID, tool.Windows.PackageID, tool.Linux.PackageID)
					}
				}
				if darwinResolved == nil || darwinResolved.Type != TypeSkill {
					t.Errorf("skill %q deve resolver alvo TypeSkill em macOS/Darwin, obteve %+v", tool.ID, darwinResolved)
				}
			}

			// 6. Dependências e resolução topológica
			for _, depID := range tool.Dependencies {
				if depID == tool.ID {
					t.Errorf("tool %q não pode depender de si mesma", tool.ID)
				}
				if FindTool(depID) == nil {
					t.Errorf("dependência %q de %q não existe no catálogo", depID, tool.ID)
				}
			}
			resolved, err := ResolveDependencies([]string{tool.ID})
			if err != nil {
				t.Errorf("ResolveDependencies([%q]) falhou: %v", tool.ID, err)
			}
			if tool.IsSupportedOnCurrentOS() {
				pos := make(map[string]int, len(resolved))
				for i, item := range resolved {
					pos[item.ID] = i
				}
				toolIdx, ok := pos[tool.ID]
				if !ok {
					t.Errorf("ResolveDependencies([%q]) não retornou %q", tool.ID, tool.ID)
				}
				for _, depID := range tool.Dependencies {
					dep := FindTool(depID)
					if dep != nil && dep.IsSupportedOnCurrentOS() {
						depIdx, depOK := pos[depID]
						if !depOK {
							t.Errorf("dependência suportada %q ausente na resolução de %q", depID, tool.ID)
						} else if depIdx >= toolIdx {
							t.Errorf("ordem topológica inválida para %q: dep %q (%d) deve preceder %q (%d)", tool.ID, depID, depIdx, tool.ID, toolIdx)
						}
					}
				}

				// 7. Presença em Search e ToolsByTopo quando suportado no SO atual
				foundSearch := false
				for _, res := range Search(tool.ID) {
					if res.ID == tool.ID {
						foundSearch = true
						break
					}
				}
				if !foundSearch {
					t.Errorf("Search(%q) não retornou a própria ferramenta suportada", tool.ID)
				}

				foundTopo := false
				for _, res := range ToolsByTopo(topo) {
					if res.ID == tool.ID {
						foundTopo = true
						break
					}
				}
				if !foundTopo {
					t.Errorf("ToolsByTopo(%q) não contém %q", topo, tool.ID)
				}
			}
		})
	}
}
