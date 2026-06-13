package issuestage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledSkillHasSkillsShStructureAndOutputContract(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "skills", "to-issues-for-heracles", "SKILL.md")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read bundled skill: %v", err)
	}
	text := string(contents)
	for _, expected := range []string{
		"---\nname: to-issues-for-heracles",
		"description:",
		"AFK or HITL",
		"full GitHub issue URLs",
		"## Acceptance criteria",
		"## Exclusive Scopes",
		"heracles:ready",
		"heracles:hitl",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("SKILL.md does not contain %q", expected)
		}
	}
	if lines := strings.Count(text, "\n"); lines > 100 {
		t.Errorf("SKILL.md has %d lines, want progressive disclosure under 100", lines)
	}
}
