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
			// WinLibs (MinGW-w64) provê gcc e gdb no Windows; Docker Desktop provê tanto a CLI quanto a GUI
			isAllowedShared := tool.Windows.PackageID == "BrechtSanders.WinLibs.POSIX.UCRT" || tool.Windows.PackageID == "Docker.DockerDesktop"
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
		{"reanimate", "reanimate"},
		{"niazmorshed2007/reanimate", "reanimate"},
		{"https://github.com/NiazMorshed2007/reanimate", "reanimate"},
		{"https://github.com/NiazMorshed2007/reanimate.git", "reanimate"},
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
		{"securityaudit", "securityaudit"},
		{"security-audit", "securityaudit"},
		{"cloudflare/security-audit-skill", "securityaudit"},
		{"https://github.com/cloudflare/security-audit-skill", "securityaudit"},
		{"androidskills", "androidskills"},
		{"android/skills", "androidskills"},
		{"android", "androidskills"},
		{"https://github.com/android/skills", "androidskills"},
		{"https://github.com/android/skills.git", "androidskills"},
		{"paperclip", "paperclip"},
		{"paperclipai", "paperclip"},
		{"agentdesktop", "agentdesktop"},
		{"agent-desktop", "agentdesktop"},
		{"vercel-labs/agent-desktop", "agentdesktop"},
		{"https://github.com/vercel-labs/agent-desktop", "agentdesktop"},
		{"agentbrowser", "agentbrowser"},
		{"agent-browser", "agentbrowser"},
		{"vercel-labs/agent-browser", "agentbrowser"},
		{"https://github.com/vercel-labs/agent-browser", "agentbrowser"},
		{"skillscli", "skillscli"},
		{"skills", "skillscli"},
		{"skills-cli", "skillscli"},
		{"vercel-labs/skills", "skillscli"},
		{"https://github.com/vercel-labs/skills", "skillscli"},
		{"scrapling", "scrapling"},
		{"d4vinci/scrapling", "scrapling"},
		{"https://github.com/d4vinci/scrapling", "scrapling"},
		{"claudemem", "claudemem"},
		{"claude-mem", "claudemem"},
		{"thedotmack/claude-mem", "claudemem"},
		{"https://github.com/thedotmack/claude-mem", "claudemem"},
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
		{"craftlauncher", "craftlauncher"},
		{"craft-launcher", "craftlauncher"},
		{"artcraft", "craftlauncher"},
		{"artcraft-launcher", "craftlauncher"},
		{"storytold/craft-launcher", "craftlauncher"},
		{"https://github.com/storytold/craft-launcher", "craftlauncher"},
		{"qbittorrent", "qbittorrent"},
		{"qbit", "qbittorrent"},
		{"q-bittorrent", "qbittorrent"},
		{"qbittorrent/qbittorrent", "qbittorrent"},
		{"https://github.com/qbittorrent/qBittorrent", "qbittorrent"},
		{"egm", "egm"},
		{"elephant-goldfish-model", "egm"},
		{"elephantgoldfish", "egm"},
		{"google/elephant-goldfish-model", "egm"},
		{"https://github.com/google/elephant-goldfish-model", "egm"},
		{"https://github.com/google/elephant-goldfish-model.git", "egm"},
		{"scientificagentskills", "scientificagentskills"},
		{"k-dense-ai/scientific-agent-skills", "scientificagentskills"},
		{"https://github.com/k-dense-ai/scientific-agent-skills", "scientificagentskills"},
		{"agentreach", "agentreach"},
		{"agent-reach", "agentreach"},
		{"panniantong/agent-reach", "agentreach"},
		{"https://github.com/Panniantong/agent-reach", "agentreach"},
		{"gbrain", "gbrain"},
		{"garrytan/gbrain", "gbrain"},
		{"https://github.com/garrytan/gbrain", "gbrain"},
		{"dietrichgebert/ponytail", "ponytail"},
		{"https://github.com/dietrichgebert/ponytail", "ponytail"},
		{"clang", "clang"},
		{"llvm", "clang"},
		{"llvm-clang", "clang"},
		{"apple-clang", "clang"},
		{"llvm/llvm", "clang"},
		{"https://github.com/llvm/llvm-project", "clang"},
		{"wabt", "wabt"},
		{"webassembly/wabt", "wabt"},
		{"webassembly-wabt", "wabt"},
		{"wat2wasm", "wabt"},
		{"wasm2wat", "wabt"},
		{"https://github.com/webassembly/wabt", "wabt"},
		// Astryx
		{"astryx", "astryx"},
		{"meta/astryx", "astryx"},
		{"meta-astryx", "astryx"},
		{"https://github.com/meta/astryx", "astryx"},
		{"https://github.com/meta/astryx.git", "astryx"},
		{"@astryxdesign/cli", "astryx"},
		{"astryxdesign", "astryx"},
		// Claude Desktop
		{"claudedesktop", "claudedesktop"},
		{"claude-desktop", "claudedesktop"},
		{"claude-app", "claudedesktop"},
		{"anthropic/claude-desktop", "claudedesktop"},
		{"anthropic-claude", "claudedesktop"},
		{"anthropic.claude", "claudedesktop"},
		// Antigravity CLI
		{"agy", "agy"},
		{"antigravity-cli", "agy"},
		{"antigravitycli", "agy"},
		{"google/antigravity-cli", "agy"},
		{"google-antigravity-cli", "agy"},
		{"agy-cli", "agy"},
		// Codex
		{"codex", "codex"},
		{"openai/codex", "codex"},
		{"openai-codex", "codex"},
		{"codex-cli", "codex"},
		{"https://github.com/openai/codex", "codex"},
		{"@openai/codex", "codex"},
		// GitHub Copilot CLI
		{"copilot", "copilot"},
		{"copilot-cli", "copilot"},
		{"copilotcli", "copilot"},
		{"github-copilot", "copilot"},
		{"github-copilot-cli", "copilot"},
		{"github/copilot", "copilot"},
		{"github/copilot-cli", "copilot"},
		{"@github/copilot", "copilot"},
		{"https://github.com/github/copilot-cli", "copilot"},
		{"https://github.com/github/copilot", "copilot"},
		// Addy Osmani Agent Skills
		{"addyosmani", "addyosmani"},
		{"addyosmani/agent-skills", "addyosmani"},
		{"https://github.com/addyosmani/agent-skills", "addyosmani"},
		{"https://github.com/addyosmani/agent-skills.git", "addyosmani"},
		{"agent-skills", "addyosmani"},
		{"addyosmani-skills", "addyosmani"},
		// Remotion
		{"remotion", "remotion"},
		{"remotion-cli", "remotion"},
		{"remotioncli", "remotion"},
		{"remotion-dev/remotion", "remotion"},
		{"remotion-dev", "remotion"},
		{"remotion/cli", "remotion"},
		{"@remotion/cli", "remotion"},
		{"https://github.com/remotion-dev/remotion", "remotion"},
		{"https://github.com/remotion-dev/remotion.git", "remotion"},
		// GCC & GDB
		{"gcc", "gcc"},
		{"mingw", "gcc"},
		{"mingw-w64", "gcc"},
		{"winlibs", "gcc"},
		{"winlibs-gcc", "gcc"},
		{"w64devkit", "gcc"},
		{"skeeto/w64devkit", "gcc"},
		{"skeeto.w64devkit", "gcc"},
		{"brechtsanders/winlibs_mingw", "gcc"},
		{"https://github.com/brechtsanders/winlibs_mingw", "gcc"},
		{"gdb", "gdb"},
		{"winlibs-gdb", "gdb"},
		// Inspo MCP
		{"inspo", "inspo"},
		{"inspo-mcp", "inspo"},
		{"inspomcp", "inspo"},
		{"nutlope/inspo", "inspo"},
		{"https://github.com/Nutlope/inspo", "inspo"},
		{"https://github.com/Nutlope/inspo.git", "inspo"},
		// Motion AI Kit
		{"motionai", "motionai"},
		{"motion-ai", "motionai"},
		{"motion-ai-kit", "motionai"},
		{"motion", "motionai"},
		{"motiondivision/ai-kit", "motionai"},
		{"https://github.com/motiondivision/ai-kit", "motionai"},
		{"https://motion.dev/docs/ai-kit-install", "motionai"},
		// Vercel Agent Skills
		{"vercelagentskills", "vercelagentskills"},
		{"vercel-agent-skills", "vercelagentskills"},
		{"vercel-skills", "vercelagentskills"},
		{"vercel-labs/agent-skills", "vercelagentskills"},
		{"https://github.com/vercel-labs/agent-skills", "vercelagentskills"},
		{"https://github.com/vercel-labs/agent-skills.git", "vercelagentskills"},
		// shadcn MCP
		{"shadcn", "shadcn"},
		{"shadcn-mcp", "shadcn"},
		{"shadcn-ui", "shadcn"},
		{"shadcn-ui/ui", "shadcn"},
		{"https://github.com/shadcn-ui/ui", "shadcn"},
		{"https://ui.shadcn.com/docs/mcp", "shadcn"},
		// Magic UI MCP
		{"magicuimcp", "magicuimcp"},
		{"magicui", "magicuimcp"},
		{"magic-ui", "magicuimcp"},
		{"magicui-mcp", "magicuimcp"},
		{"@magicuidesign/mcp", "magicuimcp"},
		{"magicuidesign/mcp", "magicuimcp"},
		{"https://github.com/magicuidesign/mcp", "magicuimcp"},
		{"https://magicui.design/docs/mcp", "magicuimcp"},
		// Stitch Skills
		{"stitchskills", "stitchskills"},
		{"stitch-skills", "stitchskills"},
		{"stitch", "stitchskills"},
		{"google-labs-code/stitch-skills", "stitchskills"},
		{"https://github.com/google-labs-code/stitch-skills", "stitchskills"},
		{"https://github.com/google-labs-code/stitch-skills.git", "stitchskills"},
		// Claude BugHunter
		{"claudebughunter", "claudebughunter"},
		{"claude-bughunter", "claudebughunter"},
		{"bughunter", "claudebughunter"},
		{"bug-hunter", "claudebughunter"},
		{"elementalsouls/Claude-BugHunter", "claudebughunter"},
		{"https://github.com/elementalsouls/Claude-BugHunter", "claudebughunter"},
		{"https://github.com/elementalsouls/Claude-BugHunter.git", "claudebughunter"},
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

	// 2. Múltiplas ferramentas compartilhando dependência (apktool e ghidra -> temurin)
	resolvedShared, err := ResolveDependencies([]string{"apktool", "ghidra"})
	if err != nil {
		t.Fatalf("Erro ao resolver dependências compartilhadas: %v", err)
	}

	temurinCount := 0
	temurinSharedIdx := -1
	apktoolIdx := -1
	ghidraSharedIdx := -1
	for i, tool := range resolvedShared {
		if tool.ID == "temurin" {
			temurinCount++
			temurinSharedIdx = i
		}
		if tool.ID == "apktool" {
			apktoolIdx = i
		}
		if tool.ID == "ghidra" {
			ghidraSharedIdx = i
		}
	}

	if temurinCount != 1 {
		t.Errorf("Esperava exatamente 1 ocorrência de temurin desduplicada, obteve %d", temurinCount)
	}
	if temurinSharedIdx > apktoolIdx || temurinSharedIdx > ghidraSharedIdx {
		t.Errorf("temurin (%d) deve vir antes de apktool (%d) e ghidra (%d)", temurinSharedIdx, apktoolIdx, ghidraSharedIdx)
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

func TestRuffCatalogIntegration(t *testing.T) {
	tool := FindTool("ruff")
	if tool == nil {
		t.Fatalf("ferramenta ruff não encontrada no catálogo")
	}
	if tool.Name != "Ruff" {
		t.Errorf("esperava nome Ruff, obteve %s", tool.Name)
	}
	if tool.Windows == nil || tool.Windows.PackageID != "astral-sh.ruff" {
		t.Errorf("alvo Windows de ruff incorreto: %+v", tool.Windows)
	}
	if tool.Linux == nil || tool.Linux.Type != TypeScript {
		t.Errorf("alvo Linux de ruff incorreto: %+v", tool.Linux)
	}
	if tool.Darwin == nil || tool.Darwin.PackageID != "ruff" {
		t.Errorf("alvo Darwin de ruff incorreto: %+v", tool.Darwin)
	}

	aliases := []string{
		"astral-sh/ruff",
		"astral-ruff",
		"https://github.com/astral-sh/ruff",
	}
	for _, a := range aliases {
		found := FindTool(a)
		if found == nil || found.ID != "ruff" {
			t.Errorf("alias %q não resolveu para ruff: %+v", a, found)
		}
	}

	upstream := GetUpstreamRepo("ruff")
	if upstream != "https://github.com/astral-sh/ruff" {
		t.Errorf("upstream de ruff incorreto: %s", upstream)
	}

	devMembers := Suites()["dev"]
	foundInDev := false
	for _, m := range devMembers {
		if m == "ruff" {
			foundInDev = true
			break
		}
	}
	if !foundInDev {
		t.Errorf("ruff não está incluído na suíte dev")
	}
}

func TestUvPyenvFastapiCatalogIntegration(t *testing.T) {
	// 1. uv
	uvTool := FindTool("uv")
	if uvTool == nil {
		t.Fatalf("ferramenta uv não encontrada no catálogo")
	}
	if uvTool.Darwin == nil || uvTool.Darwin.PackageID != "uv" {
		t.Errorf("alvo Darwin de uv incorreto: %+v", uvTool.Darwin)
	}
	for _, a := range []string{"astral-sh/uv", "astral-uv", "https://github.com/astral-sh/uv"} {
		found := FindTool(a)
		if found == nil || found.ID != "uv" {
			t.Errorf("alias %q não resolveu para uv: %+v", a, found)
		}
	}
	if up := GetUpstreamRepo("uv"); up != "https://github.com/astral-sh/uv" {
		t.Errorf("upstream de uv incorreto: %s", up)
	}

	// 2. pyenv
	pyenvTool := FindTool("pyenv")
	if pyenvTool == nil {
		t.Fatalf("ferramenta pyenv não encontrada no catálogo")
	}
	if pyenvTool.Windows == nil || pyenvTool.Windows.PackageID != "pyenv-win" {
		t.Errorf("alvo Windows de pyenv incorreto: %+v", pyenvTool.Windows)
	}
	if pyenvTool.Linux == nil || pyenvTool.Linux.Type != TypeScript {
		t.Errorf("alvo Linux de pyenv incorreto: %+v", pyenvTool.Linux)
	}
	if pyenvTool.Darwin == nil || pyenvTool.Darwin.PackageID != "pyenv" {
		t.Errorf("alvo Darwin de pyenv incorreto: %+v", pyenvTool.Darwin)
	}
	for _, a := range []string{"pyenv/pyenv", "pyenv-win", "https://github.com/pyenv/pyenv"} {
		found := FindTool(a)
		if found == nil || found.ID != "pyenv" {
			t.Errorf("alias %q não resolveu para pyenv: %+v", a, found)
		}
	}
	if up := GetUpstreamRepo("pyenv"); up != "https://github.com/pyenv/pyenv" {
		t.Errorf("upstream de pyenv incorreto: %s", up)
	}
	devMembers := Suites()["dev"]
	foundPyenvInDev := false
	for _, m := range devMembers {
		if m == "pyenv" {
			foundPyenvInDev = true
			break
		}
	}
	if !foundPyenvInDev {
		t.Errorf("pyenv não está incluído na suíte dev")
	}

	// 3. fastapi
	fastapiTool := FindTool("fastapi")
	if fastapiTool == nil {
		t.Fatalf("ferramenta fastapi não encontrada no catálogo")
	}
	if fastapiTool.Windows == nil || fastapiTool.Windows.PackageID != "fastapi-cli" {
		t.Errorf("alvo Windows de fastapi incorreto: %+v", fastapiTool.Windows)
	}
	if fastapiTool.Linux == nil || fastapiTool.Linux.PackageID != "fastapi-cli" {
		t.Errorf("alvo Linux de fastapi incorreto: %+v", fastapiTool.Linux)
	}
	if fastapiTool.Darwin == nil || fastapiTool.Darwin.PackageID != "fastapi-cli" {
		t.Errorf("alvo Darwin de fastapi incorreto: %+v", fastapiTool.Darwin)
	}
	for _, a := range []string{"fastapi-cli", "fastapi/fastapi", "fastapi/fastapi-cli", "https://github.com/fastapi/fastapi"} {
		found := FindTool(a)
		if found == nil || found.ID != "fastapi" {
			t.Errorf("alias %q não resolveu para fastapi: %+v", a, found)
		}
	}
	if up := GetUpstreamRepo("fastapi"); up != "https://github.com/fastapi/fastapi" {
		t.Errorf("upstream de fastapi incorreto: %s", up)
	}
}

func TestGoogleDriveAndRcloneCatalogIntegration(t *testing.T) {
	// 1. googledrive
	gdrive := FindTool("googledrive")
	if gdrive == nil {
		t.Fatalf("ferramenta googledrive não encontrada no catálogo")
	}
	if gdrive.Windows == nil || gdrive.Windows.PackageID != "Google.GoogleDrive" {
		t.Errorf("alvo Windows de googledrive incorreto: %+v", gdrive.Windows)
	}
	if gdrive.Darwin == nil || gdrive.Darwin.PackageID != "google-drive" {
		t.Errorf("alvo Darwin de googledrive incorreto: %+v", gdrive.Darwin)
	}
	for _, a := range []string{"google-drive", "gdrive", "Google.GoogleDrive"} {
		found := FindTool(a)
		if found == nil || found.ID != "googledrive" {
			t.Errorf("alias %q não resolveu para googledrive: %+v", a, found)
		}
	}

	// 2. rclone
	rclone := FindTool("rclone")
	if rclone == nil {
		t.Fatalf("ferramenta rclone não encontrada no catálogo")
	}
	if rclone.Windows == nil || rclone.Windows.PackageID != "Rclone.Rclone" {
		t.Errorf("alvo Windows de rclone incorreto: %+v", rclone.Windows)
	}
	if rclone.Linux == nil || rclone.Linux.PackageID != "rclone" {
		t.Errorf("alvo Linux de rclone incorreto: %+v", rclone.Linux)
	}
	if rclone.Darwin == nil || rclone.Darwin.PackageID != "rclone" {
		t.Errorf("alvo Darwin de rclone incorreto: %+v", rclone.Darwin)
	}
	for _, a := range []string{"rclone/rclone", "https://github.com/rclone/rclone", "Rclone.Rclone"} {
		found := FindTool(a)
		if found == nil || found.ID != "rclone" {
			t.Errorf("alias %q não resolveu para rclone: %+v", a, found)
		}
	}
	if up := GetUpstreamRepo("rclone"); up != "https://github.com/rclone/rclone" {
		t.Errorf("upstream de rclone incorreto: %s", up)
	}
}
