package control_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidtobonm/heracles/internal/control"
	"github.com/davidtobonm/heracles/internal/project"
)

func TestLocalControlWiresConfiguredServicesWithoutInvokingAgents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	config := `version: 1
issue_tracker:
  github: acme/backlog
repositories:
  - name: app
    path: .
    github: acme/app
    base_branch: main
agents:
  default_profile: default
  profiles:
    default:
      provider: codex
      concurrency: 1
workspaces:
  root: .heracles/workspaces
  cleanup_success: true
  preserve_failed: true
  preserve_blocked: true
labor:
  issue_concurrency: 1
delivery:
  auto_merge: false
planning:
  question_budget: 20
`
	path := filepath.Join(root, "heracles.yaml")
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loaded, err := project.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	surface, err := control.NewLocal(context.Background(), loaded)
	if err != nil {
		t.Fatalf("NewLocal() error = %v", err)
	}
	t.Cleanup(func() { _ = surface.Close() })
	result, err := surface.Execute(context.Background(), control.Operation{Name: "list", Kind: "labors"})
	if err != nil {
		t.Fatalf("Execute(list) error = %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("result = %#v", result)
	}
}
