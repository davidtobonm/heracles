package control_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/control"
	"github.com/davidtobonm/heracles/internal/labor"
	"github.com/davidtobonm/heracles/internal/project"
)

// fakeDoctorSystem reports every Doctor check as passing so preflight does
// not block control-surface tests that exercise a project configuration
// pointing at repositories and trackers that do not exist on disk or
// GitHub.
type fakeDoctorSystem struct{}

func (fakeDoctorSystem) LookPath(string) (string, error) { return "/usr/bin/fake", nil }

func (fakeDoctorSystem) Run(context.Context, string, ...string) error { return nil }

func (fakeDoctorSystem) Output(context.Context, string, ...string) (string, error) { return "", nil }

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
	surface, err := control.NewLocalWithSystem(context.Background(), loaded, fakeDoctorSystem{})
	if err != nil {
		t.Fatalf("NewLocalWithSystem() error = %v", err)
	}
	t.Cleanup(func() { _ = surface.Close() })
	result, err := surface.Execute(context.Background(), control.Operation{Name: "list", Kind: "labors"})
	if err != nil {
		t.Fatalf("Execute(list) error = %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("result = %#v", result)
	}

	if err := labor.NewFileStore(root).Save(context.Background(), labor.State{ID: "labor-1", Problem: "Original problem", Status: labor.StatusCompleted}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := surface.Execute(context.Background(), control.Operation{Name: "labor", ID: "labor-1", Problem: "Different problem"}); err == nil || !strings.Contains(err.Error(), "already exists with a different problem") {
		t.Errorf("Execute(labor with conflicting problem) error = %v, want conflict", err)
	}
}

// blockedDoctorSystem reports git as missing, which is a Doctor blocker per
// ADR 0027 and must stop Labor execution before it starts.
type blockedDoctorSystem struct{ fakeDoctorSystem }

func (blockedDoctorSystem) LookPath(executable string) (string, error) {
	if executable == "git" {
		return "", errors.New("not installed")
	}
	return "/usr/bin/fake", nil
}

func TestLocalControlBlocksLaborWhenDoctorPreflightFails(t *testing.T) {
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

	for _, operation := range []control.Operation{
		{Name: "labor", Problem: "Ship a feature"},
		{Name: "run"},
		{Name: "resume", ID: "labor-1"},
	} {
		surface, err := control.NewLocalWithSystem(context.Background(), loaded, blockedDoctorSystem{})
		if err != nil {
			t.Fatalf("NewLocalWithSystem() error = %v", err)
		}

		result, err := surface.Execute(context.Background(), operation)
		if err == nil || !strings.Contains(err.Error(), "Doctor preflight failed") {
			t.Errorf("Execute(%s) error = %v, want Doctor preflight failure", operation.Name, err)
		}
		if result.Status != "blocked" {
			t.Errorf("Execute(%s) status = %q, want blocked", operation.Name, result.Status)
		}

		_ = surface.Close()
	}
}
