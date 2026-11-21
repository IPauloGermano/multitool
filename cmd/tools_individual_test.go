package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
)

func classifyCLIKind(tool catalog.Tool) string {
	if tool.Category == "Skills" {
		return "skill"
	}
	if tool.Category == "Desktop" {
		return "app"
	}
	return "tool"
}

func TestIndividualCLITools(t *testing.T) {
	tools := catalog.DefaultCatalog()
	if len(tools) == 0 {
		t.Fatal("catálogo padrão vazio")
	}

	for _, tool := range tools {
		tool := tool
		subName := classifyCLIKind(tool) + "/" + tool.ID
		t.Run(subName, func(t *testing.T) {
			reg, err := registry.New(filepath.Join(t.TempDir(), "installed.json"))
			if err != nil {
				t.Fatalf("erro ao criar registro: %v", err)
			}
			executor := &partialFailureExecutor{}

			runCLI := func(args ...string) (string, error) {
				buf := new(bytes.Buffer)
				cmd := newRootCmd(reg, executor)
				cmd.SetOut(buf)
				cmd.SetErr(buf)
				cmd.SetArgs(args)
				err := cmd.Execute()
				return buf.String(), err
			}

			// 1. info <id> (texto)
			out, err := runCLI("info", tool.ID)
			if err != nil {
				t.Fatalf("multitool info %s falhou: %v", tool.ID, err)
			}
			if !strings.Contains(out, tool.Name) {
				t.Errorf("info %s não contém nome %q: %s", tool.ID, tool.Name, out)
			}
			if !strings.Contains(out, tool.ID) {
				t.Errorf("info %s não contém ID %q: %s", tool.ID, tool.ID, out)
			}
			if !strings.Contains(out, tool.Description) {
				t.Errorf("info %s não contém descrição %q: %s", tool.ID, tool.Description, out)
			}
			if !tool.IsSupportedOnCurrentOS() && !strings.Contains(out, "Não disponível nesta plataforma") {
				t.Errorf("info %s em plataforma não suportada deveria indicar indisponibilidade: %s", tool.ID, out)
			}

			// 2. info <id> --json
			jsonOut, err := runCLI("info", tool.ID, "--json")
			if err != nil {
				t.Fatalf("multitool info %s --json falhou: %v", tool.ID, err)
			}
			var decoded catalog.Tool
			if err := json.Unmarshal([]byte(jsonOut), &decoded); err != nil {
				t.Fatalf("info %s --json retornou JSON inválido: %v\n%s", tool.ID, err, jsonOut)
			}
			if decoded.ID != tool.ID || decoded.Name != tool.Name {
				t.Errorf("JSON decodificado divergiu: obteve ID=%q Name=%q, esperado ID=%q Name=%q", decoded.ID, decoded.Name, tool.ID, tool.Name)
			}

			// 3. Fluxo dry-run e search para ferramentas suportadas no SO atual
			if !tool.IsSupportedOnCurrentOS() {
				return
			}

			searchOut, err := runCLI("search", tool.ID)
			if err != nil {
				t.Fatalf("multitool search %s falhou: %v", tool.ID, err)
			}
			if !strings.Contains(searchOut, tool.ID) {
				t.Errorf("search %s não listou o próprio ID: %s", tool.ID, searchOut)
			}

			if _, err := runCLI("install", tool.ID, "--dry-run"); err != nil {
				t.Fatalf("multitool install %s --dry-run falhou: %v", tool.ID, err)
			}
			if !reg.IsInstalled(tool.ID) {
				t.Errorf("%s deveria constar no registro após install --dry-run", tool.ID)
			}

			if _, err := runCLI("update", tool.ID, "--dry-run"); err != nil {
				t.Fatalf("multitool update %s --dry-run falhou: %v", tool.ID, err)
			}

			if _, err := runCLI("uninstall", tool.ID, "--dry-run"); err != nil {
				t.Fatalf("multitool uninstall %s --dry-run falhou: %v", tool.ID, err)
			}
			if reg.IsInstalled(tool.ID) {
				t.Errorf("%s não deveria constar no registro após uninstall --dry-run", tool.ID)
			}

			if len(executor.commands) != 0 {
				t.Errorf("operações --dry-run não deveriam invocar comandos externos, executou: %v", executor.commands)
			}
		})
	}
}
