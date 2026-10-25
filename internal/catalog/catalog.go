package catalog

import (
	"fmt"
	"runtime"
	"strings"
)

type TargetType string

const (
	TypeWinget  TargetType = "winget"
	TypeNative  TargetType = "native"
	TypeNpm     TargetType = "npm"
	TypePipx    TargetType = "pipx"
	TypeCargo   TargetType = "cargo"
	TypeSkill   TargetType = "skill"
	TypeFlatpak TargetType = "flatpak"
	TypeScript  TargetType = "script"
	TypeDirect  TargetType = "direct"
	TypeBrew    TargetType = "brew"
)

type Target struct {
	Type      TargetType `json:"type"`
	PackageID string     `json:"package_id"`
	Binary    string     `json:"binary"`
	SHA256    string     `json:"sha256,omitempty"`
}

type Tool struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	Description  string   `json:"description"`
	Dependencies []string `json:"dependencies,omitempty"`
	Windows      *Target  `json:"windows,omitempty"`
	Linux        *Target  `json:"linux,omitempty"`
	Darwin       *Target  `json:"darwin,omitempty"`
}

type Suite struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tools       []string `json:"tools"`
}

func (t Tool) TargetForCurrentOS() *Target {
	switch runtime.GOOS {
	case "windows":
		return t.Windows
	case "darwin":
		if t.Darwin != nil {
			return t.Darwin
		}
		// Fallback para ferramentas agnósticas (npm, pipx, cargo, git skills)
		if t.Linux != nil {
			switch t.Linux.Type {
			case TypeNpm, TypePipx, TypeCargo, TypeSkill:
				return t.Linux
			}
		}
		return nil
	default:
		return t.Linux
	}
}

func (t Tool) IsSupportedOnCurrentOS() bool {
	return t.TargetForCurrentOS() != nil
}

var packages = map[string][]string{
	"vibecode": {
		"aider", "repomix", "astgrep", "ollama", "ripgrep", "jq", "llm", "goose", "claudecode", "opencode", "ohmypi", "9router", "omniroute", "hermes", "orca",
	},
	"mcp": {
		"mcpm", "mcpinspector", "supergateway", "codebase_memory", "repomix", "context7", "rea", "awsagenttoolkit",
	},
	"agentstack": {
		"langgraph", "crewai", "ax", "bubblewrap", "promptfoo", "phoenix", "otelcol", "podman", "ecc", "paperclip", "hyperframes", "orca", "openmausbot", "openshell",
	},
	"skills": {
		"superpowers", "impeccable", "tasteskill", "emilkowalski", "archify", "opencodereview", "securityaudit", "contextmode", "ponytail", "reverseskill", "pocockskills", "diagramdesign", "ihaveadhd", "addyosmani", "humanizer", "openmaic", "openseo", "linkedin", "universalmodder", "androidskills", "hallmark", "awsagenttoolkit",
	},
	"dev": {
		"git", "gh", "fzf", "btop", "nodejs", "golang", "temurin", "docker", "vscode", "neovim", "waveterm", "ghostty", "jj", "uv", "rustup", "bat",
	},
	"toolchain": {
		"gcc", "make", "gdb", "cmake",
	},
	"termstack": {
		"waveterm", "ghostty", "bat", "fzf", "zsh", "ohmyzsh",
	},
	"flatpakfull": {
		"flatpak", "flathub",
	},
}

var toolAliases = map[string]string{
	"diagram-design":                "diagramdesign",
	"cathrynlavery/diagram-design":  "diagramdesign",
	"rea-agents":                    "rea",
	"morluto/rea":                   "rea",
	"decolua/9router":               "9router",
	"diegosouzapw/omniroute":        "omniroute",
	"heygen-com/hyperframes":        "hyperframes",
	"universal-modder":              "universalmodder",
	"rehan-remade/universal-modder": "universalmodder",
	"nousresearch/hermes":           "hermes",
	"hermes-agent":                  "hermes",
	"stablyai/orca":                 "orca",
	"stably-orca":                   "orca",
	"stablyai":                      "orca",
	"android/skills":                "androidskills",
	"android-skills":                "androidskills",
	"android":                       "androidskills",
	"paperclipai/paperclip":         "paperclip",
	"paperclipai":                   "paperclip",
	"milind-soni/openmausbot":       "openmausbot",
	"mausbot":                       "openmausbot",
	"opengrokbot":                   "openmausbot",
	"nvidia/openshell":              "openshell",
	"open-shell":                    "openshell",
	"nvidia-openshell":              "openshell",
	"nutlope/hallmark":              "hallmark",
	"hallmark-skill":                "hallmark",
	"aws/agent-toolkit-for-aws":     "awsagenttoolkit",
	"agent-toolkit-for-aws":         "awsagenttoolkit",
	"aws-agent-toolkit":             "awsagenttoolkit",
	"android-studio":                "androidstudio",
	"google/android-studio":         "androidstudio",
	"ibotpeaches/apktool":           "apktool",
	"iBotPeaches/Apktool":           "apktool",
	"x64dbg/x64dbg":                 "x64dbg",
	"x64-dbg":                       "x64dbg",
	"aider-ai/aider":                "aider",
	"ollama/ollama":                 "ollama",
	"anthropics/claude-code":        "claudecode",
	"block/goose":                   "goose",
	"ast-grep/ast-grep":             "astgrep",
	"yamadashy/repomix":             "repomix",
	"aristocratos/btop":             "btop",
	"sharkdp/bat":                   "bat",
	"BurntSushi/ripgrep":            "ripgrep",
	"junegunn/fzf":                  "fzf",
	"jqlang/jq":                     "jq",
	"simonw/llm":                    "llm",
	"promptfoo/promptfoo":           "promptfoo",
	"Arize-ai/phoenix":              "phoenix",
	"GoogleChromeLabs/bubblewrap":   "bubblewrap",
	"pathwaycom/mcpm":               "mcpm",
	"superinterface/supergateway":   "supergateway",
	"upstash/context7":              "context7",
	"codebase-memory/codebase-memory": "codebase_memory",
	"modelcontextprotocol/inspector": "mcpinspector",
	"crewAIInc/crewAI":              "crewai",
	"langchain-ai/langgraph":        "langgraph",
}

var catalogList = []Tool{
	{
		ID: "9router", Name: "9Router", Category: "Agentes", Description: "Proxy e gateway de IA local para ferramentas de código",
		Windows: &Target{Type: TypeNpm, PackageID: "9router", Binary: "9router.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "9router", Binary: "9router"},
	},
	{
		ID: "addyosmani", Name: "AddyOsmani Skills", Category: "Skills", Description: "Skills de arquitetura de software e engenharia",
		Windows: &Target{Type: TypeDirect, PackageID: "addyosmani/agent-skills", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "addyosmani/agent-skills", Binary: "SKILL.md"},
	},
	{
		ID: "androidskills", Name: "Android Skills", Category: "Skills", Description: "Skills oficiais do Google para desenvolvimento Android com IA",
		Windows: &Target{Type: TypeDirect, PackageID: "android/skills", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "android/skills", Binary: "SKILL.md"},
	},
	{
		ID: "androidstudio", Name: "Android Studio", Category: "Editores", Description: "IDE oficial do Google para desenvolvimento Android",
		Windows: &Target{Type: TypeWinget, PackageID: "Google.AndroidStudio", Binary: "studio64.exe"},
		Linux:   &Target{Type: TypeFlatpak, PackageID: "com.google.AndroidStudio", Binary: "androidstudio"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "android-studio", Binary: "studio"},
	},
	{
		ID: "apktool", Name: "Apktool", Category: "Análise", Description: "Ferramenta de engenharia reversa para descompilar e reconstruir APKs Android",
		Dependencies: []string{"temurin"},
		Windows: &Target{Type: TypeDirect, PackageID: "iBotPeaches/Apktool", Binary: "apktool.bat"},
		Linux:   &Target{Type: TypeNative, PackageID: "apktool", Binary: "apktool"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "apktool", Binary: "apktool"},
	},
	{
		ID: "agy", Name: "Antigravity CLI", Category: "Agentes", Description: "Agente do Google no terminal (binário agy)",
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "agy", Binary: "agy"},
	},
	{
		ID: "aider", Name: "Aider", Category: "Agentes", Description: "Programação em par com IA no terminal",
		Windows: &Target{Type: TypePipx, PackageID: "aider-chat", Binary: "aider.exe"},
		Linux:   &Target{Type: TypePipx, PackageID: "aider-chat", Binary: "aider"},
	},
	{
		ID: "antigravity", Name: "Antigravity IDE", Category: "Editores", Description: "IDE agent-first do Google",
		Windows: &Target{Type: TypeWinget, PackageID: "Google.Antigravity", Binary: "antigravity.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "antigravity", Binary: "antigravity"},
	},
	{
		ID: "archify", Name: "Archify", Category: "Skills", Description: "Skill de diagramas de arquitetura",
		Windows: &Target{Type: TypeDirect, PackageID: "tt-a1i/archify", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "tt-a1i/archify", Binary: "SKILL.md"},
	},
	{
		ID: "astgrep", Name: "ast-grep", Category: "Agentes", Description: "Busca e reescrita de código baseada em AST",
		Windows: &Target{Type: TypeNpm, PackageID: "@ast-grep/cli", Binary: "ast-grep.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "@ast-grep/cli", Binary: "astgrep"},
	},
	{
		ID: "atkinson", Name: "Atkinson Hyperlegible Next", Category: "Fontes", Description: "Fonte legível do Braille Institute",
		Windows: &Target{Type: TypeDirect, PackageID: "googlefonts/atkinson-hyperlegible-next", Binary: "Atkinson.ttf"},
		Linux:   nil,
	},
	{
		ID: "ax", Name: "AX", Category: "Agentes", Description: "Orquestrador de agentes do Google (go install)",
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "ax", Binary: "ax"},
	},
	{
		ID: "awsagenttoolkit", Name: "Agent Toolkit for AWS", Category: "Skills", Description: "Servidores MCP, skills e plugins oficiais da AWS para agentes de IA",
		Windows: &Target{Type: TypeDirect, PackageID: "aws/agent-toolkit-for-aws", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "aws/agent-toolkit-for-aws", Binary: "SKILL.md"},
		Darwin:  &Target{Type: TypeSkill, PackageID: "aws/agent-toolkit-for-aws", Binary: "SKILL.md"},
	},
	{
		ID: "bat", Name: "bat", Category: "Terminais", Description: "cat com syntax highlighting",
		Windows: &Target{Type: TypeWinget, PackageID: "sharkdp.bat", Binary: "bat.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "bat", Binary: "bat"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "bat", Binary: "bat"},
	},
	{
		ID: "binaryninja", Name: "Binary Ninja Free", Category: "Análise", Description: "Descompilador e analisador binário moderno",
		Windows: &Target{Type: TypeWinget, PackageID: "Vector35.BinaryNinja", Binary: "binaryninja.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "binaryninja", Binary: "binaryninja"},
	},
	{
		ID: "bottles", Name: "Bottles", Category: "Desktop", Description: "Apps Windows via Wine (flatpak)",
		Windows: nil,
		Linux:   &Target{Type: TypeFlatpak, PackageID: "com.usebottles.bottles", Binary: "bottles"},
	},
	{
		ID: "branchless", Name: "Git Branchless", Category: "Versionamento", Description: "Extensão Git: stacked diffs, undo",
		Windows: &Target{Type: TypeCargo, PackageID: "git-branchless", Binary: "git-branchless.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "git-branchless", Binary: "branchless"},
	},
	{
		ID: "browseruse", Name: "Browser Use", Category: "Agentes", Description: "Agente IA que opera o navegador",
		Windows: &Target{Type: TypePipx, PackageID: "browser-use", Binary: "browser-use.exe"},
		Linux:   &Target{Type: TypePipx, PackageID: "browser-use", Binary: "browser-use"},
	},
	{
		ID: "btop", Name: "Btop", Category: "Infraestrutura", Description: "Monitor de recursos no terminal",
		Windows: &Target{Type: TypeWinget, PackageID: "aristocratos.btop4win", Binary: "btop.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "btop", Binary: "btop"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "btop", Binary: "btop"},
	},
	{
		ID: "bubblewrap", Name: "Bubblewrap", Category: "Análise", Description: "Sandbox leve e sem privilégios para execução isolada",
		Windows: nil,
		Linux:   &Target{Type: TypeNative, PackageID: "bubblewrap", Binary: "bubblewrap"},
	},
	{
		ID: "cherrystudio", Name: "Cherry Studio", Category: "Agentes", Description: "Chat IA desktop multiplataforma",
		Windows: &Target{Type: TypeWinget, PackageID: "Kangfenfei.CherryStudio", Binary: "Cherry Studio.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "cherrystudio", Binary: "cherrystudio"},
	},
	{
		ID: "chromium", Name: "Chromium", Category: "Desktop", Description: "Navegador web open-source",
		Windows: &Target{Type: TypeWinget, PackageID: "Hibbiki.Chromium", Binary: "chrome.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "chromium", Binary: "chromium"},
	},
	{
		ID: "clamav", Name: "ClamAV", Category: "Análise", Description: "Motor antivírus open-source",
		Windows: &Target{Type: TypeWinget, PackageID: "Cisco.ClamAV", Binary: "clamscan.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "clamav", Binary: "clamav"},
	},
	{
		ID: "clamtk", Name: "ClamTk", Category: "Análise", Description: "Interface gráfica do ClamAV",
		Windows: nil,
		Linux:   &Target{Type: TypeNative, PackageID: "clamtk", Binary: "clamtk"},
	},
	{
		ID: "claudecode", Name: "Claude Code", Category: "Agentes", Description: "Agente de código CLI da Anthropic",
		Windows: &Target{Type: TypeNpm, PackageID: "@anthropic-ai/claude-code", Binary: "claude.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "@anthropic-ai/claude-code", Binary: "claude"},
	},
	{
		ID: "cmake", Name: "CMake", Category: "Infraestrutura", Description: "Build multiplataforma",
		Windows: &Target{Type: TypeWinget, PackageID: "Kitware.CMake", Binary: "cmake.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "cmake", Binary: "cmake"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "cmake", Binary: "cmake"},
	},
	{
		ID: "codebase_memory", Name: "Codebase Memory", Category: "Agentes", Description: "Grafo persistente de inteligência MCP",
		Windows: &Target{Type: TypeNpm, PackageID: "codebase-memory-mcp", Binary: "codebase-memory-mcp.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "codebase-memory-mcp", Binary: "codebase_memory"},
	},
	{
		ID: "codeql", Name: "CodeQL", Category: "Análise", Description: "Análise semântica profunda de código do GitHub",
		Windows: &Target{Type: TypeWinget, PackageID: "GitHub.CodeQL", Binary: "codeql.cmd"},
		Linux:   &Target{Type: TypeScript, PackageID: "codeql", Binary: "codeql"},
	},
	{
		ID: "codex", Name: "Codex CLI", Category: "Agentes", Description: "Agente de código da OpenAI",
		Windows: &Target{Type: TypeNpm, PackageID: "@openai/codex", Binary: "codex.cmd"},
		Linux:   &Target{Type: TypeScript, PackageID: "codex", Binary: "codex"},
	},
	{
		ID: "comfyui", Name: "ComfyUI", Category: "Agentes", Description: "Engine de nós para geração de imagens",
		Windows: &Target{Type: TypePipx, PackageID: "comfy-cli", Binary: "comfy.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "python3", Binary: "comfyui"},
	},
	{
		ID: "context7", Name: "Context7", Category: "Agentes", Description: "Docs atualizadas para LLMs via MCP",
		Windows: &Target{Type: TypeNpm, PackageID: "@upstash/context7", Binary: "ctx7.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "ctx7", Binary: "context7"},
	},
	{
		ID: "contextmode", Name: "Context Mode", Category: "Skills", Description: "Otimização de context window para agentes",
		Windows: &Target{Type: TypeDirect, PackageID: "mksglu/context-mode", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "mksglu/context-mode", Binary: "SKILL.md"},
	},
	{
		ID: "crewai", Name: "CrewAI", Category: "Agentes", Description: "Orquestração multi-agentes",
		Windows: &Target{Type: TypePipx, PackageID: "crewai", Binary: "crewai.exe"},
		Linux:   &Target{Type: TypePipx, PackageID: "crewai", Binary: "crewai"},
	},
	{
		ID: "danger", Name: "Danger", Category: "Análise", Description: "Automação de regras de code review",
		Windows: &Target{Type: TypeNpm, PackageID: "danger", Binary: "danger.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "danger", Binary: "danger"},
	},
	{
		ID: "dbeaver", Name: "DBeaver Community", Category: "Desktop", Description: "IDE universal de banco de dados",
		Windows: &Target{Type: TypeWinget, PackageID: "dbeaver.dbeaver", Binary: "dbeaver.exe"},
		Linux:   &Target{Type: TypeFlatpak, PackageID: "io.dbeaver.DBeaverCommunity", Binary: "dbeaver"},
	},
	{
		ID: "diagramdesign", Name: "Diagram Design", Category: "Skills", Description: "Skill de diagramação para agentes",
		Windows: &Target{Type: TypeDirect, PackageID: "cathrynlavery/diagram-design", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "cathrynlavery/diagram-design", Binary: "SKILL.md"},
	},
	{
		ID: "discord", Name: "Discord", Category: "Desktop", Description: "Voz, vídeo e chat de comunidades",
		Windows: &Target{Type: TypeWinget, PackageID: "Discord.Discord", Binary: "Discord.exe"},
		Linux:   &Target{Type: TypeFlatpak, PackageID: "com.discordapp.Discord", Binary: "Discord"},
	},
	{
		ID: "docker", Name: "Docker Engine", Category: "Infraestrutura", Description: "Engine e CLI de containers Docker e Compose",
		Windows: &Target{Type: TypeWinget, PackageID: "Docker.DockerDesktop", Binary: "docker.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "docker", Binary: "docker"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "docker", Binary: "docker"},
	},
	{
		ID: "dockerdesktop", Name: "Docker Desktop", Category: "Desktop", Description: "Interface gráfica e dashboard oficial do Docker (~440MB)",
		Windows: &Target{Type: TypeWinget, PackageID: "Docker.DockerDesktop", Binary: "Docker Desktop.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "dockerdesktop", Binary: "dockerdesktop"},
	},
	{
		ID: "dsh", Name: "DeepSeek Harness", Category: "Agentes", Description: "Agente open-source da DeepSeek",
		Windows: &Target{Type: TypeNpm, PackageID: "@deepseek-ai/dsh", Binary: "dsh.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "@deepseek-ai/dsh", Binary: "dsh"},
	},
	{
		ID: "ecc", Name: "Everything Claude Code", Category: "Agentes", Description: "Harness OS e workflows para agentes",
		Windows: &Target{Type: TypeNpm, PackageID: "ecc-universal", Binary: "ecc.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "ecc-universal", Binary: "ecc"},
	},
	{
		ID: "effect", Name: "Effect App", Category: "Infraestrutura", Description: "Scaffolding oficial de projetos Effect TypeScript",
		Windows: &Target{Type: TypeNpm, PackageID: "create-effect-app", Binary: "create-effect-app.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "create-effect-app", Binary: "create-effect-app"},
	},
	{
		ID: "emilkowalski", Name: "Emilkowalski Skills", Category: "Skills", Description: "Skills de animação e microinterações",
		Windows: &Target{Type: TypeDirect, PackageID: "emilkowalski/skills", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "emilkowalski/skills", Binary: "SKILL.md"},
	},
	{
		ID: "extensionmanager", Name: "Extension Manager", Category: "Infraestrutura", Description: "Navega e instala extensões (flatpak)",
		Windows: nil,
		Linux:   &Target{Type: TypeFlatpak, PackageID: "com.mattjakeman.ExtensionManager", Binary: "extensionmanager"},
	},
	{
		ID: "ffmpeg", Name: "FFmpeg", Category: "Infraestrutura", Description: "Multimídia (conversão, codecs)",
		Windows: &Target{Type: TypeWinget, PackageID: "Gyan.FFmpeg", Binary: "ffmpeg.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "ffmpeg", Binary: "ffmpeg"},
	},
	{
		ID: "firecrawl", Name: "Firecrawl CLI", Category: "Infraestrutura", Description: "Scraping web via CLI",
		Windows: &Target{Type: TypeNpm, PackageID: "@mendable/firecrawl-cli", Binary: "firecrawl.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "firecrawl-cli", Binary: "firecrawl"},
	},
	{
		ID: "flathub", Name: "Flathub", Category: "Infraestrutura", Description: "Repositório oficial de flatpaks",
		Dependencies: []string{"flatpak"},
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "flathub", Binary: "flathub"},
	},
	{
		ID: "flatpak", Name: "Flatpak", Category: "Infraestrutura", Description: "Empacotamento universal de apps",
		Windows: nil,
		Linux:   &Target{Type: TypeNative, PackageID: "flatpak", Binary: "flatpak"},
	},
	{
		ID: "freellmapi", Name: "FreeLLMAPI", Category: "Agentes", Description: "Gateway de IA com provedores gratuitos",
		Windows: &Target{Type: TypeNpm, PackageID: "freellmapi", Binary: "freellmapi.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "freellmapi", Binary: "freellmapi"},
	},
	{
		ID: "freetoken", Name: "FreeToken", Category: "Agentes", Description: "Servidor local de modelos (pipx)",
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "freetoken", Binary: "freetoken"},
	},
	{
		ID: "fzf", Name: "fzf", Category: "Infraestrutura", Description: "Busca difusa no terminal",
		Windows: &Target{Type: TypeWinget, PackageID: "junegunn.fzf", Binary: "fzf.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "fzf", Binary: "fzf"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "fzf", Binary: "fzf"},
	},
	{
		ID: "gcc", Name: "GCC (w64devkit)", Category: "Infraestrutura", Description: "Compilador C/C++ para Windows",
		Windows: &Target{Type: TypeWinget, PackageID: "skeeto.w64devkit", Binary: "gcc.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "gcc", Binary: "gcc"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "gcc", Binary: "gcc"},
	},
	{
		ID: "gdb", Name: "GDB", Category: "Infraestrutura", Description: "Debugger C/C++",
		Windows: &Target{Type: TypeWinget, PackageID: "skeeto.w64devkit", Binary: "gdb.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "gdb", Binary: "gdb"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "gdb", Binary: "gdb"},
	},
	{
		ID: "geist", Name: "Geist", Category: "Fontes", Description: "Fonte moderna da Vercel (Sans + Mono)",
		Windows: &Target{Type: TypeWinget, PackageID: "Vercel.Geist", Binary: "Geist.ttf"},
		Linux:   &Target{Type: TypeScript, PackageID: "geist", Binary: "geist"},
	},
	{
		ID: "gethomepage", Name: "Homepage", Category: "Desktop", Description: "Dashboard de serviços (docker)",
		Dependencies: []string{"docker"},
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "gethomepage", Binary: "gethomepage"},
	},
	{
		ID: "gh", Name: "GitHub CLI", Category: "Versionamento", Description: "CLI oficial do GitHub",
		Windows: &Target{Type: TypeWinget, PackageID: "GitHub.cli", Binary: "gh.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "gh", Binary: "gh"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "gh", Binary: "gh"},
	},
	{
		ID: "ghidra", Name: "Ghidra", Category: "Análise", Description: "Framework de engenharia reversa da NSA",
		Dependencies: []string{"temurin"},
		Windows: &Target{Type: TypeWinget, PackageID: "NationalSecurityAgency.Ghidra", Binary: "ghidraRun.bat"},
		Linux:   &Target{Type: TypeScript, PackageID: "ghidra", Binary: "ghidra"},
	},
	{
		ID: "ghostty", Name: "Ghostty", Category: "Terminais", Description: "Terminal rápido com aceleração GPU",
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "ghostty", Binary: "ghostty"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "ghostty", Binary: "ghostty"},
	},
	{
		ID: "git", Name: "Git", Category: "Versionamento", Description: "Controle de versão distribuído",
		Windows: &Target{Type: TypeWinget, PackageID: "Git.Git", Binary: "git.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "git", Binary: "git"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "git", Binary: "git"},
	},
	{
		ID: "gnomeextensions", Name: "GNOME Extensions", Category: "Infraestrutura", Description: "Liga/desliga extensões do Shell",
		Windows: nil,
		Linux:   &Target{Type: TypeNative, PackageID: "gnome-extensions-app", Binary: "gnomeextensions"},
	},
	{
		ID: "gnomesoftware", Name: "GNOME Software", Category: "Infraestrutura", Description: "Loja de apps do GNOME",
		Windows: nil,
		Linux:   &Target{Type: TypeNative, PackageID: "gnome-software", Binary: "gnomesoftware"},
	},
	{
		ID: "goaccess", Name: "GoAccess", Category: "Infraestrutura", Description: "Analisador de logs em tempo real",
		Windows: nil,
		Linux:   &Target{Type: TypeNative, PackageID: "goaccess", Binary: "goaccess"},
	},
	{
		ID: "golang", Name: "Go", Category: "Infraestrutura", Description: "Toolchain Go da Google",
		Windows: &Target{Type: TypeWinget, PackageID: "GoLang.Go", Binary: "go.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "golang", Binary: "golang"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "go", Binary: "go"},
	},
	{
		ID: "goose", Name: "Goose", Category: "Agentes", Description: "Agente autônomo de código da Block/AAIF",
		Windows: &Target{Type: TypeDirect, PackageID: "aaif-goose/goose", Binary: "goose.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "goose", Binary: "goose"},
	},
	{
		ID: "grafana", Name: "Grafana", Category: "Observabilidade", Description: "Dashboards e visualização de métricas",
		Windows: &Target{Type: TypeWinget, PackageID: "GrafanaLabs.Grafana", Binary: "grafana-server.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "grafana", Binary: "grafana"},
	},
	{
		ID: "hallmark", Name: "Hallmark", Category: "Skills", Description: "Design skill anti-AI-slop para Claude Code, OpenCode e Codex",
		Windows: &Target{Type: TypeDirect, PackageID: "Nutlope/hallmark", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "Nutlope/hallmark", Binary: "SKILL.md"},
		Darwin:  &Target{Type: TypeSkill, PackageID: "Nutlope/hallmark", Binary: "SKILL.md"},
	},
	{
		ID: "hermes", Name: "Hermes Agent", Category: "Agentes", Description: "Agente auto-evolutivo da Nous Research (TUI, gateway, skills e cron)",
		Windows: &Target{Type: TypeNpm, PackageID: "hermes-agent", Binary: "hermes.cmd"},
		Linux:   &Target{Type: TypeScript, PackageID: "hermes", Binary: "hermes"},
	},
	{
		ID: "heroic", Name: "Heroic Games Launcher", Category: "Desktop", Description: "Launcher open-source para Epic, GOG e Amazon",
		Windows: &Target{Type: TypeWinget, PackageID: "HeroicGamesLauncher.HeroicGamesLauncher", Binary: "Heroic.exe"},
		Linux:   &Target{Type: TypeFlatpak, PackageID: "com.heroicgameslauncher.hgl", Binary: "heroic"},
	},
	{
		ID: "humanizer", Name: "Humanizer", Category: "Skills", Description: "Remove marcas de texto excessivamente robótico",
		Windows: &Target{Type: TypeDirect, PackageID: "blader/humanizer", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "blader/humanizer", Binary: "SKILL.md"},
	},
	{
		ID: "hyperframes", Name: "HyperFrames", Category: "Agentes", Description: "Framework open-source de vídeo programático para agentes IA",
		Windows: &Target{Type: TypeNpm, PackageID: "hyperframes", Binary: "hyperframes.cmd"},
		Linux:   &Target{Type: TypeSkill, PackageID: "heygen-com/hyperframes", Binary: "SKILL.md"},
	},
	{
		ID: "ihaveadhd", Name: "I Have ADHD", Category: "Skills", Description: "Assistente de foco para desenvolvimento",
		Windows: &Target{Type: TypeDirect, PackageID: "ayghri/i-have-adhd", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "ayghri/i-have-adhd", Binary: "SKILL.md"},
	},
	{
		ID: "impeccable", Name: "Impeccable", Category: "Skills", Description: "Skill de frontend/design para agentes",
		Windows: &Target{Type: TypeDirect, PackageID: "pbakaus/impeccable", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "pbakaus/impeccable", Binary: "SKILL.md"},
	},
	{
		ID: "invokeai", Name: "InvokeAI", Category: "Agentes", Description: "Geração profissional de imagens por IA",
		Windows: &Target{Type: TypePipx, PackageID: "invokeai", Binary: "invokeai.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "invokeai", Binary: "invokeai"},
	},
	{
		ID: "jetbrainsmono", Name: "JetBrains Mono", Category: "Fontes", Description: "Fonte monoespaçada da JetBrains",
		Windows: &Target{Type: TypeWinget, PackageID: "JetBrains.JetBrainsMono", Binary: "JetBrainsMono.ttf"},
		Linux:   &Target{Type: TypeScript, PackageID: "jetbrainsmono", Binary: "jetbrainsmono"},
	},
	{
		ID: "jetbrainstoolbox", Name: "JetBrains Toolbox", Category: "Editores", Description: "Gerenciador de IDEs da JetBrains",
		Windows: &Target{Type: TypeWinget, PackageID: "JetBrains.Toolbox", Binary: "jetbrains-toolbox.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "jetbrainstoolbox", Binary: "jetbrainstoolbox"},
	},
	{
		ID: "jj", Name: "Jujutsu", Category: "Versionamento", Description: "VCS compatível com Git, fluxo simples",
		Windows: &Target{Type: TypeWinget, PackageID: "martinvonz.jj", Binary: "jj.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "jj", Binary: "jj"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "jj", Binary: "jj"},
	},
	{
		ID: "jq", Name: "jq", Category: "Infraestrutura", Description: "Processador JSON de linha de comando",
		Windows: &Target{Type: TypeWinget, PackageID: "jqlang.jq", Binary: "jq.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "jq", Binary: "jq"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "jq", Binary: "jq"},
	},
	{
		ID: "kdenlive", Name: "Kdenlive", Category: "Desktop", Description: "Editor de vídeo poderoso e open-source",
		Windows: &Target{Type: TypeWinget, PackageID: "KDE.Kdenlive", Binary: "kdenlive.exe"},
		Linux:   &Target{Type: TypeFlatpak, PackageID: "org.kde.kdenlive", Binary: "kdenlive"},
	},
	{
		ID: "keep", Name: "Keep", Category: "Observabilidade", Description: "Gestão de alertas (Docker)",
		Dependencies: []string{"docker"},
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "keep", Binary: "keep"},
	},
	{
		ID: "langgraph", Name: "LangGraph CLI", Category: "Agentes", Description: "Framework de grafos de agentes",
		Windows: &Target{Type: TypePipx, PackageID: "langgraph-cli", Binary: "langgraph.exe"},
		Linux:   &Target{Type: TypePipx, PackageID: "langgraph-cli", Binary: "langgraph"},
	},
	{
		ID: "linkedin", Name: "LinkedIn Skills", Category: "Skills", Description: "Skills de escrita técnica e publicação",
		Windows: &Target{Type: TypeDirect, PackageID: "sergebulaev/linkedin-skills", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "sergebulaev/linkedin-skills", Binary: "SKILL.md"},
	},
	{
		ID: "llm", Name: "LLM CLI", Category: "Agentes", Description: "CLI universal para LLMs (Simon Willison)",
		Windows: &Target{Type: TypePipx, PackageID: "llm", Binary: "llm.exe"},
		Linux:   &Target{Type: TypePipx, PackageID: "llm", Binary: "llm"},
	},
	{
		ID: "lmstudio", Name: "LM Studio", Category: "Agentes", Description: "Interface gráfica para LLMs locais",
		Windows: &Target{Type: TypeWinget, PackageID: "ElementLabs.LMStudio", Binary: "LM Studio.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "lmstudio", Binary: "lmstudio"},
	},
	{
		ID: "localplayer", Name: "Local Player", Category: "Desktop", Description: "Player local (AppImage oficial)",
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "localplayer", Binary: "localplayer"},
	},
	{
		ID: "make", Name: "Make", Category: "Infraestrutura", Description: "Automação de build",
		Windows: &Target{Type: TypeWinget, PackageID: "GnuWin32.Make", Binary: "make.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "make", Binary: "make"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "make", Binary: "make"},
	},
	{
		ID: "margem", Name: "Margem", Category: "Desktop", Description: "Leitor desktop tipográfico (AppImage oficial)",
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "margem", Binary: "margem"},
	},
	{
		ID: "mcpinspector", Name: "MCP Inspector", Category: "Agentes", Description: "Inspetor oficial do Model Context Protocol",
		Windows: &Target{Type: TypeNpm, PackageID: "@modelcontextprotocol/inspector", Binary: "mcp-inspector.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "@modelcontextprotocol/inspector", Binary: "mcpinspector"},
	},
	{
		ID: "mcpm", Name: "MCPM", Category: "Agentes", Description: "Gerenciador de servidores MCP",
		Windows: &Target{Type: TypeNpm, PackageID: "mcpm", Binary: "mcpm.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "mcpm", Binary: "mcpm"},
	},
	{
		ID: "moonlight", Name: "Moonlight", Category: "Desktop", Description: "Streaming de jogos de alta performance",
		Windows: &Target{Type: TypeWinget, PackageID: "MoonlightGameStreamingProject.Moonlight", Binary: "Moonlight.exe"},
		Linux:   &Target{Type: TypeFlatpak, PackageID: "com.moonlight_stream.Moonlight", Binary: "moonlight"},
	},
	{
		ID: "neovim", Name: "Neovim", Category: "Editores", Description: "Editor modal moderno",
		Windows: &Target{Type: TypeWinget, PackageID: "Neovim.Neovim", Binary: "nvim.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "neovim", Binary: "neovim"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "neovim", Binary: "nvim"},
	},
	{
		ID: "nodejs", Name: "Node.js LTS", Category: "Infraestrutura", Description: "Runtime JavaScript LTS",
		Windows: &Target{Type: TypeWinget, PackageID: "OpenJS.NodeJS.LTS", Binary: "node.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "nodejs", Binary: "nodejs"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "node", Binary: "node"},
	},
	{
		ID: "obs", Name: "OBS Studio", Category: "Desktop", Description: "Gravação de tela e streaming de vídeo",
		Windows: &Target{Type: TypeWinget, PackageID: "OBSProject.OBSStudio", Binary: "obs64.exe"},
		Linux:   &Target{Type: TypeFlatpak, PackageID: "com.obsproject.Studio", Binary: "obs"},
	},
	{
		ID: "obsidian", Name: "Obsidian", Category: "Desktop", Description: "Base de conhecimento em Markdown",
		Windows: &Target{Type: TypeWinget, PackageID: "Obsidian.Obsidian", Binary: "Obsidian.exe"},
		Linux:   &Target{Type: TypeFlatpak, PackageID: "md.obsidian.Obsidian", Binary: "obsidian"},
	},
	{
		ID: "ohmypi", Name: "Oh My Pi", Category: "Agentes", Description: "Agente de código com IDE embutida",
		Windows: &Target{Type: TypeDirect, PackageID: "ohmypi/omp", Binary: "omp.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "ohmypi", Binary: "ohmypi"},
	},
	{
		ID: "ohmyzsh", Name: "Oh My Zsh", Category: "Terminais", Description: "Framework Zsh + powerlevel10k + plugins fullstack",
		Dependencies: []string{"zsh"},
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "ohmyzsh", Binary: "ohmyzsh"},
	},
	{
		ID: "ollama", Name: "Ollama", Category: "Agentes", Description: "Servidor local de modelos de IA",
		Windows: &Target{Type: TypeWinget, PackageID: "Ollama.Ollama", Binary: "ollama.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "ollama", Binary: "ollama"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "ollama", Binary: "ollama"},
	},
	{
		ID: "omniroute", Name: "OmniRoute", Category: "Agentes", Description: "Gateway de IA local para múltiplos provedores",
		Windows: &Target{Type: TypeNpm, PackageID: "omniroute", Binary: "omniroute.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "omniroute", Binary: "omniroute"},
	},
	{
		ID: "opencode", Name: "OpenCode", Category: "Agentes", Description: "Agente de código open-source no terminal",
		Windows: &Target{Type: TypeNpm, PackageID: "opencode-ai", Binary: "opencode.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "opencode-ai", Binary: "opencode"},
	},
	{
		ID: "opencodereview", Name: "Open Code Review", Category: "Skills", Description: "Skills de code review (Alibaba)",
		Windows: &Target{Type: TypeDirect, PackageID: "alibaba/open-code-review", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "alibaba/open-code-review", Binary: "SKILL.md"},
	},
	{
		ID: "opengym", Name: "OpenGym", Category: "Desktop", Description: "Tracker de treino self-hosted (docker)",
		Dependencies: []string{"docker"},
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "alexpcosta/opengym", Binary: "opengym"},
	},
	{
		ID: "openhands", Name: "OpenHands", Category: "Agentes", Description: "Engenheiro de software com IA (sandbox)",
		Windows: nil,
		Linux:   &Target{Type: TypeNpm, PackageID: "@openhands/agent-canvas", Binary: "openhands"},
	},
	{
		ID: "openmaic", Name: "OpenMAIC", Category: "Skills", Description: "Skills de orquestração multi-agente educacional",
		Windows: &Target{Type: TypeDirect, PackageID: "THU-MAIC/OpenMAIC", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "THU-MAIC/OpenMAIC", Binary: "SKILL.md"},
	},
	{
		ID: "openmaicapp", Name: "OpenMAIC App", Category: "Desktop", Description: "Sala de aula multi-agente (docker; .env manual)",
		Dependencies: []string{"docker"},
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "THU-MAIC/OpenMAIC", Binary: "openmaicapp"},
	},
	{
		ID: "openmausbot", Name: "OpenMausBot", Category: "Agentes", Description: "Alternativa open-source ao Grok Bot / Manus com equipe de agentes IA e VMs",
		Windows: &Target{Type: TypeDirect, PackageID: "milind-soni/OpenMausBot", Binary: "OpenMausBot.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "openmausbot", Binary: "openmausbot"},
	},
	{
		ID: "openseo", Name: "Open SEO", Category: "Skills", Description: "Skills de SEO e análise competitiva",
		Windows: &Target{Type: TypeDirect, PackageID: "every-app/open-seo", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "every-app/open-seo", Binary: "SKILL.md"},
	},
	{
		ID: "openshell", Name: "OpenShell", Category: "Agentes", Description: "Runtime e sandbox seguro com governança para agentes de IA da NVIDIA",
		Windows: &Target{Type: TypePipx, PackageID: "openshell", Binary: "openshell.exe"},
		Linux:   &Target{Type: TypePipx, PackageID: "openshell", Binary: "openshell"},
		Darwin:  &Target{Type: TypePipx, PackageID: "openshell", Binary: "openshell"},
	},
	{
		ID: "openssh", Name: "OpenSSH", Category: "Infraestrutura", Description: "Cliente SSH para Windows",
		Windows: &Target{Type: TypeWinget, PackageID: "Microsoft.OpenSSH.Beta", Binary: "ssh.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "openssh-client", Binary: "openssh"},
	},
	{
		ID: "openstock", Name: "OpenStock", Category: "Desktop", Description: "Bolsa de valores self-hosted (docker; .env manual)",
		Dependencies: []string{"docker"},
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "Open-Dev-Society/OpenStock", Binary: "openstock"},
	},
	{
		ID: "openv0", Name: "OpenV0", Category: "Agentes", Description: "Gerador de UI com IA (legado, descontinuado)",
		Windows: nil,
		Linux:   &Target{Type: TypeNpm, PackageID: "openv0", Binary: "openv0"},
	},
	{
		ID: "openwebui", Name: "Open WebUI", Category: "Agentes", Description: "Interface web moderna para LLMs",
		Windows: &Target{Type: TypePipx, PackageID: "open-webui", Binary: "open-webui.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "openwebui", Binary: "openwebui"},
	},
	{
		ID: "orca", Name: "Orca", Category: "Agentes", Description: "Orquestrador e ambiente de trabalho para frotas de agentes IA (worktrees paralelos)",
		Windows: &Target{Type: TypeDirect, PackageID: "stablyai/orca", Binary: "orca.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "orca", Binary: "orca"},
	},
	{
		ID: "otelcol", Name: "OpenTelemetry Collector", Category: "Observabilidade", Description: "Coletor de métricas, traces e logs",
		Windows: &Target{Type: TypeWinget, PackageID: "OpenTelemetry.Collector", Binary: "otelcol.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "otelcol", Binary: "otelcol"},
	},
	{
		ID: "paperclip", Name: "Paperclip", Category: "Agentes", Description: "Orquestração e governança de times IA",
		Windows: &Target{Type: TypeNpm, PackageID: "paperclipai", Binary: "paperclip.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "paperclipai", Binary: "paperclip"},
	},
	{
		ID: "phoenix", Name: "Arize Phoenix", Category: "Observabilidade", Description: "Tracing OpenTelemetry para agentes IA",
		Windows: &Target{Type: TypePipx, PackageID: "arize-phoenix", Binary: "phoenix.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "phoenix", Binary: "phoenix"},
	},
	{
		ID: "playwright", Name: "Playwright", Category: "Infraestrutura", Description: "Automação de browsers",
		Windows: &Target{Type: TypeNpm, PackageID: "playwright", Binary: "playwright.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "playwright", Binary: "playwright"},
	},
	{
		ID: "pocockskills", Name: "Pocock Skills", Category: "Skills", Description: "Coleção de skills de TypeScript/Engenharia",
		Windows: &Target{Type: TypeDirect, PackageID: "mattpocock/skills", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "mattpocock/skills", Binary: "SKILL.md"},
	},
	{
		ID: "podman", Name: "Podman Desktop", Category: "Infraestrutura", Description: "Containers sem daemon para Windows",
		Windows: &Target{Type: TypeWinget, PackageID: "RedHat.Podman-Desktop", Binary: "podman.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "podman", Binary: "podman"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "podman", Binary: "podman"},
	},
	{
		ID: "ponytail", Name: "Ponytail", Category: "Skills", Description: "Regras anti-overengineering para agentes",
		Windows: &Target{Type: TypeDirect, PackageID: "DietrichGebert/ponytail", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "DietrichGebert/ponytail", Binary: "SKILL.md"},
	},
	{
		ID: "promptfoo", Name: "Promptfoo", Category: "Análise", Description: "Framework de testes e evals de LLMs",
		Windows: &Target{Type: TypeNpm, PackageID: "promptfoo", Binary: "promptfoo.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "promptfoo", Binary: "promptfoo"},
	},
	{
		ID: "protonge", Name: "Proton-GE", Category: "Desktop", Description: "Compatibilidade Windows no Steam",
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "protonge", Binary: "protonge"},
	},
	{
		ID: "rea", Name: "REA", Category: "Agentes", Description: "Toolkit e MCP server de engenharia reversa para agentes IA",
		Windows: &Target{Type: TypeNpm, PackageID: "rea-agents", Binary: "rea.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "rea-agents", Binary: "rea"},
	},
	{
		ID: "repomix", Name: "Repomix", Category: "Agentes", Description: "Empacotador de repositório em contexto de IA",
		Windows: &Target{Type: TypeNpm, PackageID: "repomix", Binary: "repomix.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "repomix", Binary: "repomix"},
	},
	{
		ID: "reverseskill", Name: "Reverse Skills", Category: "Skills", Description: "Coleção de skills CTF e reversing",
		Windows: &Target{Type: TypeDirect, PackageID: "zhaoxuya520/reverse-skill", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "zhaoxuya520/reverse-skill", Binary: "SKILL.md"},
	},
	{
		ID: "ripgrep", Name: "Ripgrep", Category: "Infraestrutura", Description: "Busca de texto ultrarrápida (rg)",
		Windows: &Target{Type: TypeWinget, PackageID: "BurntSushi.ripgrep.MSVC", Binary: "rg.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "ripgrep", Binary: "ripgrep"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "ripgrep", Binary: "rg"},
	},
	{
		ID: "robusta", Name: "Robusta CLI", Category: "Observabilidade", Description: "Troubleshooting Kubernetes com IA",
		Windows: &Target{Type: TypePipx, PackageID: "robusta-cli", Binary: "robusta.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "robusta", Binary: "robusta"},
	},
	{
		ID: "rustdesk", Name: "RustDesk", Category: "Desktop", Description: "Acesso e suporte a desktop remoto",
		Windows: &Target{Type: TypeWinget, PackageID: "RustDesk.RustDesk", Binary: "rustdesk.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "rustdesk", Binary: "rustdesk"},
	},
	{
		ID: "rustup", Name: "Rustup", Category: "Infraestrutura", Description: "Toolchain Rust (rustc/cargo)",
		Windows: &Target{Type: TypeWinget, PackageID: "Rustlang.Rustup", Binary: "rustup.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "rustup", Binary: "rustup"},
	},
	{
		ID: "securityaudit", Name: "Security Audit", Category: "Skills", Description: "Skill de auditoria de segurança",
		Windows: &Target{Type: TypeDirect, PackageID: "cloudflare/security-audit-skills", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "cloudflare/security-audit-skill", Binary: "SKILL.md"},
	},
	{
		ID: "semgrep", Name: "Semgrep", Category: "Análise", Description: "Scanner estático de código",
		Windows: &Target{Type: TypePipx, PackageID: "semgrep", Binary: "semgrep.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "semgrep", Binary: "semgrep"},
	},
	{
		ID: "sourcesans3", Name: "Source Sans 3", Category: "Fontes", Description: "Sans serif da Adobe para interfaces",
		Windows: &Target{Type: TypeDirect, PackageID: "adobe-fonts/source-sans", Binary: "SourceSans3.ttf"},
		Linux:   nil,
	},
	{
		ID: "sourceserif4", Name: "Source Serif 4", Category: "Fontes", Description: "Serifada da Adobe para leitura",
		Windows: &Target{Type: TypeDirect, PackageID: "adobe-fonts/source-serif", Binary: "SourceSerif4.ttf"},
		Linux:   nil,
	},
	{
		ID: "sqlitebrowser", Name: "DB Browser for SQLite", Category: "Editores", Description: "Navegador visual de bancos SQLite",
		Windows: &Target{Type: TypeWinget, PackageID: "DBBrowserForSQLite.DBBrowserForSQLite", Binary: "DB Browser for SQLite.exe"},
		Linux:   &Target{Type: TypeNative, PackageID: "sqlitebrowser", Binary: "sqlitebrowser"},
	},
	{
		ID: "stabilitymatrix", Name: "StabilityMatrix", Category: "Agentes", Description: "Gerenciador de Stable Diffusion",
		Windows: &Target{Type: TypeWinget, PackageID: "LykosAI.StabilityMatrix", Binary: "StabilityMatrix.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "stabilitymatrix", Binary: "stabilitymatrix"},
	},
	{
		ID: "supergateway", Name: "Supergateway", Category: "Agentes", Description: "Bridge MCP entre stdio e SSE / WebSockets",
		Windows: &Target{Type: TypeNpm, PackageID: "supergateway", Binary: "supergateway.cmd"},
		Linux:   &Target{Type: TypeNpm, PackageID: "supergateway", Binary: "supergateway"},
	},
	{
		ID: "superpowers", Name: "Superpowers", Category: "Skills", Description: "Metodologia de dev com skills (TDD, plans)",
		Windows: &Target{Type: TypeDirect, PackageID: "obra/superpowers", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "obra/superpowers", Binary: "SKILL.md"},
	},
	{
		ID: "tailscale", Name: "Tailscale", Category: "Infraestrutura", Description: "VPN mesh WireGuard",
		Windows: &Target{Type: TypeWinget, PackageID: "Tailscale.Tailscale", Binary: "tailscale.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "tailscale", Binary: "tailscale"},
	},
	{
		ID: "tasteskill", Name: "Taste Skill", Category: "Skills", Description: "Skills anti-slop de frontend para agentes",
		Windows: &Target{Type: TypeDirect, PackageID: "leonxlnx/taste-skill", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "leonxlnx/taste-skill", Binary: "SKILL.md"},
	},
	{
		ID: "telegram", Name: "Telegram Desktop", Category: "Desktop", Description: "Mensageiro rápido e seguro",
		Windows: &Target{Type: TypeWinget, PackageID: "Telegram.TelegramDesktop", Binary: "Telegram.exe"},
		Linux:   &Target{Type: TypeFlatpak, PackageID: "org.telegram.desktop", Binary: "telegram"},
	},
	{
		ID: "temurin", Name: "Temurin JDK 21", Category: "Infraestrutura", Description: "OpenJDK 21 da Adoptium",
		Windows: &Target{Type: TypeWinget, PackageID: "EclipseAdoptium.Temurin.21.JDK", Binary: "java.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "temurin", Binary: "temurin"},
	},
	{
		ID: "torbrowser", Name: "Tor Browser", Category: "Análise", Description: "Navegador focado em privacidade e anonimato",
		Windows: &Target{Type: TypeWinget, PackageID: "TorProject.TorBrowser", Binary: "firefox.exe"},
		Linux:   &Target{Type: TypeFlatpak, PackageID: "org.torproject.torbrowser-launcher", Binary: "torbrowser"},
	},
	{
		ID: "trivy", Name: "Trivy", Category: "Análise", Description: "Scanner de vulnerabilidades em repos e containers",
		Windows: &Target{Type: TypeWinget, PackageID: "AquaSecurity.Trivy", Binary: "trivy.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "trivy", Binary: "trivy"},
	},
	{
		ID: "tweaks", Name: "GNOME Tweaks", Category: "Infraestrutura", Description: "Ajustes finos do GNOME",
		Windows: nil,
		Linux:   &Target{Type: TypeNative, PackageID: "gnome-tweaks", Binary: "tweaks"},
	},
	{
		ID: "universalmodder", Name: "Universal Modder", Category: "Skills", Description: "Skills e MCP para agentes de IA realizarem modding em jogos de PC",
		Windows: &Target{Type: TypeDirect, PackageID: "rehan-remade/universal-modder", Binary: "SKILL.md"},
		Linux:   &Target{Type: TypeSkill, PackageID: "rehan-remade/universal-modder", Binary: "SKILL.md"},
	},
	{
		ID: "uv", Name: "uv", Category: "Infraestrutura", Description: "Gerenciador Python ultrarrápido",
		Windows: &Target{Type: TypeWinget, PackageID: "astral-sh.uv", Binary: "uv.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "uv", Binary: "uv"},
	},
	{
		ID: "vlc", Name: "VLC media player", Category: "Desktop", Description: "Reprodutor multimídia universal",
		Windows: &Target{Type: TypeWinget, PackageID: "VideoLAN.VLC", Binary: "vlc.exe"},
		Linux:   &Target{Type: TypeFlatpak, PackageID: "org.videolan.VLC", Binary: "vlc"},
	},
	{
		ID: "vscode", Name: "Visual Studio Code", Category: "Editores", Description: "Editor de código da Microsoft",
		Windows: &Target{Type: TypeWinget, PackageID: "Microsoft.VisualStudioCode", Binary: "code.cmd"},
		Linux:   &Target{Type: TypeScript, PackageID: "vscode", Binary: "vscode"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "visual-studio-code", Binary: "code"},
	},
	{
		ID: "warp", Name: "Warp", Category: "Terminais", Description: "Terminal moderno com IA",
		Windows: &Target{Type: TypeWinget, PackageID: "WarpDotDev.Warp", Binary: "warp.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "warp", Binary: "warp"},
	},
	{
		ID: "waveterm", Name: "Wave Terminal", Category: "Terminais", Description: "Terminal com IA e sessões persistentes",
		Windows: &Target{Type: TypeWinget, PackageID: "wavetermdev.waveterm", Binary: "waveterm.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "waveterm", Binary: "waveterm"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "waveterm", Binary: "waveterm"},
	},
	{
		ID: "waydroid", Name: "Waydroid", Category: "Infraestrutura", Description: "Android em container (init manual)",
		Windows: nil,
		Linux:   &Target{Type: TypeScript, PackageID: "waydroid", Binary: "waydroid"},
	},
	{
		ID: "x64dbg", Name: "x64dbg", Category: "Análise", Description: "Depurador x64/x32 open-source para engenharia reversa e análise de malware",
		Windows: &Target{Type: TypeWinget, PackageID: "x64dbg.x64dbg", Binary: "x64dbg.exe"},
		Linux:   nil,
	},
	{
		ID: "zed", Name: "Zed", Category: "Editores", Description: "Editor de alta performance com IA",
		Windows: &Target{Type: TypeWinget, PackageID: "ZedIndustries.Zed", Binary: "zed.exe"},
		Linux:   &Target{Type: TypeScript, PackageID: "zed", Binary: "zed"},
	},
	{
		ID: "zsh", Name: "Zsh", Category: "Terminais", Description: "Shell interativo (base do oh-my-zsh)",
		Windows: nil,
		Linux:   &Target{Type: TypeNative, PackageID: "zsh", Binary: "zsh"},
		Darwin:  &Target{Type: TypeBrew, PackageID: "zsh", Binary: "zsh"},
	},
}

func DefaultCatalog() []Tool {
	return catalogList
}

func SupportedCatalog() []Tool {
	var list []Tool
	for _, t := range catalogList {
		if t.IsSupportedOnCurrentOS() {
			list = append(list, t)
		}
	}
	return list
}

func FindTool(id string) *Tool {
	cleanID := strings.TrimSpace(id)
	cleanID = strings.TrimPrefix(cleanID, "https://github.com/")
	cleanID = strings.TrimPrefix(cleanID, "http://github.com/")
	cleanID = strings.TrimSuffix(cleanID, ".git")
	cleanID = strings.TrimSuffix(cleanID, "/")
	cleanLower := strings.ToLower(cleanID)

	if targetID, ok := toolAliases[cleanLower]; ok {
		cleanLower = targetID
	}

	for i := range catalogList {
		if strings.EqualFold(catalogList[i].ID, cleanLower) {
			return &catalogList[i]
		}
	}

	for i := range catalogList {
		target := catalogList[i].TargetForCurrentOS()
		if target != nil && (strings.EqualFold(target.PackageID, cleanID) || strings.EqualFold(target.PackageID, cleanLower)) {
			return &catalogList[i]
		}
	}

	norm := strings.ReplaceAll(strings.ReplaceAll(cleanLower, "-", ""), "_", "")
	for i := range catalogList {
		catNorm := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(catalogList[i].ID), "-", ""), "_", "")
		if catNorm == norm {
			return &catalogList[i]
		}
	}

	return nil
}

func Expand(args []string) ([]string, error) {
	var result []string
	seen := make(map[string]bool)

	for _, arg := range args {
		if strings.HasPrefix(arg, "@") {
			pkgName := strings.TrimPrefix(arg, "@")
			items, ok := packages[pkgName]
			if !ok {
				return nil, fmt.Errorf("pacote desconhecido: %s", arg)
			}
			for _, item := range items {
				t := FindTool(item)
				if t != nil && !t.IsSupportedOnCurrentOS() {
					continue
				}
				if !seen[item] {
					seen[item] = true
					result = append(result, item)
				}
			}
		} else {
			clean := arg
			if alias, ok := toolAliases[strings.ToLower(arg)]; ok {
				clean = alias
			}
			if !seen[clean] {
				seen[clean] = true
				result = append(result, clean)
			}
		}
	}
	return result, nil
}

// ResolveDependencies expande argumentos (@pacotes ou IDs), resolve a árvore de dependências
// em ordem topológica e retorna a lista final ordenada de ferramentas pronta para instalação.
func ResolveDependencies(args []string) ([]Tool, error) {
	expanded, err := Expand(args)
	if err != nil {
		return nil, err
	}

	var ordered []Tool
	visited := make(map[string]bool)
	inStack := make(map[string]bool)

	var visit func(toolID string) error
	visit = func(toolID string) error {
		if inStack[toolID] {
			return fmt.Errorf("ciclo de dependência detectado envolvendo '%s'", toolID)
		}
		if visited[toolID] {
			return nil
		}

		tool := FindTool(toolID)
		if tool == nil {
			return fmt.Errorf("ferramenta ou dependência desconhecida: %s", toolID)
		}

		if !tool.IsSupportedOnCurrentOS() {
			visited[toolID] = true
			return nil
		}

		inStack[toolID] = true

		// Visita primeiro as dependências para que venham antes na ordem de instalação
		for _, depID := range tool.Dependencies {
			depTool := FindTool(depID)
			if depTool == nil {
				return fmt.Errorf("dependência '%s' da ferramenta '%s' não existe no catálogo", depID, toolID)
			}
			if depTool.IsSupportedOnCurrentOS() {
				if err := visit(depID); err != nil {
					return err
				}
			}
		}

		inStack[toolID] = false
		visited[toolID] = true
		ordered = append(ordered, *tool)
		return nil
	}

	for _, id := range expanded {
		if err := visit(id); err != nil {
			return nil, err
		}
	}

	return ordered, nil
}

func TopoDaSub(category string) string {
	switch category {
	case "Agentes", "Skills":
		return "IA"
	case "Editores", "Versionamento", "Terminais", "Fontes":
		return "Código"
	case "Infraestrutura", "Observabilidade":
		return "Sistema"
	case "Desktop":
		return "Apps"
	case "Análise":
		return "Segurança"
	default:
		return category
	}
}

func Topos() []string {
	return []string{"IA", "Código", "Sistema", "Apps", "Segurança"}
}

func ToolsByTopo(topo string) []Tool {
	var list []Tool
	for _, tool := range SupportedCatalog() {
		if TopoDaSub(tool.Category) == topo {
			list = append(list, tool)
		}
	}
	return list
}

func Suites() map[string][]string {
	res := make(map[string][]string, len(packages))
	for k, v := range packages {
		var supported []string
		for _, item := range v {
			t := FindTool(item)
			if t != nil && t.IsSupportedOnCurrentOS() {
				supported = append(supported, item)
			}
		}
		res[k] = supported
	}
	return res
}

var suiteMetadata = []Suite{
	{
		ID:          "vibecode",
		Name:        "Vibe Coding & Pair AI",
		Description: "Desenvolvimento interativo orientado a IA no terminal",
		Category:    "Suítes de IA & Dev",
	},
	{
		ID:          "mcp",
		Name:        "Model Context Protocol Toolkit",
		Description: "Ferramentas, pontes e servidores MCP",
		Category:    "Suítes de IA & Dev",
	},
	{
		ID:          "agentstack",
		Name:        "Agent Engineering & Sandbox",
		Description: "Frameworks, observabilidade e isolamento de agentes",
		Category:    "Suítes de IA & Dev",
	},
	{
		ID:          "skills",
		Name:        "Coleção Curada de Skills",
		Description: "Skills de frontend, engenharia, SEO e CTF",
		Category:    "Suítes de IA & Dev",
	},
	{
		ID:          "dev",
		Name:        "Essenciais p/ Dev",
		Description: "Git, terminal, linguagens e editores essenciais",
		Category:    "Suítes de IA & Dev",
	},
	{
		ID:          "toolchain",
		Name:        "Toolchain C/C++",
		Description: "Compilação nativa com GCC, Make, GDB e CMake",
		Category:    "Conjuntos Integrados",
	},
	{
		ID:          "termstack",
		Name:        "Stack Moderna de Terminal",
		Description: "Wave Terminal, Ghostty, bat, fzf, Zsh e Oh My Zsh",
		Category:    "Conjuntos Integrados",
	},
	{
		ID:          "flatpakfull",
		Name:        "Flatpak + Flathub",
		Description: "Empacotamento sandbox universal no Linux",
		Category:    "Conjuntos Integrados",
	},
}

func AllSuites() []Suite {
	var list []Suite
	for _, meta := range suiteMetadata {
		s := meta
		s.Tools = packages[meta.ID]
		list = append(list, s)
	}
	return list
}

func SupportedSuites() []Suite {
	var list []Suite
	for _, meta := range suiteMetadata {
		var supported []string
		for _, tid := range packages[meta.ID] {
			t := FindTool(tid)
			if t != nil && t.IsSupportedOnCurrentOS() {
				supported = append(supported, tid)
			}
		}
		s := meta
		s.Tools = supported
		list = append(list, s)
	}
	return list
}

func GetSuite(id string) *Suite {
	cleanID := strings.TrimPrefix(strings.TrimSpace(id), "@")
	for _, s := range AllSuites() {
		if strings.EqualFold(s.ID, cleanID) {
			return &s
		}
	}
	return nil
}

func Search(query string) []Tool {
	q := strings.ToLower(strings.TrimSpace(query))
	q = strings.TrimPrefix(q, "https://github.com/")
	q = strings.TrimPrefix(q, "http://github.com/")
	q = strings.TrimSuffix(q, ".git")
	q = strings.TrimSuffix(q, "/")
	if q == "" {
		return SupportedCatalog()
	}

	var results []Tool
	for _, t := range SupportedCatalog() {
		target := t.TargetForCurrentOS()
		pkgID := ""
		if target != nil {
			pkgID = target.PackageID
		}
		if strings.Contains(strings.ToLower(t.ID), q) ||
			strings.Contains(strings.ToLower(t.Name), q) ||
			strings.Contains(strings.ToLower(t.Category), q) ||
			strings.Contains(strings.ToLower(t.Description), q) ||
			strings.Contains(strings.ToLower(pkgID), q) {
			results = append(results, t)
		}
	}
	return results
}

// GetUpstreamRepo retorna a URL canônica do repositório ou projeto oficial da ferramenta se conhecida.
func GetUpstreamRepo(toolID string) string {
	tool := FindTool(toolID)
	if tool == nil {
		return ""
	}

	// 1. Alvos do tipo skill apontam diretamente para o repositório Git
	for _, target := range []*Target{tool.Linux, tool.Windows, tool.Darwin} {
		if target != nil && target.Type == TypeSkill {
			if strings.HasPrefix(target.PackageID, "http") {
				return target.PackageID
			}
			if strings.Contains(target.PackageID, "/") {
				return "https://github.com/" + target.PackageID
			}
		}
	}

	// 2. Verifica se há aliases no formato owner/repo apontando para este ID
	for alias, tid := range toolAliases {
		if tid == tool.ID && strings.Contains(alias, "/") && !strings.Contains(alias, " ") && !strings.HasPrefix(alias, "http") {
			return "https://github.com/" + alias
		}
	}

	// 3. Procura por links do GitHub no PackageID de qualquer plataforma
	for _, target := range []*Target{tool.Linux, tool.Windows, tool.Darwin} {
		if target != nil {
			if strings.HasPrefix(target.PackageID, "https://github.com/") {
				return target.PackageID
			}
		}
	}

	return ""
}
