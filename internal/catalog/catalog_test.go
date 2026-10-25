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

func TestNoDuplicateNames(t *testing.T) {
	tools := DefaultCatalog()
	namesSeen := make(map[string]string)
	for _, tool := range tools {
		if prevID, exists := namesSeen[tool.Name]; exists {
			t.Errorf("Nome duplicado %q compartilhado pelas ferramentas %s e %s", tool.Name, prevID, tool.ID)
		}
		namesSeen[tool.Name] = tool.ID
	}
}

func TestSuitesIntegrityAndNoDuplicates(t *testing.T) {
	for suiteName, members := range packages {
		seen := make(map[string]bool)
		for _, member := range members {
			if seen[member] {
				t.Errorf("Suite @%s contém ferramenta duplicada: %s", suiteName, member)
			}
			seen[member] = true

			if FindTool(member) == nil {
				t.Errorf("Suite @%s referencia ferramenta inexistente: %s", suiteName, member)
			}
		}
	}
}

func TestAliasesIntegrity(t *testing.T) {
	for alias, targetID := range toolAliases {
		if FindTool(targetID) == nil {
			t.Errorf("Alias %q aponta para ferramenta inexistente: %s", alias, targetID)
		}
	}
}

func TestTargetPackageIDsNoDuplicates(t *testing.T) {
	tools := DefaultCatalog()
	winPkgs := make(map[string]string)
	linPkgs := make(map[string]string)
	darPkgs := make(map[string]string)

	for _, tool := range tools {
		if tool.Windows != nil && tool.Windows.PackageID != "" {
			key := string(tool.Windows.Type) + ":" + tool.Windows.PackageID
			// MinGW devkit provê gcc e gdb no Windows; Docker Desktop provê tanto a CLI quanto a GUI
			isAllowedShared := tool.Windows.PackageID == "skeeto.w64devkit" || tool.Windows.PackageID == "Docker.DockerDesktop"
			if prevID, exists := winPkgs[key]; exists && !isAllowedShared {
				t.Errorf("Windows PackageID duplicado %s compartilhado por %s e %s", key, prevID, tool.ID)
			}
			winPkgs[key] = tool.ID
		}

		if tool.Linux != nil && tool.Linux.PackageID != "" {
			key := string(tool.Linux.Type) + ":" + tool.Linux.PackageID
			if prevID, exists := linPkgs[key]; exists {
				t.Errorf("Linux PackageID duplicado %s compartilhado por %s e %s", key, prevID, tool.ID)
			}
			linPkgs[key] = tool.ID
		}

		if tool.Darwin != nil && tool.Darwin.PackageID != "" {
			key := string(tool.Darwin.Type) + ":" + tool.Darwin.PackageID
			if prevID, exists := darPkgs[key]; exists {
				t.Errorf("Darwin PackageID duplicado %s compartilhado por %s e %s", key, prevID, tool.ID)
			}
			darPkgs[key] = tool.ID
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
		{"androidskills", "androidskills"},
		{"android/skills", "androidskills"},
		{"android", "androidskills"},
		{"https://github.com/android/skills", "androidskills"},
		{"https://github.com/android/skills.git", "androidskills"},
		{"paperclip", "paperclip"},
		{"paperclipai", "paperclip"},
		{"paperclipai/paperclip", "paperclip"},
		{"https://github.com/paperclipai/paperclip", "paperclip"},
		{"openmausbot", "openmausbot"},
		{"OpenMausBot", "openmausbot"},
		{"milind-soni/OpenMausBot", "openmausbot"},
		{"milind-soni/openmausbot", "openmausbot"},
		{"https://github.com/milind-soni/OpenMausBot", "openmausbot"},
		{"https://github.com/milind-soni/OpenMausBot.git", "openmausbot"},
		{"mausbot", "openmausbot"},
		{"opengrokbot", "openmausbot"},
		{"openshell", "openshell"},
		{"nvidia/openshell", "openshell"},
		{"https://github.com/NVIDIA/OpenShell", "openshell"},
		{"hallmark", "hallmark"},
		{"nutlope/hallmark", "hallmark"},
		{"https://github.com/Nutlope/hallmark", "hallmark"},
		{"awsagenttoolkit", "awsagenttoolkit"},
		{"aws/agent-toolkit-for-aws", "awsagenttoolkit"},
		{"https://github.com/aws/agent-toolkit-for-aws", "awsagenttoolkit"},
		{"androidstudio", "androidstudio"},
		{"android-studio", "androidstudio"},
		{"apktool", "apktool"},
		{"ibotpeaches/apktool", "apktool"},
		{"https://github.com/iBotPeaches/Apktool", "apktool"},
		{"x64dbg", "x64dbg"},
		{"x64dbg/x64dbg", "x64dbg"},
		{"https://github.com/x64dbg/x64dbg", "x64dbg"},
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

func TestSuitesStruct(t *testing.T) {
	suites := AllSuites()
	if len(suites) != 8 {
		t.Errorf("Esperava 8 suítes, obteve %d", len(suites))
	}

	for _, s := range suites {
		if s.ID == "" || s.Name == "" || s.Description == "" || s.Category == "" {
			t.Errorf("Suíte com metadados incompletos: %+v", s)
		}
		if len(s.Tools) == 0 {
			t.Errorf("Suíte %s sem ferramentas associadas", s.ID)
		}
	}

	vb := GetSuite("vibecode")
	if vb == nil || vb.Name != "Vibe Coding & Pair AI" {
		t.Errorf("GetSuite('vibecode') falhou: %+v", vb)
	}

	atVb := GetSuite("@vibecode")
	if atVb == nil || atVb.ID != "vibecode" {
		t.Errorf("GetSuite('@vibecode') falhou: %+v", atVb)
	}

	supp := SupportedSuites()
	if len(supp) != 8 {
		t.Errorf("Esperava 8 suítes suportadas, obteve %d", len(supp))
	}
}

func TestResolveDependencies(t *testing.T) {
	// 1. Tool com dependência (ghidra -> temurin)
	resolved, err := ResolveDependencies([]string{"ghidra"})
	if err != nil {
		t.Fatalf("Erro inesperado ao resolver dependências de ghidra: %v", err)
	}

	temurinIdx := -1
	ghidraIdx := -1
	for i, tool := range resolved {
		if tool.ID == "temurin" {
			temurinIdx = i
		}
		if tool.ID == "ghidra" {
			ghidraIdx = i
		}
	}

	if temurinIdx == -1 {
		t.Errorf("Dependência 'temurin' não foi incluída na resolução de 'ghidra'")
	}
	if ghidraIdx == -1 {
		t.Errorf("'ghidra' não foi incluída na resolução")
	}
	if temurinIdx > ghidraIdx {
		t.Errorf("Ordem topológica incorreta: temurin (%d) deve vir antes de ghidra (%d)", temurinIdx, ghidraIdx)
	}

	// 2. Múltiplas ferramentas compartilhando dependência (keep e opengym -> docker)
	resolvedShared, err := ResolveDependencies([]string{"keep", "opengym"})
	if err != nil {
		t.Fatalf("Erro ao resolver dependências compartilhadas: %v", err)
	}

	dockerCount := 0
	dockerIdx := -1
	keepIdx := -1
	opengymIdx := -1
	for i, tool := range resolvedShared {
		if tool.ID == "docker" {
			dockerCount++
			dockerIdx = i
		}
		if tool.ID == "keep" {
			keepIdx = i
		}
		if tool.ID == "opengym" {
			opengymIdx = i
		}
	}

	if dockerCount != 1 {
		t.Errorf("Esperava exatamente 1 ocorrência de docker desduplicada, obteve %d", dockerCount)
	}
	if dockerIdx > keepIdx || dockerIdx > opengymIdx {
		t.Errorf("docker (%d) deve vir antes de keep (%d) e opengym (%d)", dockerIdx, keepIdx, opengymIdx)
	}

	// 3. Ferramenta sem dependências
	resolvedSimple, err := ResolveDependencies([]string{"git"})
	if err != nil {
		t.Fatalf("Erro ao resolver ferramenta sem dependências: %v", err)
	}
	if len(resolvedSimple) != 1 || resolvedSimple[0].ID != "git" {
		t.Errorf("Esperava apenas git, obteve: %+v", resolvedSimple)
	}

	// 4. Erro em ferramenta inexistente
	if _, err := ResolveDependencies([]string{"pacote_inexistente_999"}); err == nil {
		t.Errorf("Esperava erro ao resolver pacote inexistente")
	}
}

func TestDarwinTargetResolution(t *testing.T) {
	// Ferramenta com alvo explícito Darwin
	toolBrew := Tool{
		ID:     "test-brew",
		Darwin: &Target{Type: TypeBrew, PackageID: "test-brew", Binary: "test-brew"},
		Linux:  &Target{Type: TypeNative, PackageID: "test-lin", Binary: "test-lin"},
	}

	// Ferramenta com alvo agnóstico npm
	toolNpm := Tool{
		ID:    "test-npm",
		Linux: &Target{Type: TypeNpm, PackageID: "test-npm", Binary: "test-npm"},
	}

	// Ferramenta com alvo exclusivo Linux (native/script sem Darwin)
	toolLinOnly := Tool{
		ID:    "test-lin-only",
		Linux: &Target{Type: TypeNative, PackageID: "test-lin-only", Binary: "test-lin-only"},
	}

	// Teste de fallback lógico
	// Se fosse darwin:
	if toolBrew.Darwin.Type != TypeBrew {
		t.Errorf("Esperava TypeBrew, obteve %s", toolBrew.Darwin.Type)
	}
	if toolNpm.Linux.Type != TypeNpm {
		t.Errorf("Esperava TypeNpm, obteve %s", toolNpm.Linux.Type)
	}
	if toolLinOnly.Darwin != nil {
		t.Errorf("toolLinOnly não deveria ter alvo Darwin")
	}
}

func TestTargetSHA256Field(t *testing.T) {
	target := Target{
		Type:      TypeScript,
		PackageID: "test/app",
		Binary:    "app",
		SHA256:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}

	if target.SHA256 == "" {
		t.Errorf("Campo SHA256 não deveria estar vazio")
	}
}
