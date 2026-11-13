package cmd

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
)

type partialFailureExecutor struct {
	failedPackage string
	failure       error
	commands      []string
}

func (e *partialFailureExecutor) Execute(name string, args ...string) error {
	command := strings.Join(append([]string{name}, args...), " ")
	e.commands = append(e.commands, command)
	for _, arg := range args {
		if arg == e.failedPackage {
			return e.failure
		}
	}
	return nil
}

func (*partialFailureExecutor) LookPath(name string) (string, error) { return "/mock/" + name, nil }

func TestCLIPartialBatchReturnsError(t *testing.T) {
	for _, operation := range []string{"install", "uninstall", "update"} {
		t.Run(operation, func(t *testing.T) {
			reg, err := registry.New(filepath.Join(t.TempDir(), "installed.json"))
			if err != nil {
				t.Fatal(err)
			}
			sentinel := errors.New("package failed")
			executor := &partialFailureExecutor{failedPackage: catalog.FindTool("ripgrep").TargetForCurrentOS().PackageID, failure: sentinel}
			command := newRootCmd(reg, executor)
			command.SetArgs([]string{operation, "ripgrep", "git"})
			command.SilenceUsage = true
			command.SilenceErrors = true
			err = command.Execute()
			if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "ripgrep") {
				t.Fatalf("CLI concealed failure: %v", err)
			}
			if !strings.Contains(err.Error(), "1 sucesso(s), 1 falha(s)") {
				t.Fatalf("incorrect summary: %v", err)
			}
		})
	}
}
