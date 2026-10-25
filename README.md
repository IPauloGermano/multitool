# Multitool Universal

Orquestrador universal CLI e TUI em **Go** para automação de instalação, remoção, atualização e gerenciamento de ferramentas de desenvolvimento, agentes de inteligência artificial, compiladores e utilitários no **Linux**, **Windows** e **macOS**.

Unificação e evolução integrada dos projetos `linux-multitool` e `windows-multitool` em uma arquitetura única, compilável nativamente sem dependências de interpretador.

---

## 🚀 Funcionalidades

- **Detecção Automática do Sistema Operacional e Distribuição**:
  - **Linux**: Lê `/etc/os-release` e identifica automaticamente se a distro é Ubuntu, Debian, Fedora, Arch, openSUSE ou Solus, mapeando o gerenciador nativo (`apt`, `dnf`, `pacman`, `zypper`, `eopkg`).
  - **Windows**: Identifica x64 e ARM64 (Snapdragon X / Copilot+ PCs) utilizando `winget`.
  - **macOS**: Suporta Apple Silicon e Intel utilizando `brew`.
- **Catálogo Unificado de 154 Ferramentas**:
  - Mapeamento granular com alvos específicos por plataforma (`winget`, `native`, `npm`, `pipx`, `cargo`, `flatpak`, `git skill`, `script`).
  - Filtro contextual: exibe e instala apenas ferramentas compatíveis com o SO atual.
- **TUI Interativa Avançada com Bubble Tea**:
  - Badge no topo indicando o sistema detectado e status (ex: `MULTITOOL • linux (ubuntu / apt)` ou `MULTITOOL • Windows (x64 / winget)`).
  - Abas rápidas: `[1-5]` Topos (IA, Código, Sistema, Apps, Segurança), `[t]` Todas, `[p]` Pacotes (@suítes), `[i]` Instalados, `[c]` Carrinho.
  - Alternância de modo em tempo real com tecla `r` (`instalar` ↔ `desinstalar`).
  - Busca difusa em tempo real com `/`.
  - Carrinho persistente de compras para execução em lote (`Espaço` marca/desmarca, `Enter` executa).
- **CLI Moderna com Cobra**:
  - Subcomandos completos: `install`, `uninstall`, `update`, `list`, `installed`, `search`, `info` (ou `docs`), `suites`, `system`, `sync`, `version`.
  - Flag `--dry-run` para simulação segura sem alterar o sistema.
  - Flag `--json` para automações em scripts, CI e pipelines.
- **Sincronização e Auto-descoberta no Host (`multitool sync`)**:
  - Varre o `$PATH` do sistema e auto-detecta ferramentas do catálogo instaladas previamente fora do Multitool, sincronizando o registro e re-linkando skills existentes para novos agentes.
- **Paralelismo Inteligente no Clone de `@skills`**:
  - Processamento concorrente de skills via worker pool limitado (4 goroutines), acelerando drasticamente o provisionamento em lote.
- **Validação de Integridade Criptográfica (SHA-256)**:
  - Verificação de hash SHA-256 para binários diretos e AppImages contra corrupção e supply-chain tampering.
- **Suítes Integradas (@pacotes)**:
  - `@vibecode`: Aider, Repomix, ast-grep, Ollama, Ripgrep, jq, LLM CLI, Goose, Claude Code, OpenCode, Oh My Pi, 9Router, OmniRoute, Hermes Agent, Orca.
  - `@mcp`: MCPM, MCP Inspector, Supergateway, Codebase Memory, Repomix, Context7, REA, Agent Toolkit for AWS.
  - `@agentstack`: LangGraph CLI, CrewAI, AX, Bubblewrap, Promptfoo, Arize Phoenix, OpenTelemetry Collector, Podman, ECC, Paperclip, HyperFrames, Orca, OpenMausBot, OpenShell.
  - `@skills`: Coleção de 22 skills (incluindo Hallmark, AWS Agent Toolkit, Android Skills, Superpowers, Impeccable, REA, etc.) com linking automático para Claude Code (`~/.claude/skills`), OpenCode (`~/.config/opencode/skills`) e Gemini (`~/.gemini/skills`).
  - `@dev`: Git, GitHub CLI, fzf, Btop, Node.js, Go, Temurin JDK, Docker, VS Code, Neovim, Wave Terminal, Ghostty, Jujutsu, uv, Rustup, bat.
  - `@toolchain`: GCC, Make, GDB, CMake.
  - `@termstack`: Wave Terminal, Ghostty, bat, fzf, Zsh, Oh My Zsh.
  - `@flatpakfull`.
- **Resolução de Dependências em Grafo (DAG)**:
  - Ordenação topológica e detecção de ciclos automática (`Dependencies: []string`).
  - Instalação inteligente de softwares base antes dos dependentes (ex: `Ghidra` ➔ `Temurin JDK`, `Keep`/`OpenGym` ➔ `Docker`, `Oh My Zsh` ➔ `Zsh`, `Flathub` ➔ `Flatpak`).
  - Detecção de requisitos já satisfeitos no hospedeiro para evitar execuções redundantes.
- **Auto-provisionamento de Runtimes no Host**:
  - Quando uma ferramenta requer `npm`, `pipx`, `cargo`, `flatpak` ou `git`, o Multitool detecta comandos ausentes e os provisiona proativamente através do gerenciador de pacotes nativo do sistema (`apt`, `dnf`, `pacman`, `zypper`, `eopkg`, `brew`, `winget`).
- **Rastreamento de Estado & Auditoria**:
  - Registra instalações em formato JSON estruturado (`~/.local/share/multitool/installed.json` no Linux/macOS ou `%LOCALAPPDATA%\multitool\installed.json` no Windows).
  - Registro de auditoria com rotação automática em 512 KB (`multitool.log`).

---

## 🛠️ Como Usar

### 1. Menu Interativo (TUI)
Para iniciar a interface terminal com abas e busca em tempo real:
```bash
# No Linux / macOS:
./multitool

# No Windows (PowerShell ou CMD):
multitool.exe
```

> **Atalhos no Menu**:
> - `[1]` a `[5]`: Filtra por categoria topo (1=IA, 2=Código, 3=Sistema, 4=Apps, 5=Segurança).
> - `t`: Exibe todas as ferramentas suportadas.
> - `i`: Filtra ferramentas instaladas.
> - `c`: Abre o carrinho de compras.
> - `/`: Digita termo para busca e filtro instantâneo.
> - `r`: Alterna o modo (`instalar` ↔ `desinstalar`).
> - `↑` / `↓` ou `k` / `j`: Navega entre itens (rolagem responsiva via viewport).
> - `Espaço`: Marca/desmarca do carrinho.
> - `a`: Marca todas as ferramentas da aba atual no carrinho.
> - `A`: Limpa completamente o carrinho.
> - `d`: Abre o modal de detalhes da ferramenta ou pacote no cursor (sempre acessível, mesmo com itens no carrinho).
> - `Enter`: Se houver itens no carrinho, executa o lote; se o carrinho estiver vazio, abre modal de detalhes da ferramenta ou pacote.
> - `q` ou `Esc`: Fecha o modal ou sai do programa.

---

### 2. Comandos CLI

```bash
# Diagnóstico do sistema operacional e runtimes detectados
multitool system

# Auto-detectar e sincronizar ferramentas pré-existentes no sistema host
multitool sync
multitool sync --dry-run

# Listar ferramentas disponíveis na plataforma atual
multitool list

# Listar todas as 154 ferramentas de todas as plataformas
multitool list --all-platforms

# Listar em formato JSON para automação
multitool list --json

# Buscar ferramentas por palavra-chave
multitool search ai
multitool search terminal

# Exibir detalhes, repositório e alvos por SO de uma ferramenta
multitool info universalmodder
multitool info hermes

# Listar suítes integradas (@vibecode, @mcp, etc.)
multitool suites

# Instalar ferramentas ou suítes
multitool install btop @vibecode

# Simulação segura sem alterações no sistema (--dry-run)
multitool install @agentstack --dry-run

# Desinstalar ferramentas ou suítes
multitool uninstall ripgrep

# Atualizar pacotes
multitool update @mcp
```

---

## 🏗️ Compilação Multiplataforma

O projeto foi construído em Go puro (sem CGO) e pode ser cross-compilado para qualquer plataforma a partir de qualquer sistema operacional:

```bash
# Executar todos os testes unitários
make test

# Compilar binário nativo para a máquina atual
make build

# Compilar binários Linux (x64 e ARM64)
make build-linux

# Compilar binários Windows (x64 e ARM64)
make build-windows

# Compilar todas as plataformas de uma só vez (Linux, Windows, macOS)
make build-all
```

---

## 📄 Licença

Distribuído sob a licença **MIT**. Consulte o arquivo [`LICENSE`](LICENSE) para mais informações.
