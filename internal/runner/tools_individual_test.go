package runner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/registry"
	"github.com/IPauloGermano/multitool/internal/system"
)

type individualToolExecutor struct {
	*MockExecutor
	installedFlatpaks map[string]bool
}

func newIndividualToolExecutor() *individualToolExecutor {
	return &individualToolExecutor{
		MockExecutor:      &MockExecutor{},
		installedFlatpaks: make(map[string]bool),
	}
}

func (e *individualToolExecutor) Execute(name string, args ...string) error {
	// Simula estado real de pacotes Flatpak para evitar fast-path falso positivo
	if name == "flatpak" {
		if len(args) >= 2 && args[0] == "info" {
			pkg := args[1]
			if !e.installedFlatpaks[pkg] {
				return errors.New("flatpak package not installed")
			}
			return nil
		}
		for i, a := range args {
			if a == "install" && len(args) > 0 {
				e.installedFlatpaks[args[len(args)-1]] = true
				break
			}
			if a == "uninstall" && i < len(args)-1 {
				delete(e.installedFlatpaks, args[len(args)-1])
				break
			}
		}
	}

	if err := e.MockExecutor.Execute(name, args...); err != nil {
		return err
	}

	// Simula clone Git criando diretório e SKILL.md para validar symlinks/junções e pull no Update
	if name == "git" {
		for i, a := range args {
			if a == "clone" && len(args) > i+1 {
				dest := args[len(args)-1]
				if err := os.MkdirAll(dest, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dest, "SKILL.md"), []byte("---\nname: mock\n---\n"), 0644); err != nil {
					return err
				}
				break
			}
		}
	}

	// Simula extração tar.exe do WABT no Windows
	if name == "tar.exe" {
		for i, a := range args {
			if a == "-C" && i+1 < len(args) {
				binDir := filepath.Join(args[i+1], "bin")
				if err := os.MkdirAll(binDir, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(binDir, "wat2wasm.exe"), []byte("stub"), 0755); err != nil {
					return err
				}
				break
			}
		}
	}

	return nil
}

func classifyRunnerKind(tool catalog.Tool) string {
	if tool.Category == "Skills" {
		return "skill"
	}
	if tool.Category == "Desktop" {
		return "app"
	}
	return "tool"
}

func commandsContain(executed [][]string, substrs ...string) bool {
	for _, cmd := range executed {
		joined := strings.Join(cmd, " ")
		allMatch := true
		for _, s := range substrs {
			if !strings.Contains(joined, s) {
				allMatch = false
				break
			}
		}
		if allMatch {
			return true
		}
	}
	return false
}

func TestIndividualToolsLifecycle(t *testing.T) {
	tools := catalog.DefaultCatalog()
	if len(tools) == 0 {
		t.Fatal("catálogo padrão vazio")
	}

	osConfigs := []struct {
		name system.OSType
		arch string
		mgr  system.PkgManager
	}{
		{name: system.OSWindows, arch: "amd64", mgr: system.MgrWinget},
		{name: system.OSLinux, arch: "amd64", mgr: system.MgrApt},
		{name: system.OSDarwin, arch: "arm64", mgr: system.MgrBrew},
	}

	for _, tool := range tools {
		tool := tool
		subName := classifyRunnerKind(tool) + "/" + tool.ID
		t.Run(subName, func(t *testing.T) {
			for _, cfg := range osConfigs {
				cfg := cfg
				target := tool.TargetForOS(string(cfg.name))

				t.Run(string(cfg.name), func(t *testing.T) {
					root := t.TempDir()
					t.Setenv("HOME", root)
					t.Setenv("USERPROFILE", root)
					t.Setenv("LOCALAPPDATA", filepath.Join(root, "AppData", "Local"))
					t.Setenv("APPDATA", filepath.Join(root, "AppData", "Roaming"))
					t.Setenv("ProgramFiles", filepath.Join(root, "Program Files"))
					t.Setenv("ProgramFiles(x86)", filepath.Join(root, "Program Files (x86)"))
					t.Setenv("WINDIR", filepath.Join(root, "Windows"))

					skillDir := filepath.Join(root, ".claude", "skills")

					newIsolatedRunner := func(exec CommandExecutor, dryRun bool) (*Runner, *registry.Registry) {
						reg, err := registry.New(filepath.Join(t.TempDir(), "installed.json"))
						if err != nil {
							t.Fatalf("erro ao criar registro: %v", err)
						}
						r := NewWithExecutor(reg, exec, dryRun)
						info := *r.sysInfo
						r.sysInfo = &info
						r.sysInfo.OS = cfg.name
						r.sysInfo.Arch = cfg.arch
						r.sysInfo.PrimaryManager = cfg.mgr
						r.sysInfo.NeedsSudo = false
						r.sysInfo.SkillDirs = []string{skillDir}
						r.SetVerbose(true)
						return r, reg
					}

					// Caso 1: SO não suportado -> Install, Update e Uninstall devem retornar erro sem executar nada
					if target == nil {
						exec := newIndividualToolExecutor()
						r, reg := newIsolatedRunner(exec, false)

						if err := r.Install(tool); err == nil || !strings.Contains(err.Error(), "não é compatível") {
							t.Errorf("Install em SO não suportado (%s) deveria falhar, obteve: %v", cfg.name, err)
						}
						if err := r.Update(tool); err == nil || !strings.Contains(err.Error(), "não é compatível") {
							t.Errorf("Update em SO não suportado (%s) deveria falhar, obteve: %v", cfg.name, err)
						}
						if err := r.Uninstall(tool); err == nil || !strings.Contains(err.Error(), "não é compatível") {
							t.Errorf("Uninstall em SO não suportado (%s) deveria falhar, obteve: %v", cfg.name, err)
						}
						if len(exec.executed) != 0 {
							t.Errorf("SO não suportado não deveria executar comandos, executou: %v", exec.executed)
						}
						if reg.IsInstalled(tool.ID) {
							t.Errorf("%q não deveria estar marcado no registro em SO não suportado", tool.ID)
						}
						return
					}

					// Caso 2: Dry-Run (Install -> Update -> Uninstall)
					dryExec := newIndividualToolExecutor()
					rDry, regDry := newIsolatedRunner(dryExec, true)
					if err := rDry.Install(tool); err != nil {
						t.Fatalf("Install dry-run falhou: %v", err)
					}
					if !regDry.IsInstalled(tool.ID) {
						t.Errorf("Install dry-run deveria registrar %q", tool.ID)
					}
					if err := rDry.Update(tool); err != nil {
						t.Fatalf("Update dry-run falhou: %v", err)
					}
					if err := rDry.Uninstall(tool); err != nil {
						t.Fatalf("Uninstall dry-run falhou: %v", err)
					}
					if regDry.IsInstalled(tool.ID) {
						t.Errorf("Uninstall dry-run deveria remover %q do registro", tool.ID)
					}
					if len(dryExec.executed) != 0 {
						t.Errorf("Dry-run não deveria executar comandos externos, executou: %v", dryExec.executed)
					}

					// Caso 3: Execução mockada real (Install -> Update -> Uninstall) para 100% das ferramentas
					exec := newIndividualToolExecutor()
					exec.missing = map[string]bool{target.Binary: true}
					if tool.ID == "temurin" {
						exec.missing["java"] = true
						exec.missing["java.exe"] = true
					}
					r, reg := newIsolatedRunner(exec, false)

					// 3a. Install
					if err := r.Install(tool); err != nil {
						t.Fatalf("Install(%s/%s) falhou: %v (comandos: %v)", tool.ID, cfg.name, err, exec.executed)
					}
					if !reg.IsInstalled(tool.ID) {
						t.Errorf("Install(%s/%s) não registrou ferramenta em installed.json", tool.ID, cfg.name)
					}
					assertInstallCommands(t, r, tool, cfg.name, target, exec.executed, skillDir)

					// Marca binário como presente após instalação para simular estado real pós-install
					delete(exec.missing, target.Binary)
					if exec.present == nil {
						exec.present = make(map[string]bool)
					}
					exec.present[target.Binary] = true
					r.cacheMu.Lock()
					r.cmdCache = make(map[string]bool)
					r.cacheMu.Unlock()

					// 3b. Update
					exec.executed = nil
					if err := r.Update(tool); err != nil {
						t.Fatalf("Update(%s/%s) falhou: %v (comandos: %v)", tool.ID, cfg.name, err, exec.executed)
					}
					assertUpdateCommands(t, tool, cfg.name, target, exec.executed)

					// 3c. Uninstall
					exec.executed = nil
					if err := r.Uninstall(tool); err != nil {
						t.Fatalf("Uninstall(%s/%s) falhou: %v (comandos: %v)", tool.ID, cfg.name, err, exec.executed)
					}
					if reg.IsInstalled(tool.ID) {
						t.Errorf("Uninstall(%s/%s) não removeu ferramenta do registro", tool.ID, cfg.name)
					}
					assertUninstallOutcome(t, r, tool, cfg.name, target, exec.executed, skillDir)
				})
			}
		})
	}
}

func assertInstallCommands(t *testing.T, r *Runner, tool catalog.Tool, osName system.OSType, target *catalog.Target, executed [][]string, skillDir string) {
	t.Helper()
	if len(executed) == 0 {
		t.Errorf("Install(%s/%s) não executou nenhum comando", tool.ID, osName)
		return
	}

	switch target.Type {
	case catalog.TypeWinget:
		if !commandsContain(executed, "winget", "install", "--id", target.PackageID) {
			t.Errorf("esperava winget install --id %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeNative:
		if !commandsContain(executed, "apt-get", "install", "-y", target.PackageID) {
			t.Errorf("esperava apt-get install -y %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeNpm:
		if !commandsContain(executed, "npm", "install", "-g", target.PackageID) {
			t.Errorf("esperava npm install -g %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypePipx:
		if !commandsContain(executed, "pipx", "install", target.PackageID) {
			t.Errorf("esperava pipx install %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeCargo:
		if !commandsContain(executed, "cargo", "install", target.PackageID) {
			t.Errorf("esperava cargo install %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeFlatpak:
		if !commandsContain(executed, "flatpak", "install", "flathub", target.PackageID) {
			t.Errorf("esperava flatpak install flathub %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeBrew:
		if !commandsContain(executed, "brew", "install", target.PackageID) {
			t.Errorf("esperava brew install %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeSkill:
		assertGitSkillInstalled(t, r, tool, target, executed, skillDir)
	case catalog.TypeDirect:
		if isDirectWindowsBinary(tool.ID) {
			if !commandsContain(executed, "curl.exe", "https://") {
				t.Errorf("esperava download via curl.exe HTTPS para %s, obteve: %v", tool.ID, executed)
			}
		} else {
			assertGitSkillInstalled(t, r, tool, target, executed, skillDir)
		}
	case catalog.TypeScript:
		switch tool.ID {
		case "flathub":
			if !commandsContain(executed, "flatpak", "remote-add", "flathub") {
				t.Errorf("esperava flatpak remote-add flathub, obteve: %v", executed)
			}
		case "ax":
			if !commandsContain(executed, "go", "install", "github.com/google/ax/cmd/ax@latest") {
				t.Errorf("esperava go install para ax, obteve: %v", executed)
			}
		case "opengym", "openstock", "openmaicapp", "craftlauncher", "gethomepage", "keep":
			if !commandsContain(executed, "git", "clone", "https://github.com/"+target.PackageID) {
				t.Errorf("esperava git clone para %s, obteve: %v", tool.ID, executed)
			}
		default:
			if !commandsContain(executed, "curl", "https://") {
				t.Errorf("esperava download via curl HTTPS para script %s, obteve: %v", tool.ID, executed)
			}
		}
	}
}

func assertGitSkillInstalled(t *testing.T, r *Runner, tool catalog.Tool, target *catalog.Target, executed [][]string, skillDir string) {
	t.Helper()
	expectedURL := "https://github.com/" + target.PackageID + ".git"
	if !commandsContain(executed, "git", "clone", expectedURL) {
		t.Errorf("esperava git clone %s, obteve: %v", expectedURL, executed)
	}
	dest := r.getSkillDir(tool.ID)
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		t.Errorf("diretório da skill %s não criado em %s: %v", tool.ID, dest, err)
	}
	linkPath := filepath.Join(skillDir, tool.ID)
	if _, err := os.Stat(filepath.Join(linkPath, "SKILL.md")); err != nil {
		t.Errorf("link da skill %s não acessível em %s: %v", tool.ID, linkPath, err)
	}
}

func assertUpdateCommands(t *testing.T, tool catalog.Tool, osName system.OSType, target *catalog.Target, executed [][]string) {
	t.Helper()
	if len(executed) == 0 {
		t.Errorf("Update(%s/%s) não executou nenhum comando", tool.ID, osName)
		return
	}

	switch target.Type {
	case catalog.TypeWinget:
		if !commandsContain(executed, "winget", "upgrade", "--id", target.PackageID) {
			t.Errorf("esperava winget upgrade --id %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeNative:
		if !commandsContain(executed, "apt-get", "install", "--only-upgrade", target.PackageID) {
			t.Errorf("esperava apt-get --only-upgrade %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeNpm:
		if !commandsContain(executed, "npm", "update", "-g", target.PackageID) {
			t.Errorf("esperava npm update -g %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypePipx:
		if strings.HasPrefix(target.PackageID, "git+") || strings.HasPrefix(target.PackageID, "http") {
			if !commandsContain(executed, "pipx", "install", "--force", target.PackageID) {
				t.Errorf("esperava pipx install --force %s, obteve: %v", target.PackageID, executed)
			}
		} else {
			if !commandsContain(executed, "pipx", "upgrade", target.PackageID) {
				t.Errorf("esperava pipx upgrade %s, obteve: %v", target.PackageID, executed)
			}
		}
	case catalog.TypeCargo:
		if !commandsContain(executed, "cargo", "install", target.PackageID) {
			t.Errorf("esperava cargo install %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeFlatpak:
		if !commandsContain(executed, "flatpak", "update", target.PackageID) {
			t.Errorf("esperava flatpak update %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeBrew:
		if !commandsContain(executed, "brew", "upgrade", target.PackageID) {
			t.Errorf("esperava brew upgrade %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeSkill:
		if !commandsContain(executed, "git", "pull", "--ff-only") {
			t.Errorf("esperava git pull --ff-only no update da skill %s, obteve: %v", tool.ID, executed)
		}
	case catalog.TypeDirect:
		if isDirectWindowsBinary(tool.ID) {
			if !commandsContain(executed, "curl.exe", "https://") {
				t.Errorf("esperava download curl.exe no update de %s, obteve: %v", tool.ID, executed)
			}
		} else if !commandsContain(executed, "git", "pull", "--ff-only") {
			t.Errorf("esperava git pull --ff-only no update de %s, obteve: %v", tool.ID, executed)
		}
	case catalog.TypeScript:
		switch tool.ID {
		case "flathub":
			if !commandsContain(executed, "flatpak", "remote-add", "flathub") {
				t.Errorf("esperava flatpak remote-add no update de flathub, obteve: %v", executed)
			}
		case "ax":
			if !commandsContain(executed, "go", "install", "github.com/google/ax/cmd/ax@latest") {
				t.Errorf("esperava go install no update de ax, obteve: %v", executed)
			}
		case "opengym", "openstock", "openmaicapp", "craftlauncher", "gethomepage", "keep":
			if !commandsContain(executed, "git", "pull", "--ff-only") {
				t.Errorf("esperava git pull --ff-only no update de %s, obteve: %v", tool.ID, executed)
			}
		default:
			if !commandsContain(executed, "curl", "https://") {
				t.Errorf("esperava download curl no update de %s, obteve: %v", tool.ID, executed)
			}
		}
	}
}

func assertUninstallOutcome(t *testing.T, r *Runner, tool catalog.Tool, osName system.OSType, target *catalog.Target, executed [][]string, skillDir string) {
	t.Helper()

	switch target.Type {
	case catalog.TypeWinget:
		if !commandsContain(executed, "winget", "uninstall", "--id", target.PackageID) {
			t.Errorf("esperava winget uninstall --id %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeNative:
		if !commandsContain(executed, "apt-get", "remove", "-y", target.PackageID) {
			t.Errorf("esperava apt-get remove -y %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeNpm:
		if !commandsContain(executed, "npm", "uninstall", "-g", target.PackageID) {
			t.Errorf("esperava npm uninstall -g %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypePipx:
		expectedPkg := target.PackageID
		if strings.HasPrefix(expectedPkg, "git+") || strings.HasPrefix(expectedPkg, "http") {
			expectedPkg = strings.TrimSuffix(target.Binary, ".exe")
		}
		if !commandsContain(executed, "pipx", "uninstall", expectedPkg) {
			t.Errorf("esperava pipx uninstall %s, obteve: %v", expectedPkg, executed)
		}
	case catalog.TypeCargo:
		if !commandsContain(executed, "cargo", "uninstall", target.PackageID) {
			t.Errorf("esperava cargo uninstall %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeFlatpak:
		if !commandsContain(executed, "flatpak", "uninstall", target.PackageID) {
			t.Errorf("esperava flatpak uninstall %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeBrew:
		if !commandsContain(executed, "brew", "uninstall", target.PackageID) {
			t.Errorf("esperava brew uninstall %s, obteve: %v", target.PackageID, executed)
		}
	case catalog.TypeSkill:
		assertGitSkillUninstalled(t, r, tool, skillDir)
	case catalog.TypeDirect:
		if !isDirectWindowsBinary(tool.ID) {
			assertGitSkillUninstalled(t, r, tool, skillDir)
		} else {
			localApp := os.Getenv("LOCALAPPDATA")
			switch tool.ID {
			case "wabt":
				if _, err := os.Stat(filepath.Join(localApp, "multitool", "bin", "wat2wasm.exe")); !os.IsNotExist(err) {
					t.Errorf("wat2wasm.exe deveria ter sido removido no uninstall de wabt")
				}
			case "apktool":
				if _, err := os.Stat(filepath.Join(localApp, "multitool", "bin", "apktool.bat")); !os.IsNotExist(err) {
					t.Errorf("apktool.bat deveria ter sido removido no uninstall de apktool")
				}
			case "ohmypi":
				if _, err := os.Stat(filepath.Join(localApp, "multitool", "bin", "omp.exe")); !os.IsNotExist(err) {
					t.Errorf("omp.exe deveria ter sido removido no uninstall de ohmypi")
				}
			case "goose":
				if _, err := os.Stat(filepath.Join(localApp, "multitool", "goose")); !os.IsNotExist(err) {
					t.Errorf("diretório goose deveria ter sido removido no uninstall de goose")
				}
			case "atkinson", "sourcesans3", "sourceserif4":
				if _, err := os.Stat(filepath.Join(localApp, "multitool", "fonts", tool.ID)); !os.IsNotExist(err) {
					t.Errorf("diretório de fontes %s deveria ter sido removido no uninstall", tool.ID)
				}
			}
		}
	case catalog.TypeScript:
		switch tool.ID {
		case "rustup":
			if !commandsContain(executed, "rustup", "self", "uninstall") {
				t.Errorf("esperava rustup self uninstall -y, obteve: %v", executed)
			}
		case "flathub":
			if !commandsContain(executed, "flatpak", "remote-delete", "flathub") {
				t.Errorf("esperava flatpak remote-delete flathub, obteve: %v", executed)
			}
		case "antigravity":
			if !commandsContain(executed, "apt-get", "remove", "-y", "antigravity") {
				t.Errorf("esperava apt-get remove antigravity, obteve: %v", executed)
			}
		case "dockerdesktop":
			if !commandsContain(executed, "apt-get", "remove", "-y", "docker-desktop") {
				t.Errorf("esperava apt-get remove docker-desktop, obteve: %v", executed)
			}
		case "warp":
			if !commandsContain(executed, "apt-get", "remove", "-y", "warp-terminal") {
				t.Errorf("esperava apt-get remove warp-terminal, obteve: %v", executed)
			}
		case "waydroid":
			if !commandsContain(executed, "apt-get", "remove", "-y", "waydroid") {
				t.Errorf("esperava apt-get remove waydroid, obteve: %v", executed)
			}
		case "opengym", "openstock", "openmaicapp", "craftlauncher", "gethomepage", "keep":
			home := os.Getenv("HOME")
			if _, err := os.Stat(filepath.Join(home, ".local", "share", "apps", tool.ID)); !os.IsNotExist(err) {
				t.Errorf("diretório de app %s deveria ter sido removido no uninstall", tool.ID)
			}
		}
	}
}

func assertGitSkillUninstalled(t *testing.T, r *Runner, tool catalog.Tool, skillDir string) {
	t.Helper()
	dest := r.getSkillDir(tool.ID)
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("diretório da skill %s ainda existe após uninstall: %s", tool.ID, dest)
	}
	linkPath := filepath.Join(skillDir, tool.ID)
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Errorf("link da skill %s ainda existe após uninstall: %s", tool.ID, linkPath)
	}
}
