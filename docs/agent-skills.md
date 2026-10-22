# Ciclo de Vida e Integração de Skills para Agentes de IA

Um dos recursos mais inovadores do **Multitool** é a gestão nativa e transparente de **Skills de IA**.

---

## 💡 O que são Agent Skills?

Skills são diretórios contendo regras operacionais padronizadas em Markdown (frequentemente com um arquivo central `SKILL.md`), scripts e ferramentas auxiliares que orientam agentes de inteligência artificial sobre metodologias específicas (TDD, microinterações, modding, revisão de código ou segurança).

---

## 🔄 Fluxo de Instalação e Linking

Quando você instala qualquer skill (individualmente ou via `@skills`), o Multitool realiza o ciclo completo sem necessidade de configuração manual:

```mermaid
flowchart TD
    A["multitool install superpowers"] --> B["Clone Git Neutro do Repositório"]
    B --> C["Salva em repositório central neutro:\n~/.local/share/multitool/skills/superpowers\nou\n%LOCALAPPDATA%\\multitool\\skills\\superpowers"]
    
    C --> D["Varredura do Ambiente por Agentes Locais"]
    
    D --> E1{"Existe ~/.claude?"}
    E1 -->|"Sim"| F1["Cria Symlink em\n~/.claude/skills/superpowers"]
    
    D --> E2{"Existe OpenCode?"}
    E2 -->|"Sim"| F2["Cria Symlink em\n~/.config/opencode/skills/superpowers"]
    
    D --> E3{"Existe ~/.gemini?"}
    E3 -->|"Sim"| F3["Cria Symlink em\n~/.gemini/skills/superpowers"]
    
    F1 --> G["Skill pronta para uso imediato em todos os agentes"]
    F2 --> G
    F3 --> G
```

---

## 📍 Localizações por Sistema Operacional

### 1. Repositório Neutro (Clone Central)
* **Linux / macOS**: `~/.local/share/multitool/skills/<id>`
* **Windows**: `%LOCALAPPDATA%\multitool\skills\<id>`

### 2. Destinos de Symlinks por Agente Detectado

| Agente de IA | Caminho no Linux / macOS | Caminho no Windows |
| :--- | :--- | :--- |
| **Claude Code** | `~/.claude/skills/<id>` | `%USERPROFILE%\.claude\skills\<id>` |
| **OpenCode** | `~/.config/opencode/skills/<id>` | `%LOCALAPPDATA%\opencode\skills\<id>` |
| **Gemini CLI / Antigravity** | `~/.gemini/skills/<id>` | `%USERPROFILE%\.gemini\skills\<id>` |

Se o agente for instalado posteriormente, basta executar `multitool update @skills` para que os links sejam atualizados e criados automaticamente.

---

## 🧹 Desinstalação Limpa

Ao executar:
```bash
multitool uninstall superpowers
```
O Multitool:
1. Localiza todos os symlinks nos agentes instalados (`.claude`, `.config/opencode`, `.gemini`).
2. Remove os links de forma limpa.
3. Remove o clone do diretório neutro central.
4. Desregistra a ferramenta no arquivo `installed.json`.
