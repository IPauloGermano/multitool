package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
	"github.com/IPauloGermano/multitool/internal/runner"
	"github.com/IPauloGermano/multitool/internal/system"
	"github.com/IPauloGermano/multitool/internal/tui"
)

var (
	Version   = "2.1.0"
	BuildDate = "2026-10-22"

	flagList         bool
	flagInstalled    bool
	flagInstall      []string
	flagUninstall    []string
	flagUpdate       []string
	flagDryRun       bool
	flagJSON         bool
	flagAllPlatforms bool

	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#2563EB"))
	styleDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	styleGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
	styleCyan  = lipgloss.NewStyle().Foreground(lipgloss.Color("#06B6D4"))
	styleRed   = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true)
)

func NewRootCmd(reg *registry.Registry) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "multitool",
		Short: "multitool - Orquestrador Universal de Ferramentas, IA e Agentes",
		Long: `multitool é um orquestrador CLI e TUI multiplataforma (Linux, Windows e macOS)
para automação de instalação, remoção, atualização e auditoria de ferramentas de desenvolvimento,
agentes de IA, compiladores e aplicativos de sistema.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			r := runner.New(reg, flagDryRun)

			if flagList {
				return runList(out, reg, flagJSON, flagAllPlatforms)
			}
			if flagInstalled {
				return runInstalled(out, reg, flagJSON)
			}
			if len(flagInstall) > 0 {
				return runInstall(r, flagInstall)
			}
			if len(flagUninstall) > 0 {
				return runUninstall(r, flagUninstall)
			}
			if len(flagUpdate) > 0 {
				return runUpdate(r, flagUpdate)
			}

			// Se nenhuma flag for passada, abre a TUI interativa
			return tui.Run(reg, flagDryRun)
		},
	}

	rootCmd.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "Simula operações sem alterar o sistema")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Exibe saída estruturada em JSON")

	rootCmd.Flags().BoolVarP(&flagList, "list", "l", false, "Lista as ferramentas disponíveis")
	rootCmd.Flags().BoolVar(&flagAllPlatforms, "all-platforms", false, "Exibe catálogo de todas as plataformas")
	rootCmd.Flags().BoolVar(&flagInstalled, "installed", false, "Lista apenas as ferramentas instaladas")
	rootCmd.Flags().StringSliceVarP(&flagInstall, "install", "i", nil, "Instala ferramentas ou @pacotes (ex: --install btop @vibecode)")
	rootCmd.Flags().StringSliceVarP(&flagUninstall, "uninstall", "u", nil, "Desinstala ferramentas ou @pacotes")
	rootCmd.Flags().StringSliceVar(&flagUpdate, "update", nil, "Atualiza ferramentas ou @pacotes")

	// Subcomandos
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Lista as ferramentas disponíveis no catálogo",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.OutOrStdout(), reg, flagJSON, flagAllPlatforms)
		},
	}
	listCmd.Flags().BoolVar(&flagAllPlatforms, "all-platforms", false, "Exibe catálogo de todas as plataformas")

	installedCmd := &cobra.Command{
		Use:   "installed",
		Short: "Lista as ferramentas instaladas no sistema",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstalled(cmd.OutOrStdout(), reg, flagJSON)
		},
	}

	installCmd := &cobra.Command{
		Use:   "install [id|@pacote...]",
		Short: "Instala uma ou mais ferramentas ou @suítes",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r := runner.New(reg, flagDryRun)
			return runInstall(r, args)
		},
	}

	uninstallCmd := &cobra.Command{
		Use:   "uninstall [id|@pacote...]",
		Short: "Desinstala uma ou mais ferramentas ou @suítes",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r := runner.New(reg, flagDryRun)
			return runUninstall(r, args)
		},
	}

	updateCmd := &cobra.Command{
		Use:   "update [id|@pacote...]",
		Short: "Atualiza uma ou mais ferramentas ou @suítes",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r := runner.New(reg, flagDryRun)
			return runUpdate(r, args)
		},
	}

	searchCmd := &cobra.Command{
		Use:   "search [termo]",
		Short: "Busca ferramentas no catálogo por nome, ID ou descrição",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			results := catalog.Search(args[0])
			out := cmd.OutOrStdout()

			if flagJSON {
				data, _ := json.MarshalIndent(results, "", "  ")
				fmt.Fprintln(out, string(data))
				return nil
			}

			if len(results) == 0 {
				fmt.Fprintf(out, "Nenhuma ferramenta encontrada para '%s'.\n", args[0])
				return nil
			}

			fmt.Fprintf(out, "%s Encontradas %d ferramentas:\n\n", styleGreen.Render("✔"), len(results))
			for _, t := range results {
				tag := ""
				if reg.IsInstalled(t.ID) {
					tag = styleGreen.Render(" [instalado]")
				}
				method := ""
				target := t.TargetForCurrentOS()
				if target != nil {
					method = fmt.Sprintf("(%s)", target.Type)
				}
				fmt.Fprintf(out, "%-14s %-24s [%s] %-10s %s%s\n", t.ID, t.Name, t.Category, method, t.Description, tag)
			}
			return nil
		},
	}

	infoCmd := &cobra.Command{
		Use:     "info [id]",
		Aliases: []string{"docs"},
		Short:   "Exibe detalhes, documentação e alvos de uma ferramenta",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tool := catalog.FindTool(args[0])
			if tool == nil {
				return fmt.Errorf("ferramenta não encontrada: %s", args[0])
			}

			out := cmd.OutOrStdout()
			if flagJSON {
				data, _ := json.MarshalIndent(tool, "", "  ")
				fmt.Fprintln(out, string(data))
				return nil
			}

			installed := "Não"
			if reg.IsInstalled(tool.ID) {
				installed = styleGreen.Render("Sim")
			}

			fmt.Fprintf(out, "\n%s\n", styleTitle.Render(fmt.Sprintf(" %s (%s) ", tool.Name, tool.ID)))
			fmt.Fprintf(out, "• Descrição:    %s\n", tool.Description)
			fmt.Fprintf(out, "• Categoria:    %s (%s)\n", tool.Category, catalog.TopoDaSub(tool.Category))
			fmt.Fprintf(out, "• Instalado:    %s\n", installed)
			if len(tool.Dependencies) > 0 {
				fmt.Fprintf(out, "• Dependências: %s\n", styleCyan.Render(strings.Join(tool.Dependencies, ", ")))
			}

			target := tool.TargetForCurrentOS()
			if target != nil {
				fmt.Fprintf(out, "• Alvo Atual:   %s (%s)\n", styleCyan.Render(target.PackageID), target.Type)
				fmt.Fprintf(out, "• Executável:   %s\n", target.Binary)
			} else {
				fmt.Fprintf(out, "• Alvo Atual:   %s\n", styleRed.Render("Não disponível nesta plataforma"))
			}

			if tool.Windows != nil {
				fmt.Fprintf(out, "• Windows:      %s [%s]\n", tool.Windows.PackageID, tool.Windows.Type)
			}
			if tool.Linux != nil {
				fmt.Fprintf(out, "• Linux:        %s [%s]\n", tool.Linux.PackageID, tool.Linux.Type)
			}
			fmt.Fprintln(out)
			return nil
		},
	}

	suitesCmd := &cobra.Command{
		Use:     "suites",
		Aliases: []string{"packages"},
		Short:   "Lista os pacotes e suítes integradas (@vibecode, @mcp, etc.)",
		RunE: func(cmd *cobra.Command, args []string) error {
			suites := catalog.Suites()
			out := cmd.OutOrStdout()

			if flagJSON {
				data, _ := json.MarshalIndent(suites, "", "  ")
				fmt.Fprintln(out, string(data))
				return nil
			}

			keys := make([]string, 0, len(suites))
			for k := range suites {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			fmt.Fprintf(out, "%s Suítes de Ferramentas disponíveis:\n\n", styleTitle.Render(" CONJUNTOS INTEGRADOS "))
			for _, k := range keys {
				members := suites[k]
				fmt.Fprintf(out, "  %s (%d ferramentas)\n    %s\n\n",
					styleGreen.Render("@"+k), len(members), styleDim.Render(strings.Join(members, ", ")))
			}
			return nil
		},
	}

	systemCmd := &cobra.Command{
		Use:   "system",
		Short: "Exibe diagnóstico do sistema operacional detectado",
		RunE: func(cmd *cobra.Command, args []string) error {
			info := system.GetSystemInfo()
			out := cmd.OutOrStdout()

			if flagJSON {
				data, _ := json.MarshalIndent(info, "", "  ")
				fmt.Fprintln(out, string(data))
				return nil
			}

			fmt.Fprintf(out, "\n%s\n", styleTitle.Render(" DIAGNÓSTICO DO SISTEMA "))
			fmt.Fprintf(out, "• Sistema Operacional:  %s (%s)\n", styleCyan.Render(string(info.OS)), info.Arch)
			if info.DistroPretty != "" {
				fmt.Fprintf(out, "• Distribuição Linux:   %s (ID: %s)\n", styleGreen.Render(info.DistroPretty), info.Distro)
			}
			fmt.Fprintf(out, "• Gerenciador Nativo:   %s\n", styleGreen.Render(string(info.PrimaryManager)))
			fmt.Fprintf(out, "• Privilégios Sudo:     requer sudo=%v, possui sudo=%v\n", info.NeedsSudo, info.HasSudo)

			var available []string
			for k, v := range info.AvailableTools {
				if v {
					available = append(available, k)
				}
			}
			sort.Strings(available)
			fmt.Fprintf(out, "• Runtimes Detectados:  %s\n", styleDim.Render(strings.Join(available, ", ")))

			if len(info.SkillDirs) > 0 {
				fmt.Fprintf(out, "• Diretórios de Skills: %s\n", strings.Join(info.SkillDirs, ", "))
			} else {
				fmt.Fprintf(out, "• Diretórios de Skills: Nenhum agente detectado (~/.claude, ~/.config/opencode, ~/.gemini)\n")
			}
			fmt.Fprintln(out)
			return nil
		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Exibe a versão do Multitool",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "multitool v%s (%s/%s) built on %s\n", Version, runtime.GOOS, runtime.GOARCH, BuildDate)
			return nil
		},
	}

	rootCmd.AddCommand(listCmd, installedCmd, installCmd, uninstallCmd, updateCmd, searchCmd, infoCmd, suitesCmd, systemCmd, versionCmd)
	return rootCmd
}

func runList(out io.Writer, reg *registry.Registry, asJSON, allPlatforms bool) error {
	var tools []catalog.Tool
	if allPlatforms {
		tools = catalog.DefaultCatalog()
	} else {
		tools = catalog.SupportedCatalog()
	}

	if asJSON {
		data, _ := json.MarshalIndent(tools, "", "  ")
		fmt.Fprintln(out, string(data))
		return nil
	}

	for _, t := range tools {
		tag := ""
		if reg.IsInstalled(t.ID) {
			tag = styleGreen.Render(" [instalado]")
		}
		method := ""
		target := t.TargetForCurrentOS()
		if target != nil {
			method = fmt.Sprintf("(%s)", target.Type)
		} else {
			method = styleRed.Render("(outro SO)")
		}
		fmt.Fprintf(out, "%-14s %-24s [%s] %-12s %s%s\n", t.ID, t.Name, t.Category, method, t.Description, tag)
	}
	return nil
}

func runInstalled(out io.Writer, reg *registry.Registry, asJSON bool) error {
	installed := reg.ListInstalled()
	if asJSON {
		data, _ := json.MarshalIndent(installed, "", "  ")
		fmt.Fprintln(out, string(data))
		return nil
	}

	if len(installed) == 0 {
		fmt.Fprintln(out, "Nenhuma ferramenta registrada como instalada no momento.")
		return nil
	}

	fmt.Fprintf(out, "%s %d ferramenta(s) instalada(s):\n\n", styleGreen.Render("✔"), len(installed))
	for _, id := range installed {
		t := catalog.FindTool(id)
		if t != nil {
			fmt.Fprintf(out, "%-14s %-24s [%s] %s\n", t.ID, t.Name, t.Category, t.Description)
		} else {
			fmt.Fprintf(out, "%-14s\n", id)
		}
	}
	return nil
}

func runInstall(r *runner.Runner, args []string) error {
	tools, err := catalog.ResolveDependencies(args)
	if err != nil {
		return err
	}

	explicitSet := make(map[string]bool)
	expanded, _ := catalog.Expand(args)
	for _, id := range expanded {
		explicitSet[id] = true
	}

	var eligible []catalog.Tool
	for _, t := range tools {
		if !t.IsSupportedOnCurrentOS() {
			continue
		}
		// Se foi adicionada apenas como dependência e já está satisfeita no sistema, pula
		if !explicitSet[t.ID] && r.IsDependencySatisfied(&t) {
			fmt.Printf("✔ Dependência '%s' (%s) já está disponível no sistema.\n", t.Name, t.ID)
			continue
		}
		eligible = append(eligible, t)
	}

	if len(eligible) == 0 {
		fmt.Println("Todas as ferramentas solicitadas ou dependências já estão instaladas.")
		return nil
	}

	if len(eligible) > 1 {
		fmt.Printf("Plano de instalação (%d itens em ordem de dependência):\n", len(eligible))
		for i, t := range eligible {
			fmt.Printf("  %d. %s (%s)\n", i+1, t.Name, t.ID)
		}
		fmt.Println()
	}

	ok, fail := r.RunBatchInstall(eligible)
	if fail > 0 && ok == 0 {
		return fmt.Errorf("todas as %d instalações falharam", fail)
	}
	return nil
}

func runUninstall(r *runner.Runner, args []string) error {
	expanded, err := catalog.Expand(args)
	if err != nil {
		return err
	}

	var tools []catalog.Tool
	for _, id := range expanded {
		t := catalog.FindTool(id)
		if t == nil {
			return fmt.Errorf("ferramenta não encontrada no catálogo: %s", id)
		}
		tools = append(tools, *t)
	}

	ok, fail := r.RunBatchUninstall(tools)
	if fail > 0 && ok == 0 {
		return fmt.Errorf("todas as %d desinstalações falharam", fail)
	}
	return nil
}

func runUpdate(r *runner.Runner, args []string) error {
	expanded, err := catalog.Expand(args)
	if err != nil {
		return err
	}

	var tools []catalog.Tool
	for _, id := range expanded {
		t := catalog.FindTool(id)
		if t == nil {
			return fmt.Errorf("ferramenta não encontrada no catálogo: %s", id)
		}
		if !t.IsSupportedOnCurrentOS() {
			continue
		}
		tools = append(tools, *t)
	}

	ok, fail := r.RunBatchUpdate(tools)
	if fail > 0 && ok == 0 {
		return fmt.Errorf("todas as %d atualizações falharam", fail)
	}
	return nil
}
