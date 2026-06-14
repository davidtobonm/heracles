package project_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/project"
)

func TestInitializeDetectsContainingGitHubRepository(t *testing.T) {
	t.Parallel()

	repositoryPath := filepath.Join(t.TempDir(), "widget")
	run(t, "", "git", "init", "--initial-branch=main", repositoryPath)
	run(t, repositoryPath, "git", "remote", "add", "origin", "git@github.com:example/widget.git")

	nestedPath := filepath.Join(repositoryPath, "docs", "design")
	if err := os.MkdirAll(nestedPath, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}

	result, err := project.Initialize(context.Background(), project.InitOptions{
		WorkingDirectory: nestedPath,
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	canonicalRepositoryPath, err := filepath.EvalSymlinks(repositoryPath)
	if err != nil {
		t.Fatalf("resolve repository path: %v", err)
	}
	if result.Path != filepath.Join(canonicalRepositoryPath, "heracles.yaml") {
		t.Errorf("configuration path = %q, want repository root", result.Path)
	}
	if result.Config.IssueTracker.GitHub != "example/widget" {
		t.Errorf("issue tracker = %q, want example/widget", result.Config.IssueTracker.GitHub)
	}
	if len(result.Config.Repositories) != 1 {
		t.Fatalf("repositories = %d, want 1", len(result.Config.Repositories))
	}

	repository := result.Config.Repositories[0]
	if repository.Name != "widget" || repository.Path != "." || repository.GitHub != "example/widget" || repository.BaseBranch != "main" {
		t.Errorf("repository = %#v, want detected portable defaults", repository)
	}
	if result.Config.Agents.DefaultProfile != "default" || result.Config.Agents.Profiles["default"].Provider != "codex" {
		t.Errorf("agents = %#v, want usable default Agent Profile", result.Config.Agents)
	}
	if !result.Config.Delivery.AutoMerge {
		t.Errorf("delivery = %#v, want automatic merging enabled by default", result.Config.Delivery)
	}
	if result.Config.Planning.QuestionBudget != 20 {
		t.Errorf("planning = %#v, want default Question Budget 20", result.Config.Planning)
	}
}

func TestInitializeOutsideGitRequiresTrackerAndRepository(t *testing.T) {
	t.Parallel()

	_, err := project.Initialize(context.Background(), project.InitOptions{
		WorkingDirectory: t.TempDir(),
	})
	if err == nil {
		t.Fatal("Initialize() error = nil, want actionable setup failure")
	}
	for _, expected := range []string{"no containing Git repository", "--tracker", "--repo"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("Initialize() error = %q, want %q", err, expected)
		}
	}
}

func TestInitializeAcceptsExplicitTrackerAndRepeatedRepositories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	configDirectory := filepath.Join(root, "project")
	if err := os.MkdirAll(configDirectory, 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}

	relativeRepository := filepath.Join(root, "frontend")
	createRepository(t, relativeRepository, "git@github.com:acme/frontend.git")

	absoluteRepository := filepath.Join(t.TempDir(), "backend")
	createRepository(t, absoluteRepository, "https://github.com/acme/backend.git")

	result, err := project.Initialize(context.Background(), project.InitOptions{
		WorkingDirectory: configDirectory,
		ConfigPath:       filepath.Join(configDirectory, "heracles.yaml"),
		Tracker:          "acme/backlog",
		Repositories:     []string{"../frontend", absoluteRepository},
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if result.Config.IssueTracker.GitHub != "acme/backlog" {
		t.Errorf("issue tracker = %q, want explicit tracker", result.Config.IssueTracker.GitHub)
	}
	if len(result.Config.Repositories) != 2 {
		t.Fatalf("repositories = %d, want 2", len(result.Config.Repositories))
	}
	if result.Config.Repositories[0].Path != "../frontend" {
		t.Errorf("relative repository path = %q, want preserved portable path", result.Config.Repositories[0].Path)
	}
	if result.Config.Repositories[1].Path != absoluteRepository {
		t.Errorf("absolute repository path = %q, want preserved absolute path", result.Config.Repositories[1].Path)
	}
}

func createRepository(t testing.TB, path, origin string) {
	t.Helper()
	run(t, "", "git", "init", "--initial-branch=main", path)
	run(t, path, "git", "remote", "add", "origin", origin)
}

func run(t testing.TB, workingDirectory, command string, args ...string) {
	t.Helper()

	process := exec.Command(command, args...)
	if workingDirectory != "" {
		process.Dir = workingDirectory
	}
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", command, args, err, output)
	}
}
