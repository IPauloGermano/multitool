package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRegistryStaleInstancesMerge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installed.json")
	a, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.MarkInstalled("first"); err != nil {
		t.Fatal(err)
	}
	if err := b.MarkInstalled("second"); err != nil {
		t.Fatal(err)
	}
	if err := a.MarkUninstalled("first"); err != nil {
		t.Fatal(err)
	}
	got, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsInstalled("first") || !got.IsInstalled("second") {
		t.Fatalf("lost concurrent update: %v", got.ListInstalled())
	}
}

func TestRegistryProcessHelper(t *testing.T) {
	path := os.Getenv("MULTITOOL_TEST_REGISTRY")
	if path == "" {
		return
	}
	r, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if err := r.MarkInstalled(fmt.Sprintf("%s-%d", os.Getenv("MULTITOOL_TEST_WORKER"), i)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRegistryConcurrentProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installed.json")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			cmd := exec.CommandContext(ctx, exe, "-test.run=^TestRegistryProcessHelper$", "-test.timeout=15s")
			cmd.Env = append(os.Environ(), "MULTITOOL_TEST_REGISTRY="+path, fmt.Sprintf("MULTITOOL_TEST_WORKER=%d", worker))
			out, err := cmd.CombinedOutput()
			if err != nil {
				results <- fmt.Errorf("worker %d: %w: %s", worker, err, out)
			}
		}(i)
	}
	wg.Wait()
	close(results)
	for err := range results {
		t.Error(err)
	}
	r, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.ListInstalled()) != 32 {
		t.Fatalf("lost process writes: %d/32", len(r.ListInstalled()))
	}
}

func TestRegistryAtomicReaders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installed.json")
	r, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.MarkInstalled("initial"); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		for {
			select {
			case <-done:
				result <- nil
				return
			default:
			}
			data, err := readRegistryFile(path)
			if err != nil {
				result <- err
				return
			}
			if !json.Valid(data) {
				result <- fmt.Errorf("partial JSON observed: %q", data)
				return
			}
		}
	}()
	for i := 0; i < 30; i++ {
		if err := r.MarkInstalled(fmt.Sprint(i)); err != nil {
			t.Error(err)
			break
		}
	}
	close(done)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestRegistryFailedReplacementCleansTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "destination-directory")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(target, []byte(`{"valid":true}`)); err == nil {
		t.Fatal("expected replacement failure")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "destination-directory" {
		t.Fatalf("temporary files left behind: %v: %v", entries, err)
	}
}

func TestRegistryDryRunDoesNotCreateFiles(t *testing.T) {
	root := t.TempDir()
	r, err := New(filepath.Join(root, "missing", "installed.json"))
	if err != nil {
		t.Fatal(err)
	}
	r.SetDryRun(true)
	if err := r.MarkInstalled("simulated"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("dry-run wrote files: %v: %v", entries, err)
	}
}
