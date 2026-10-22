package registry

import (
	"encoding/json"
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
	Installed map[string]InstalledItem `json:"installed"`
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

	data, err := os.ReadFile(r.filePath)
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
	data, err := json.MarshalIndent(r.Installed, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(r.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(r.filePath, data, 0644)
}

func (r *Registry) IsInstalled(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.Installed[id]
	return exists
}

func (r *Registry) MarkInstalled(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Installed[id] = InstalledItem{
		Timestamp: time.Now().Unix(),
		OS:        runtime.GOOS,
	}
	return r.save()
}

func (r *Registry) MarkUninstalled(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Installed, id)
	return r.save()
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
