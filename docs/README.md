# Documentação do Multitool

Bem-vindo à documentação oficial do **Multitool**, o orquestrador universal de ferramentas de desenvolvimento, agentes de inteligência artificial, compiladores e aplicativos de sistema para Linux, Windows e macOS.

---

## 📚 Índice da Documentação

| Documento | Descrição |
| :--- | :--- |
| **[1. Arquitetura e Design](architecture.md)** | Visão interna, camadas de detecção de SO/distro, ciclo de vida de execução e controle de estado. |
| **[2. Guia de Uso: CLI e TUI](cli-and-tui.md)** | Manual detalhado de todos os comandos CLI, flags de simulação (`--dry-run`), automação (`--json`) e atalhos da interface interativa Bubble Tea. |
| **[3. Catálogo e Suítes](catalog-and-suites.md)** | Relação completa das 144 ferramentas, divisão em Topos/Categorias e detalhamento das suítes (`@vibecode`, `@mcp`, `@agentstack`, `@skills`, `@dev`, etc.). |
| **[4. Integração com Agentes de IA](agent-skills.md)** | Como funciona o ciclo de vida e linking automático de skills para Claude Code, OpenCode e Gemini CLI / Antigravity. |
| **[5. Guia de Plataformas](platform-guide.md)** | Especificidades de distribuições Linux (`apt`, `dnf`, `pacman`, `zypper`, `eopkg`), Windows (`winget`, x64, ARM64 Copilot+) e macOS (`brew`). |
| **[6. Guia de Contribuição](contributing.md)** | Como adicionar novas ferramentas ao catálogo, convenções de código, testes unitários e compilação cruzada. |

---

## ⚡ Início Rápido

### Executar a Interface Interativa (TUI)
```bash
# Linux / macOS
./multitool

# Windows
multitool.exe
```

### Comandos Rápidos Essenciais
```bash
# Diagnóstico completo do sistema detectado
multitool system

# Listar ferramentas compatíveis com seu sistema operacional
multitool list

# Ver suítes integradas disponíveis
multitool suites

# Simulação segura da instalação de uma suíte (--dry-run)
multitool install @vibecode --dry-run

# Instalar ferramentas reais
multitool install btop ripgrep @vibecode
```
