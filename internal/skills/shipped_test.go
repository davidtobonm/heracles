package skills_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrillWithDocsSkillHasSkillsShStructure(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "skills", "grill-with-docs", "SKILL.md")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read bundled skill: %v", err)
	}
	text := string(contents)
	for _, expected := range []string{
		"---\nname: grill-with-docs",
		"description:",
		"one",
		"Question Budget",
		"to-prd-for-heracles",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("SKILL.md does not contain %q", expected)
		}
	}
	if lines := strings.Count(text, "\n"); lines > 100 {
		t.Errorf("SKILL.md has %d lines, want progressive disclosure under 100", lines)
	}
}

func TestToPRDForHeraclesSkillHasSkillsShStructureAndLabels(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "skills", "to-prd-for-heracles", "SKILL.md")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read bundled skill: %v", err)
	}
	text := string(contents)
	for _, expected := range []string{
		"---\nname: to-prd-for-heracles",
		"description:",
		"heracles:prd",
		"heracles:review",
		"heracles:approved",
		"heracles:prd-revision:",
		"heracles plan --id",
		"heracles approve planning",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("SKILL.md does not contain %q", expected)
		}
	}
	if lines := strings.Count(text, "\n"); lines > 100 {
		t.Errorf("SKILL.md has %d lines, want progressive disclosure under 100", lines)
	}
}
