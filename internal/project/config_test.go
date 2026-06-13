package project_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/project"
)

func TestDiscoverAndLoadResolvePortableRepositoryPaths(t *testing.T) {
	t.Parallel()

	repositoryPath := filepath.Join(t.TempDir(), "widget")
	createRepository(t, repositoryPath, "git@github.com:example/widget.git")
	nestedPath := filepath.Join(repositoryPath, "docs", "design")
	if err := os.MkdirAll(nestedPath, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}

	initialized, err := project.Initialize(context.Background(), project.InitOptions{
		WorkingDirectory: nestedPath,
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	discovered, err := project.Discover(nestedPath, "")
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if discovered != initialized.Path {
		t.Errorf("discovered path = %q, want %q", discovered, initialized.Path)
	}

	explicit, err := project.Discover(t.TempDir(), initialized.Path)
	if err != nil {
		t.Fatalf("Discover(explicit) error = %v", err)
	}
	if explicit != initialized.Path {
		t.Errorf("explicit path = %q, want %q", explicit, initialized.Path)
	}

	loaded, err := project.Load(discovered)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	resolvedPath, err := loaded.RepositoryPath("widget")
	if err != nil {
		t.Fatalf("RepositoryPath() error = %v", err)
	}

	canonicalRepositoryPath, err := filepath.EvalSymlinks(repositoryPath)
	if err != nil {
		t.Fatalf("resolve repository path: %v", err)
	}
	if resolvedPath != canonicalRepositoryPath {
		t.Errorf("resolved repository path = %q, want %q", resolvedPath, canonicalRepositoryPath)
	}
	if loaded.WorkspaceRoot() != filepath.Join(canonicalRepositoryPath, ".heracles", "workspaces") {
		t.Errorf("workspace root = %q, want portable state path", loaded.WorkspaceRoot())
	}
}

func TestLoadRejectsUnsupportedVersionAndUnknownFields(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		contents string
		expected string
	}{
		"unsupported version": {
			contents: "version: 2\nissue_tracker:\n  github: example/widget\nrepositories:\n  - name: widget\n    path: .\n    github: example/widget\n    base_branch: main\n",
			expected: "unsupported",
		},
		"unknown field": {
			contents: "version: 1\nsurprise: true\nissue_tracker:\n  github: example/widget\nrepositories:\n  - name: widget\n    path: .\n    github: example/widget\n    base_branch: main\n",
			expected: "field surprise",
		},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "heracles.yaml")
			if err := os.WriteFile(path, []byte(testCase.contents), 0o644); err != nil {
				t.Fatalf("write configuration: %v", err)
			}

			_, err := project.Load(path)
			if err == nil {
				t.Fatal("Load() error = nil, want validation failure")
			}
			if !strings.Contains(err.Error(), testCase.expected) {
				t.Errorf("Load() error = %q, want actionable %s failure", err, name)
			}
		})
	}
}

func TestLoadValidatesDeliveryMergeOrder(t *testing.T) {
	t.Parallel()

	for name, mergeOrder := range map[string]string{
		"unknown repository":   "missing",
		"duplicate repository": "widget, widget",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "heracles.yaml")
			contents := "version: 1\nissue_tracker:\n  github: example/widget\nrepositories:\n  - name: widget\n    path: .\n    github: example/widget\n    base_branch: main\ndelivery:\n  auto_merge: true\n  merge_order: [" + mergeOrder + "]\n"
			if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
				t.Fatalf("write configuration: %v", err)
			}

			_, err := project.Load(path)
			if err == nil || !strings.Contains(err.Error(), "merge_order") {
				t.Fatalf("Load() error = %v, want actionable merge_order failure", err)
			}
		})
	}
}

func TestLoadRejectsNegativePlanningQuestionBudget(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "heracles.yaml")
	contents := "version: 1\nissue_tracker:\n  github: example/widget\nrepositories:\n  - name: widget\n    path: .\n    github: example/widget\n    base_branch: main\nplanning:\n  question_budget: -1\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write configuration: %v", err)
	}

	_, err := project.Load(path)
	if err == nil || !strings.Contains(err.Error(), "question_budget") {
		t.Fatalf("Load() error = %v, want actionable question budget failure", err)
	}
}
