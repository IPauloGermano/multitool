package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/IPauloGermano/multitool/internal/catalog"
	"github.com/IPauloGermano/multitool/internal/system"
)

type DoctorStatus string

const (
	DoctorOK   DoctorStatus = "ok"
	DoctorWarn DoctorStatus = "warn"
	DoctorFail DoctorStatus = "fail"
)

type DoctorCheck struct {
	Name   string       `json:"name"`
	Status DoctorStatus `json:"status"`
	Detail string       `json:"detail"`
	Hint   string       `json:"hint,omitempty"`
}

type DoctorSummary struct {
	OK   int `json:"ok"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
}

type DoctorReport struct {
	OS             string        `json:"os"`
	Arch           string        `json:"arch"`
	PrimaryManager string        `json:"primary_manager"`
	Runtimes       []DoctorCheck `json:"runtimes"`
	Tools          []DoctorCheck `json:"tools"`
	Skills         []DoctorCheck `json:"skills"`
	Summary        DoctorSummary `json:"summary"`
}

func (rep *DoctorReport) tally(checks []DoctorCheck) {
	for _, c := range checks {
		switch c.Status {
		case DoctorOK:
			rep.Summary.OK++
		case DoctorWarn:
			rep.Summary.Warn++
		case DoctorFail:
			rep.Summary.Fail++
		}
	}
}

// Doctor audita runtimes base, integridade das ferramentas registradas no installed.json
// e validade dos symlinks/diretórios de skills de IA.
func (r *Runner) Doctor() DoctorReport {
	rep := DoctorReport{
		OS:             string(r.sysInfo.OS),
		Arch:           r.sysInfo.Arch,
		PrimaryManager: string(r.sysInfo.PrimaryManager),
	}

	rep.Runtimes = r.doctorRuntimes()
	rep.Tools, rep.Skills = r.doctorInstalledAndSkills()

	rep.tally(rep.Runtimes)
	rep.tally(rep.Tools)
	rep.tally(rep.Skills)
	return rep
}

func (r *Runner) doctorRuntimes() []DoctorCheck {
	var checks []DoctorCheck

	// 1. Gerenciador nativo primário
	primaryCmd := ""
	switch r.sysInfo.OS {
	case system.OSWindows:
		primaryCmd = "winget"
	case system.OSDarwin:
		primaryCmd = "brew"
	case system.OSLinux:
		switch r.sysInfo.PrimaryManager {
		case system.MgrApt:
			primaryCmd = "apt-get"
		case system.MgrDnf:
			primaryCmd = "dnf"
		case system.MgrPacman:
			primaryCmd = "pacman"
		case system.MgrZypper:
			primaryCmd = "zypper"
		case system.MgrEopkg:
			primaryCmd = "eopkg"
		case system.MgrBrew:
			primaryCmd = "brew"
		}
	}

	if primaryCmd == "" {
		checks = append(checks, DoctorCheck{
			Name:   "gerenciador-nativo",
			Status: DoctorFail,
			Detail: "nenhum gerenciador de pacotes nativo detectado",
			Hint:   "Instale um gerenciador suportado (winget, brew, apt, dnf, pacman, zypper ou eopkg)",
		})
	} else if p, err := r.executor.LookPath(primaryCmd); err == nil {
		checks = append(checks, DoctorCheck{
			Name:   primaryCmd,
			Status: DoctorOK,
			Detail: p,
		})
	} else {
		st := DoctorFail
		if r.sysInfo.OS == system.OSDarwin {
			st = DoctorWarn
		}
		checks = append(checks, DoctorCheck{
			Name:   primaryCmd,
			Status: st,
			Detail: "não encontrado no PATH",
			Hint:   fmt.Sprintf("Garanta que '%s' esteja instalado e acessível no PATH", primaryCmd),
		})
	}

	// 2. Runtimes auxiliares
	curlCmd := "curl"
	if r.sysInfo.OS == system.OSWindows {
		curlCmd = "curl.exe"
	}
	aux := []string{"git", curlCmd, "npm", "pipx", "cargo"}
	if r.sysInfo.OS == system.OSLinux {
		aux = append(aux, "flatpak")
	}

	for _, cmd := range aux {
		if cmd == primaryCmd {
			continue
		}
		if p, err := r.executor.LookPath(cmd); err == nil {
			checks = append(checks, DoctorCheck{
				Name:   cmd,
				Status: DoctorOK,
				Detail: p,
			})
		} else {
			checks = append(checks, DoctorCheck{
				Name:   cmd,
				Status: DoctorWarn,
				Detail: "ausente no PATH (opcional; auto-provisionado sob demanda)",
			})
		}
	}

	// 3. Verifica diretório local de binários portáteis no PATH (se existir com arquivos)
	localBin := ""
	if r.sysInfo.OS == system.OSWindows {
		if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
			localBin = filepath.Join(localApp, "multitool", "bin")
		}
	} else if home, err := os.UserHomeDir(); err == nil && home != "" {
		localBin = filepath.Join(home, ".local", "bin")
	}
	if localBin != "" {
		if entries, err := os.ReadDir(localBin); err == nil && len(entries) > 0 {
			if dirInPath(localBin, os.Getenv("PATH"), r.sysInfo.OS == system.OSWindows) {
				checks = append(checks, DoctorCheck{
					Name:   "local-bin-path",
					Status: DoctorOK,
					Detail: fmt.Sprintf("%s está no PATH (%d binário(s))", localBin, len(entries)),
				})
			} else {
				checks = append(checks, DoctorCheck{
					Name:   "local-bin-path",
					Status: DoctorWarn,
					Detail: fmt.Sprintf("%s contém %d arquivo(s), mas não está no PATH", localBin, len(entries)),
					Hint:   fmt.Sprintf("Adicione '%s' à variável de ambiente PATH", localBin),
				})
			}
		}
	}

	return checks
}

func (r *Runner) doctorInstalledAndSkills() ([]DoctorCheck, []DoctorCheck) {
	var toolChecks []DoctorCheck
	var skillChecks []DoctorCheck

	installed := r.reg.ListInstalled()
	sort.Strings(installed)

	registeredSkills := 0
	for _, id := range installed {
		tool := catalog.FindTool(id)
		if tool == nil {
			toolChecks = append(toolChecks, DoctorCheck{
				Name:   id,
				Status: DoctorWarn,
				Detail: "registrada no installed.json, mas ausente no catálogo atual",
				Hint:   fmt.Sprintf("Execute 'multitool uninstall %s' para limpar o registro", id),
			})
			continue
		}

		target := r.targetForTool(*tool)
		if target == nil {
			toolChecks = append(toolChecks, DoctorCheck{
				Name:   tool.ID,
				Status: DoctorWarn,
				Detail: fmt.Sprintf("%s registrada sem alvo compatível para %s", tool.Name, r.sysInfo.OS),
			})
			continue
		}

		if isGitSkillTarget(tool.ID, target) {
			registeredSkills++
			skillChecks = append(skillChecks, r.doctorSkill(*tool))
			continue
		}

		switch target.Type {
		case catalog.TypeFlatpak:
			if !r.haveCommand("flatpak") {
				toolChecks = append(toolChecks, DoctorCheck{
					Name:   tool.ID,
					Status: DoctorFail,
					Detail: fmt.Sprintf("%s requer flatpak, mas flatpak não está no PATH", tool.Name),
					Hint:   fmt.Sprintf("Instale flatpak ou execute 'multitool uninstall %s'", tool.ID),
				})
			} else if r.execSilent("flatpak", "info", target.PackageID) == nil {
				toolChecks = append(toolChecks, DoctorCheck{
					Name:   tool.ID,
					Status: DoctorOK,
					Detail: fmt.Sprintf("flatpak (%s)", target.PackageID),
				})
			} else {
				toolChecks = append(toolChecks, DoctorCheck{
					Name:   tool.ID,
					Status: DoctorFail,
					Detail: fmt.Sprintf("aplicativo flatpak '%s' não encontrado", target.PackageID),
					Hint:   fmt.Sprintf("Execute 'multitool install %s --force' ou 'multitool uninstall %s'", tool.ID, tool.ID),
				})
			}
		default:
			if tool.ID == "temurin" {
				if p, err := r.executor.LookPath("java"); err == nil {
					toolChecks = append(toolChecks, DoctorCheck{
						Name:   tool.ID,
						Status: DoctorOK,
						Detail: p,
					})
					continue
				}
			}
			if target.Binary == "" {
				toolChecks = append(toolChecks, DoctorCheck{
					Name:   tool.ID,
					Status: DoctorOK,
					Detail: fmt.Sprintf("%s registrada (sem binário verificável no PATH)", tool.Name),
				})
				continue
			}
			if binPath := r.findTargetBinaryPath(*tool, target); binPath != "" {
				toolChecks = append(toolChecks, DoctorCheck{
					Name:   tool.ID,
					Status: DoctorOK,
					Detail: binPath,
				})
			} else {
				toolChecks = append(toolChecks, DoctorCheck{
					Name:   tool.ID,
					Status: DoctorFail,
					Detail: fmt.Sprintf("binário '%s' não encontrado no PATH ou diretórios padrão", target.Binary),
					Hint:   fmt.Sprintf("Execute 'multitool install %s --force' ou 'multitool uninstall %s'", tool.ID, tool.ID),
				})
			}
		}
	}

	if registeredSkills > 0 && len(r.sysInfo.SkillDirs) == 0 {
		skillChecks = append(skillChecks, DoctorCheck{
			Name:   "agentes-ia",
			Status: DoctorWarn,
			Detail: "nenhum diretório de agente detectado (~/.claude, ~/.config/opencode, ~/.gemini, ~/.codex)",
			Hint:   "Instale um agente suportado e execute 'multitool sync' para criar os links",
		})
	}

	// Audita symlinks quebrados nos diretórios de skills detectados
	for _, targetDir := range r.sysInfo.SkillDirs {
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			linkPath := filepath.Join(targetDir, entry.Name())
			if _, rerr := os.Readlink(linkPath); rerr == nil {
				if _, serr := os.Stat(linkPath); serr != nil {
					skillChecks = append(skillChecks, DoctorCheck{
						Name:   filepath.Join(filepath.Base(filepath.Dir(targetDir)), "skills", entry.Name()),
						Status: DoctorFail,
						Detail: fmt.Sprintf("symlink/junção quebrado em %s", linkPath),
						Hint:   "Execute 'multitool sync' ou remova o link órfão",
					})
				}
			}
		}
	}

	return toolChecks, skillChecks
}

func (r *Runner) doctorSkill(tool catalog.Tool) DoctorCheck {
	skillDir := r.getSkillDir(tool.ID)
	fi, err := os.Stat(skillDir)
	if err != nil || !fi.IsDir() {
		return DoctorCheck{
			Name:   tool.ID,
			Status: DoctorFail,
			Detail: fmt.Sprintf("diretório da skill ausente (%s)", skillDir),
			Hint:   fmt.Sprintf("Execute 'multitool install %s --force'", tool.ID),
		}
	}

	if len(r.sysInfo.SkillDirs) == 0 {
		return DoctorCheck{
			Name:   tool.ID,
			Status: DoctorOK,
			Detail: fmt.Sprintf("clonada em %s", skillDir),
		}
	}

	discovered := FindSkillsInDir(skillDir)
	missingLinks := 0
	brokenLinks := 0
	validLinks := 0

	for _, targetDir := range r.sysInfo.SkillDirs {
		for skillName := range discovered {
			dest := filepath.Join(targetDir, skillName)
			if _, lerr := os.Lstat(dest); lerr != nil {
				missingLinks++
				continue
			}
			if _, serr := os.Stat(dest); serr != nil {
				brokenLinks++
				continue
			}
			validLinks++
		}
	}

	if brokenLinks > 0 {
		return DoctorCheck{
			Name:   tool.ID,
			Status: DoctorFail,
			Detail: fmt.Sprintf("%d link(s) quebrado(s) para %d agente(s)", brokenLinks, len(r.sysInfo.SkillDirs)),
			Hint:   "Execute 'multitool sync' para reparar os links",
		}
	}
	if missingLinks > 0 {
		return DoctorCheck{
			Name:   tool.ID,
			Status: DoctorWarn,
			Detail: fmt.Sprintf("%d link(s) ausente(s) (%d válido(s))", missingLinks, validLinks),
			Hint:   "Execute 'multitool sync' para re-linkar skills nos agentes detectados",
		}
	}
	return DoctorCheck{
		Name:   tool.ID,
		Status: DoctorOK,
		Detail: fmt.Sprintf("%d link(s) íntegro(s) em %d destino(s)", validLinks, len(r.sysInfo.SkillDirs)),
	}
}

func dirInPath(dir, pathEnv string, caseInsensitive bool) bool {
	if dir == "" || pathEnv == "" {
		return false
	}
	want := filepath.Clean(dir)
	for _, entry := range filepath.SplitList(pathEnv) {
		if entry == "" {
			continue
		}
		cur := filepath.Clean(entry)
		if caseInsensitive {
			if strings.EqualFold(cur, want) {
				return true
			}
		} else if cur == want {
			return true
		}
	}
	return false
}
