package runner

import (
	"fmt"
	"github.com/IPauloGermano/multitool/internal/catalog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// -------------------------------------------------------------
// LOG DE AUDITORIA COM ROTAÇÃO (512 KB)
// -------------------------------------------------------------
var (
	logMu       sync.Mutex
	customLog   string
	maxLogBytes int64 = 512 * 1024
)

func DefaultLogPath() string {
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, "multitool", "multitool.log")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "multitool", "multitool.log")
}

func SetLogPath(p string) {
	logMu.Lock()
	defer logMu.Unlock()
	customLog = p
}

func getLogPath() string {
	if customLog != "" {
		return customLog
	}
	return DefaultLogPath()
}

func logGira(path string) {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() <= maxLogBytes {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	keep := data[len(data)-int(maxLogBytes):]
	_ = os.WriteFile(path, keep, 0644)
}

func LogOp(format string, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()

	p := getLogPath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	logGira(p)

	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(f, "[%s] %s\n", timestamp, msg)
}

// -------------------------------------------------------------
// SINCRONIZAÇÃO E DETECÇÃO DE FERRAMENTAS DO HOST
// -------------------------------------------------------------
type SyncResult struct {
	ToolID   string `json:"id"`
	ToolName string `json:"name"`
	Binary   string `json:"binary,omitempty"`
	Method   string `json:"method"`
}

func (r *Runner) SyncInstalled() ([]SyncResult, int, error) {
	supported := catalog.SupportedCatalog()
	var newlyDetected []SyncResult
	alreadyCount := 0

	for _, tool := range supported {
		if r.reg.IsInstalled(tool.ID) {
			alreadyCount++
			continue
		}

		target := r.targetForTool(tool)
		if target == nil {
			continue
		}

		detected := false
		detectMethod := ""

		if tool.ID == "temurin" && r.haveCommand("java") {
			detected = true
			detectMethod = "PATH (java)"
		} else if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" && target.Binary != "" && (func() bool {
			_, err1 := os.Stat(filepath.Join(localApp, "multitool", "bin", target.Binary))
			_, err2 := os.Stat(filepath.Join(localApp, "multitool", "bin", tool.ID+".bat"))
			return err1 == nil || err2 == nil
		})() {
			detected = true
			detectMethod = "LOCALAPPDATA/multitool/bin"
		} else if target.Binary != "" && r.haveCommand(target.Binary) {
			detected = true
			detectMethod = fmt.Sprintf("PATH (%s)", target.Binary)
		} else if binPath := r.findTargetBinaryPath(tool, target); binPath != "" {
			detected = true
			detectMethod = fmt.Sprintf("disco (%s)", filepath.Dir(binPath))
		} else if target.Type == catalog.TypeSkill {
			skillDir := r.getSkillDir(tool.ID)
			if fi, err := os.Stat(skillDir); err == nil && fi.IsDir() {
				detected = true
				detectMethod = "diretório local de skills"
			}
		}

		if detected {
			newlyDetected = append(newlyDetected, SyncResult{
				ToolID:   tool.ID,
				ToolName: tool.Name,
				Binary:   target.Binary,
				Method:   detectMethod,
			})

			if !r.dryRun {
				_ = r.reg.MarkInstalled(tool.ID)
			}
		}
	}

	if !r.dryRun {
		r.RelinkAllSkills()
	}

	return newlyDetected, alreadyCount, nil
}

func (r *Runner) RelinkAllSkills() int {
	if r.dryRun {
		return 0
	}
	targets := r.sysInfo.SkillDirs
	if len(targets) == 0 {
		return 0
	}

	relinked := 0
	skillsBase := filepath.Dir(r.getSkillDir("dummy"))
	entries, err := os.ReadDir(skillsBase)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		if entry.IsDir() {
			skillPath := filepath.Join(skillsBase, entry.Name())
			n, _ := LinkSkillToTargets(skillPath, targets)
			relinked += n
		}
	}
	return relinked
}
