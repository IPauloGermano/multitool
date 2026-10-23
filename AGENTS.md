# AGENTS.md

> Este arquivo define como um agente de IA deve operar neste repositório: quando pedir contrato antes de codar, como fatiar tarefas, o que verificar antes de entregar, como validar fontes confiáveis e como integrar novas ferramentas sem quebrar a paridade multiplataforma. Formato denso de propósito — é referência operacional, não prosa explicativa.

---

## 0. Projeto

```text
# stack: Go 1.24+ puro (sem CGO) + Cobra CLI + Bubble Tea TUI + Lip Gloss + apt/dnf/pacman/zypper/eopkg + winget + brew + npm/pipx/cargo + git skills
# commands: make test (ou go test -v ./...) | make build | make build-all | ./multitool system | ./multitool list | ./multitool install <id> --dry-run | ./multitool info <id>
# entry points:
#   main.go (ponto de entrada)
#   cmd/root.go (Cobra CLI: subcomandos install, uninstall, update, list, installed, search, info/docs, suites, system, version; flags --dry-run, --json, --all-platforms)
#   internal/system/detect.go (Detecção dinâmica: runtime.GOOS, /etc/os-release, distros Linux, gerenciadores nativos, sudo check, discovery de agentes de IA)
#   internal/catalog/catalog.go (Catálogo universal com alvos duplos Windows/Linux, toolAliases, suítes em packages, Topos e Subsessões)
#   internal/runner/runner.go (Despachante multiplataforma, sudo sob demanda, audit log com rotação 512KB logGira, symlink de skills)
#   internal/registry/registry.go (Controle de estado thread-safe installed.json em ~/.local/share ou %LOCALAPPDATA%)
#   internal/tui/tui.go (Interface reativa Bubble Tea: abas [1-5]/t/p/i/c, busca rápida /, modo r, carrinho, suítes integradas)
# sensitive: curl|sh / AppImage / downloads de releases GitHub + execução de sudo no Linux + criação de symlinks em ~/.claude, ~/.config/opencode, ~/.gemini + gravação em installed.json
# catalogo: 147 ferramentas ativas
```

---

## 1. Postura do Agente

1. Você prevê o próximo token a partir do contexto — não consulta uma fonte de verdade mágica. Pedido vago = especificação incompleta. Pergunte ou declare a suposição; nunca adivinhe em silêncio.
2. Prefira a solução mais **convencional, tipada e previsível** para código Go, não a mais "criativa": mesmo padrão de arquitetura, tratamento de erro explícito (`if err != nil`), sem pânicos não recuperados.
3. Não invente ferramentas, repositórios ou comandos inexistentes. Trate nomes de pacotes, repositórios upstream e comandos como rascunho até confirmar via ferramentas (`gh api`, `curl -I`, `search_web` ou verificação estática).
4. Divida tarefas em fatias verificáveis. Teste proporcional: toda modificação de catálogo ou runner deve ser validada por testes unitários (`go test ./...`).
5. Não altere código sem garantir que os alvos multiplataforma (`Windows` e `Linux`) continuem íntegros.

---

## 2. Contrato Antes de Código

Não codifique sem critério de aceite claro. Se o usuário pedir adição de ferramentas ou modificação de lógica:
```text
# papel: [ex: Adicionar nova ferramenta de IA ao catálogo]
# entrada: [URL do repo ou ID do pacote] # saída: [Suporte Windows + Linux + Teste verde]
# fonte: [Repositório oficial / Release assinado / Package manager verificado]
# aceite:
# [ ] go test ./... passa 100%
# [ ] multitool info <id> resolve nome, alvos e aliases
# [ ] multitool install <id> --dry-run simula com sucesso
# [ ] Documentação e contagens atualizadas
```

---

## 3. Fontes Confiáveis e Validação de Procedência (§ Regra de Ouro)

Ao adicionar qualquer ferramenta solicitada pelo usuário, o agente **DEVE** validar a procedência da fonte antes de adicionar a receita de instalação:

### 3.1. Hierarquia de Fontes Aceitas
1. **Repositório Oficial no GitHub**:
   - Organizações ou mantenedores legítimos (ex: `stablyai/orca`, `nousresearch/hermes`, `anthropics/claude-code`).
   - Verificar se o repositório existe e possui releases/tags ativas (`gh repo view <owner>/<repo>`).
2. **Gerenciadores de Pacotes Oficiais**:
   - **Windows**: Windows Package Manager Community Repository (`winget`).
   - **npm**: Registro oficial (`registry.npmjs.org`), pacotes com escopo ou ampla adoção.
   - **PyPI / pipx**: Pacotes Python verificados via `pipx`.
   - **Crates.io**: Binários Rust via `cargo`.
   - **Linux Nativo**: Pacotes oficiais dos repositórios Debian, Ubuntu, Fedora, Arch, openSUSE ou Flathub.
3. **Downloads Diretos e Releases Binários**:
   - Devem apontar **sempre** para a URL oficial de release da organização no GitHub (ex: `https://github.com/<owner>/<repo>/releases/latest/download/...`).
   - **PROIBIDO**: Apontar para links encurtados, mirrors não verificados, forks desconhecidos ou buckets pessoais sem procedência.

### 3.2. Verificação de URLs e Alvos
- Se o usuário fornecer apenas o nome ou uma URL incompleta, resolva para a URL canônica do GitHub.
- Mapeie aliases comuns (`toolAliases`) para garantir que o usuário consiga chamar via ID curto, nome com hífen ou URL completa:
  - Ex: `orca`, `stablyai/orca`, `stably-orca`, `https://github.com/stablyai/orca`.

---

## 4. Protocolo de Integração de Novas Ferramentas

Ao adicionar uma nova ferramenta (`<id>`), siga **obrigatoriamente** o checklist de 6 etapas:

### 1. Modelagem no Catálogo (`internal/catalog/catalog.go`)
- Definir ID minúsculo, alfanumérico e sem espaços.
- Definir Nome formal, Categoria correta e Descrição em português concisa.
- Definir **Alvo Windows (`*Target`)**:
  - `TypeWinget` com `PackageID` do winget; OU
  - `TypeNpm` / `TypePipx` / `TypeCargo`; OU
  - `TypeDirect` para instaladores oficiais `.exe` / scripts; OU
  - `nil` se comprovadamente incompatível com Windows.
- Definir **Alvo Linux (`*Target`)**:
  - `TypeNative` para pacotes dos gerenciadores de sistema; OU
  - `TypeNpm` / `TypePipx` / `TypeCargo`; OU
  - `TypeFlatpak` com App ID do Flathub; OU
  - `TypeSkill` para repositórios de skills de IA; OU
  - `TypeScript` para AppImage / instaladores curl oficiais; OU
  - `nil` se comprovadamente incompatível com Linux.

### 2. Mapeamento de Aliases (`toolAliases`)
- Adicionar variantes com hífen e formato `owner/repo` apontando para o `<id>`.

### 3. Avaliação de Suítes (`packages`)
- **Não adicione ferramentas a suítes de forma arbitrária**.
- `@vibecode`: Ferramentas focadas em Pair AI, terminal LLMs e Vibe Coding.
- `@mcp`: Servidores, bridges e ferramentas do ecossistema Model Context Protocol.
- `@agentstack`: Harnesses, frameworks multi-agente, observabilidade e sandboxes.
- `@skills`: Apenas skills de metodologia em Markdown (`SKILL.md`) clonadas via git.
- `@dev`: Toolchain universal de desenvolvimento e editores.
- `@toolchain`: Compiladores e automação de build C/C++.
- `@termstack`: Emuladores de terminal modernos, shells e fuzzy finders.

### 4. Implementação no Runner se necessário (`internal/runner/runner.go`)
- Se a ferramenta for `TypeScript` ou `TypeDirect` com lógica de download específico:
  - Adicionar o caso correspondente em `installScript` ou `installDirect`.
  - Adicionar a remoção limpa no `Uninstall`.

### 5. Cascatas Obrigatórias de Documentação e Contagens
Sempre que o número total de ferramentas mudar:
- **`README.md`**: Atualizar contagem total no cabeçalho e descrição das suítes.
- **`docs/catalog-and-suites.md`**: Atualizar contagem total, listas de suítes e adicionar linha na tabela de resumo.
- **`AGENTS.md`**: Atualizar contagem no §0 (`# catalogo: Nx ferramentas`).

### 6. Validação e Testes
- Adicionar caso de teste em `internal/catalog/catalog_test.go` cobrindo o `<id>`, alias e URL do GitHub.
- Executar `make test` (ou `go test -v ./...`) e garantir **100% verde**.
- Executar `make build-all` para certificar que compila em todas as arquiteturas (Linux, Windows, macOS).

---

## 5. Padrões de Engenharia

1. **KISS**: Código direto, tipado e idiomático em Go. Sem abstrações desnecessárias.
2. **Idempotência**: Comandos de instalação, symlinks e persistência em `installed.json` não devem quebrar se executados repetidamente.
3. **Isolamento de Erros**: Falha em uma ferramenta durante execução em lote (`RunBatchInstall`) não deve abortar as demais. O resumo final reporta sucessos e falhas com tempos medidos.
4. **Respeito ao Sistema do Usuário**:
   - Skills nunca são salvas soltas na `$HOME`; ficam isoladas em `~/.local/share/multitool/skills/` ou `%LOCALAPPDATA%\multitool\skills\`.
   - Symlinks são removidos de forma limpa na desinstalação.
   - Logs de auditoria nunca crescem indefinidamente (rotação em 512 KB).

---

## 6. Base de Segurança

1. Nunca use `eval` ou execute strings de fontes externas não sanitizadas.
2. Não desative verificações de certificados TLS (`--insecure` ou `-k` no curl).
3. Scripts de shell executados via Go devem ter argumentos parametrizados.
4. Jamais comite segredos, tokens de API ou credenciais de SSH/GitHub no repositório.
5. Em operações destrutivas ou irreversíveis (remoção de diretórios fora do escopo do multitool), recuse ou exija confirmação explícita.

---

## 7. Checklist Obrigatório de Entrega

Antes de finalizar qualquer alteração ou responder ao usuário:

```text
[ ] 1. Rodou 'make test' (ou 'go test -v ./...') e todos os testes passaram?
[ ] 2. Os alvos para Windows e Linux foram configurados corretamente?
[ ] 3. Adicionou aliases e resolução de URL em toolAliases?
[ ] 4. Atualizou a contagem de ferramentas no README.md, docs/ e AGENTS.md?
[ ] 5. Rodou 'make build-all' e os binários compilaram sem erros?
[ ] 6. O commit foi feito com mensagem semântica clara (feat, fix, docs, chore)?
```
