# Guia de Uso: Interface CLI e TUI

O **Multitool** oferece duas formas complementares de interação:
1. **Interface Interativa Terminal (TUI)**: para navegação visual, busca difusa rápida e seleção por carrinho.
2. **Interface de Linha de Comando (CLI)**: com subcomandos modernos, flags de automação e integração com scripts.

---

## 🖥️ 1. Interface Interativa (TUI)

Para abrir a interface terminal:
```bash
./multitool          # Linux / macOS
multitool.exe        # Windows
```

### Layout da Interface

```
 MULTITOOL   linux (ubuntu / apt) 

[ MODO: INSTALAR ]    Carrinho: 2 item(ns)

Abas: [1] IA  [2] Código  [3] Sistema  [4] Apps  [5] Segurança  |  [t] Todas  [p] Pacotes  [i] Instalados  [c] Carrinho

▶ [✓] btop           Btop                   (native)   Monitor de recursos no terminal [instalado]
  [ ] git            Git                    (native)   Controle de versão distribuído
  [✓] ollama         Ollama                 (script)   Servidor local de modelos de IA

Atalhos: [↑/↓] Navegar • [Espaço] Marcar • [d] Detalhes • [Enter] Executar • [a/A] Tudo/Limpar • [r] Modo • [/] Buscar • [q] Sair
```

### Tabela de Atalhos do Teclado

| Tecla | Ação |
| :---: | :--- |
| `↑` / `↓` ou `k` / `j` | Navega entre as ferramentas ou suítes da lista atual (com rolagem suave via `bubbles/viewport`). |
| `PgUp` / `PgDown` | Rolagem por página na viewport ativa. |
| `Espaço` | Marca ou desmarca a ferramenta atual (ou todos os membros da suíte selecionada) no carrinho. |
| `d` | **Detalhes**: Abre a janela modal com detalhes completos da ferramenta ou pacote no cursor (sempre acessível, mesmo com itens no carrinho). Pressione novamente para fechar. |
| `Enter` | **Executar ou Inspecionar**: Se o carrinho tiver itens, executa a ação em lote. Se o carrinho estiver vazio, abre a janela modal com detalhes da ferramenta (`info`) ou pacote. |
| `a` | **Marcar Tudo**: Adiciona todas as ferramentas da visualização/aba atual ao carrinho. |
| `A` | **Limpar Carrinho**: Remove todos os itens selecionados do carrinho. |
| `r` | Alterna entre os modos de operação: `[ MODO: INSTALAR ]` ↔ `[ MODO: DESINSTALAR ]`. |
| `/` | Ativa a busca difusa em tempo real (digite o nome, descrição ou ferramenta). |
| `Esc` | Fecha o modal de detalhes ativo, limpa o filtro de busca ou encerra o programa. |
| `1` a `5` | Filtra por topo: `1`=IA, `2`=Código, `3`=Sistema, `4`=Apps, `5`=Segurança. |
| `t` | Aba "Todas": exibe todas as ferramentas suportadas pela plataforma. |
| `p` | Aba "Pacotes": exibe e gerencia suítes integradas (`@vibecode`, `@dev`, `@mcp`, etc.). |
| `i` | Aba "Instalados": exibe apenas ferramentas registradas no sistema. |
| `c` | Aba "Carrinho": exibe apenas as ferramentas marcadas para execução. |
| `q` | Fecha o modal ativo ou encerra o programa. |

---

## ⌨️ 2. Guia de Comandos CLI

### Diagnóstico do Sistema: `system`
Exibe os dados detalhados da detecção do sistema operacional:
```bash
multitool system
```

Saída de exemplo:
```text
 DIAGNÓSTICO DO SISTEMA 
• Sistema Operacional:  linux (amd64)
• Distribuição Linux:   Ubuntu 26.04.1 LTS (ID: ubuntu)
• Gerenciador Nativo:   apt
• Privilégios Sudo:     requer sudo=true, possui sudo=true
• Runtimes Detectados:  apt-get, cargo, curl, git, npm, sudo
• Diretórios de Skills: /home/ubuntu/.config/opencode/skills, /home/ubuntu/.gemini/skills
```

---

### Sincronização com o Sistema Host: `sync`
Detecta ferramentas pré-existentes instaladas no sistema operacional (fora do Multitool) e sincroniza o registro local (`installed.json`):
```bash
# Detectar e registrar ferramentas presentes no PATH
multitool sync

# Simular a detecção sem gravar no registro
multitool sync --dry-run

# Exportar resultado estruturado em JSON
multitool sync --json
```

O comando também re-linka automaticamente as skills locais existentes para novos agentes de IA detectados na máquina.

---

### Listagem: `list` e `installed`

* **Listar ferramentas suportadas no SO atual**:
  ```bash
  multitool list
  ```
* **Listar o catálogo completo de todas as plataformas**:
  ```bash
  multitool list --all-platforms
  ```
* **Listar apenas o que já está instalado**:
  ```bash
  multitool installed
  ```
* **Exportar saída em JSON para scripts**:
  ```bash
  multitool list --json
  multitool installed --json
  ```

---

### Busca no Catálogo: `search`
Pesquisa por ID, nome, categoria ou termos da descrição:
```bash
multitool search terminal
multitool search ai
multitool search "code review"
```

---

### Detalhes de Ferramenta: `info` (ou `docs`)
Exibe a descrição completa, alvos em cada plataforma e status de instalação:
```bash
multitool info universalmodder
multitool info hermes
multitool docs git
```

Suporta resolução inteligente de URLs do GitHub e aliases com hífen:
```bash
multitool info https://github.com/rehan-remade/universal-modder
multitool info diagram-design
```

---

### Exibição de Suítes: `suites` (ou `packages`)
Lista todos os pacotes integrados e a contagem de ferramentas suportadas:
```bash
multitool suites
```

---

### Operações em Lote: `install`, `uninstall`, `update`

* **Instalar uma ou mais ferramentas**:
  ```bash
  multitool install btop ripgrep
  ```
* **Instalar uma ou várias suítes**:
  ```bash
  multitool install @vibecode
  multitool install @dev @toolchain
  ```
* **Desinstalar ferramentas ou suítes**:
  ```bash
  multitool uninstall ripgrep
  multitool uninstall @mcp
  ```
* **Atualizar ferramentas ou suítes para a versão mais recente**:
  ```bash
  multitool update @vibecode
  multitool update ollama
  ```

---

## 🛡️ 3. Modo de Simulação (`--dry-run`)

Adicione `--dry-run` a qualquer comando para validar o que seria executado sem fazer alterações reais no sistema operacional:

```bash
multitool install @vibecode --dry-run
multitool uninstall ripgrep --dry-run
```

Exemplo de saída de simulação:
```text
--- [1/2] ---
=== Instalando Git (git) ===
[DRY-RUN] Simulando instalação de Git via native (git)

--- [2/2] ---
=== Instalando Repomix (repomix) ===
[DRY-RUN] Simulando instalação de Repomix via npm (repomix)

Resumo: 2 instalado(s), 0 falha(s) em 5ms.
```
