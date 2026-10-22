# Guia de Contribuição e Desenvolvimento

Este documento descreve as etapas para estender o **Multitool**, adicionar novas ferramentas, manter a paridade entre plataformas e rodar a suíte de testes.

---

## 🛠️ 1. Ambiente de Desenvolvimento

### Pré-requisitos
* **Go 1.22+** (recomendado Go 1.24+).
* Git e Make.

### Clonar e Compilar
```bash
git clone https://github.com/IPauloGermano/multitool.git
cd multitool

# Compilar binário nativo
make build

# Executar a bateria completa de testes
make test
```

---

## ➕ 2. Adicionando uma Nova Ferramenta

Para adicionar uma nova ferramenta ao catálogo, edite o arquivo **[`internal/catalog/catalog.go`](../internal/catalog/catalog.go)**.

### Passo 1: Definir a Entrada no `catalogList`
Adicione o item preenchendo os alvos para Windows e Linux:

```go
{
    ID:          "minhaferramenta",
    Name:        "Minha Ferramenta",
    Category:    "Agentes", // Categorias: Agentes, Skills, Editores, Versionamento, Terminais, Fontes, Infraestrutura, Observabilidade, Desktop, Análise
    Description: "Descrição concisa do utilitário",
    
    // Alvo para Windows (via winget, npm, pipx, cargo ou direct)
    Windows: &Target{
        Type:      TypeWinget,
        PackageID: "Vendor.MinhaFerramenta",
        Binary:    "minhaferramenta.exe",
    },
    
    // Alvo para Linux (via native, npm, pipx, cargo, flatpak, skill ou script)
    Linux: &Target{
        Type:      TypeNative,
        PackageID: "minha-ferramenta",
        Binary:    "minhaferramenta",
    },
},
```

> **Dica**: Se a ferramenta não estiver disponível em uma das plataformas, defina o alvo como `nil`. O sistema filtrará automaticamente no SO correspondente.

### Passo 2: Adicionar Aliases (Opcional)
Se a ferramenta for frequentemente chamada com hífens ou referenciada por URL do GitHub, adicione o mapeamento no mapa `toolAliases`:
```go
var toolAliases = map[string]string{
    "minha-ferramenta":              "minhaferramenta",
    "github-user/minha-ferramenta":  "minhaferramenta",
}
```

### Passo 3: Adicionar a uma Suíte (Opcional)
Se a ferramenta fizer parte de um conjunto lógico coeso, adicione seu `ID` ao mapa `packages`:
```go
"vibecode": {
    ..., "minhaferramenta",
},
```

---

## 🧪 3. Executando e Escrevendo Testes

Antes de submeter qualquer alteração, certifique-se de que todos os testes estejam passando:

```bash
make test
```

Para rodar testes com verbosidade detalhada de um pacote específico:
```bash
go test -v ./internal/catalog
go test -v ./internal/runner
go test -v ./internal/system
go test -v ./cmd
```

---

## 📦 4. Compilação Multiplataforma

O `Makefile` inclui tarefas automatizadas para gerar binários de release:

```bash
# Compilar todos os binários (Linux amd64/arm64, Windows x64/arm64, macOS arm64/amd64)
make build-all
```
