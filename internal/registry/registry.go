package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"
)

type InstalledItem struct {
	Timestamp int64  `json:"timestamp"`
	OS        string `json:"os"`
	Version   string `json:"version,omitempty"`
}

type Registry struct {
	mu        sync.RWMutex
	filePath  string
	dryRun    bool
	Installed map[string]InstalledItem `json:"installed"`
}

func (r *Registry) SetDryRun(dryRun bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dryRun = dryRun
}

func DefaultPath() string {
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, "multitool", "installed.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "multitool", "installed.json")
}

func New(customPath string) (*Registry, error) {
	target := customPath
	if target == "" {
		target = DefaultPath()
	}

	r := &Registry{
		filePath:  target,
		Installed: make(map[string]InstalledItem),
	}

	if err := r.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return r, nil
}

func (r *Registry) load() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.loadLocked()
}

// loadLocked é usado dentro da transação protegida pelo lock de processo.
func (r *Registry) loadLocked() error {
	data, err := readRegistryFile(r.filePath)
	if err != nil {
		return err
	}

	// Suporte a compatibilidade: pode ser map[string]int64 legado ou map[string]InstalledItem
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	r.Installed = make(map[string]InstalledItem)
	for k, v := range raw {
		switch val := v.(type) {
		case float64:
			r.Installed[k] = InstalledItem{Timestamp: int64(val), OS: runtime.GOOS}
		case map[string]interface{}:
			item := InstalledItem{OS: runtime.GOOS}
			if ts, ok := val["timestamp"].(float64); ok {
				item.Timestamp = int64(ts)
			}
			if o, ok := val["os"].(string); ok {
				item.OS = o
			}
			if ver, ok := val["version"].(string); ok {
				item.Version = ver
			}
			r.Installed[k] = item
		}
	}
	return nil
}

func (r *Registry) save() error {
	if r.dryRun {
		return nil
	}
	data, err := json.MarshalIndent(r.Installed, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(r.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return writeAtomic(r.filePath, data)
}

func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".installed-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return replaceRegistryFile(name, path)
}

// mutate relê o estado após obter o lock para não perder alterações de outro
// processo que tenha carregado o mesmo registro em um instante diferente.
func (r *Registry) mutate(id string, item *InstalledItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.dryRun {
		if err := os.MkdirAll(filepath.Dir(r.filePath), 0755); err != nil {
			return err
		}
		unlock, err := lockRegistryFile(r.filePath + ".lock")
		if err != nil {
			return fmt.Errorf("lock do registro: %w", err)
		}
		defer unlock()
		if err := r.loadLocked(); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			r.Installed = make(map[string]InstalledItem)
		}
	}
	previous, existed := r.Installed[id]
	if item == nil {
		delete(r.Installed, id)
	} else {
		r.Installed[id] = *item
	}
	if err := r.save(); err != nil {
		if existed {
			r.Installed[id] = previous
		} else {
			delete(r.Installed, id)
		}
		return fmt.Errorf("gravação do registro: %w", err)
	}
	return nil
}

func (r *Registry) IsInstalled(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.Installed[id]
	return exists
}

func (r *Registry) MarkInstalled(id string) error {
	item := InstalledItem{
		Timestamp: time.Now().Unix(),
		OS:        runtime.GOOS,
	}
	return r.mutate(id, &item)
}

func (r *Registry) MarkUninstalled(id string) error {
	return r.mutate(id, nil)
}

func (r *Registry) ListInstalled() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]string, 0, len(r.Installed))
	for k := range r.Installed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
