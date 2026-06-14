package skills_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidtobonm/heracles/internal/skills"
)

func TestBundledReturnsShippedSkillsWithDescriptions(t *testing.T) {
	t.Parallel()

	bundledSkills, err := skills.Bundled()
	if err != nil {
		t.Fatalf("Bundled() error = %v", err)
	}

	names := make(map[string]skills.Skill, len(bundledSkills))
	for _, skill := range bundledSkills {
		names[skill.Name] = skill
	}

	for _, want := range []string{"grill-with-docs", "to-prd-for-heracles", "to-issues-for-heracles"} {
		skill, ok := names[want]
		if !ok {
			t.Fatalf("Bundled() = %#v, want skill %q", bundledSkills, want)
		}
		if skill.Description == "" {
			t.Errorf("skill %q Description = %q, want non-empty", want, skill.Description)
		}
		content, ok := skill.Files["SKILL.md"]
		if !ok || len(content) == 0 {
			t.Errorf("skill %q Files[SKILL.md] missing or empty", want)
		}
	}
}

func TestProjectAndGlobalDirFollowSkillsShConvention(t *testing.T) {
	t.Parallel()

	if got, want := skills.ProjectDir("claude", "/repo"), filepath.Join("/repo", ".claude", "skills"); got != want {
		t.Errorf("ProjectDir() = %q, want %q", got, want)
	}
	if got, want := skills.GlobalDir("codex", "/home/user"), filepath.Join("/home/user", ".codex", "skills"); got != want {
		t.Errorf("GlobalDir() = %q, want %q", got, want)
	}
}

func TestInstallWritesBundledSkillsAndProtectsExistingFiles(t *testing.T) {
	t.Parallel()

	bundledSkills, err := skills.Bundled()
	if err != nil {
		t.Fatalf("Bundled() error = %v", err)
	}

	dir := t.TempDir()
	results, err := skills.Install(dir, bundledSkills, false)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if len(results) != len(bundledSkills) {
		t.Fatalf("Install() = %#v, want one result per bundled skill", results)
	}
	for _, result := range results {
		if !result.Installed || result.Skipped {
			t.Errorf("result = %#v, want freshly installed", result)
		}
	}
	for _, skill := range bundledSkills {
		path := filepath.Join(dir, skill.Name, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("Stat(%s) error = %v, want installed SKILL.md", path, err)
		}
	}

	// Locally modify one installed skill.
	modified := filepath.Join(dir, bundledSkills[0].Name, "SKILL.md")
	if err := os.WriteFile(modified, []byte("local edits"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Re-installing without --force must not overwrite the local edit.
	results, err = skills.Install(dir, bundledSkills, false)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	for _, result := range results {
		if result.Skill == bundledSkills[0].Name {
			if !result.Skipped || result.Installed {
				t.Errorf("result = %#v, want skipped to protect local edits", result)
			}
		}
	}
	contents, err := os.ReadFile(modified)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(contents) != "local edits" {
		t.Errorf("SKILL.md = %q, want local edits preserved", contents)
	}

	// Re-installing with force overwrites.
	results, err = skills.Install(dir, bundledSkills, true)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	for _, result := range results {
		if result.Skill == bundledSkills[0].Name && (!result.Installed || result.Skipped) {
			t.Errorf("result = %#v, want force re-install", result)
		}
	}
	contents, err = os.ReadFile(modified)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(contents) == "local edits" {
		t.Errorf("SKILL.md = %q, want overwritten by --force", contents)
	}
}
