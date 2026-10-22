package main

import (
	"fmt"
	"os"

	"github.com/IPauloGermano/multitool/cmd"
	"github.com/IPauloGermano/multitool/internal/registry"
)

func main() {
	reg, err := registry.New("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao carregar registro: %v\n", err)
		os.Exit(1)
	}

	rootCmd := cmd.NewRootCmd(reg)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
