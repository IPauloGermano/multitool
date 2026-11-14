package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IPauloGermano/multitool/internal/system"
)

func TestCodexSkillsLifecycle(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))
	r := securityRunner(t, &MockExecutor{}, system.OSWindows)
	target := filepath.Join(root, ".agents", "skills")
	r.sysInfo.SkillDirs = []string{target}
	source := r.getSkillDir("test-skill")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: test-skill\ndescription: test\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := r.installGitSkill("test-skill", "owner/repo"); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(target, "test-skill")
	if _, err := os.ReadFile(filepath.Join(link, "SKILL.md")); err != nil {
		t.Fatalf("Codex cannot read skill: %v", err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if count := r.RelinkAllSkills(); count != 1 {
		t.Fatalf("sync relinked %d skills", count)
	}
	if err := r.uninstallGitSkill("test-skill"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("managed link remains: %v", err)
	}
}

func TestCodexUninstallPreservesUnrelatedSkill(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))
	r := securityRunner(t, &MockExecutor{}, system.OSWindows)
	target := filepath.Join(root, ".agents", "skills")
	r.sysInfo.SkillDirs = []string{target}
	other := filepath.Join(target, "test-skill")
	if err := os.MkdirAll(other, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "SKILL.md"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := r.uninstallGitSkill("test-skill"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(other, "SKILL.md")); err != nil || string(data) != "keep" {
		t.Fatalf("unrelated skill changed: %v", err)
	}
}

func TestSubSkillsCollectionLifecycle(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))
	r := securityRunner(t, &MockExecutor{}, system.OSWindows)
	codexTarget := filepath.Join(root, ".codex", "skills")
	geminiTarget := filepath.Join(root, ".gemini", "skills")
	r.sysInfo.SkillDirs = []string{codexTarget, geminiTarget}

	source := r.getSkillDir("superpowers")
	sub1 := filepath.Join(source, "skills", "brainstorming")
	sub2 := filepath.Join(source, "skills", "writing-plans")
	if err := os.MkdirAll(sub1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sub2, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub1, "SKILL.md"), []byte("---\nname: brainstorming\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub2, "SKILL.md"), []byte("---\nname: writing-plans\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.installGitSkill("superpowers", "obra/superpowers"); err != nil {
		t.Fatal(err)
	}

	// Verify both sub-skills are readable in both Codex and Gemini/Antigravity targets
	for _, target := range []string{codexTarget, geminiTarget} {
		bLink := filepath.Join(target, "brainstorming")
		wLink := filepath.Join(target, "writing-plans")

		bData, err := os.ReadFile(filepath.Join(bLink, "SKILL.md"))
		if err != nil || !filepath.IsAbs(bLink) {
			t.Fatalf("failed to read subskill brainstorming in %s: %v", target, err)
		}
		if string(bData) != "---\nname: brainstorming\n---\n" {
			t.Fatalf("unexpected content for brainstorming in %s", target)
		}

		wData, err := os.ReadFile(filepath.Join(wLink, "SKILL.md"))
		if err != nil {
			t.Fatalf("failed to read subskill writing-plans in %s: %v", target, err)
		}
		if string(wData) != "---\nname: writing-plans\n---\n" {
			t.Fatalf("unexpected content for writing-plans in %s", target)
		}
	}

	// Uninstall should cleanly remove both sub-skill links and the package link
	if err := r.uninstallGitSkill("superpowers"); err != nil {
		t.Fatal(err)
	}

	for _, target := range []string{codexTarget, geminiTarget} {
		for _, name := range []string{"brainstorming", "writing-plans", "superpowers"} {
			link := filepath.Join(target, name)
			if _, err := os.Lstat(link); !os.IsNotExist(err) {
				t.Fatalf("link %s still exists after uninstall", link)
			}
		}
	}
}
