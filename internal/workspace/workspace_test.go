package workspace_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/workspace"
)

func TestIssueWorkspaceIsolatesDirtyRepositoriesDetectsTouchesAndFollowsPolicy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	backend := createRepository(t, filepath.Join(root, "backend"))
	frontend := createRepository(t, filepath.Join(root, "frontend"))
	if err := os.WriteFile(filepath.Join(backend, "tracked.txt"), []byte("dirty original\n"), 0o644); err != nil {
		t.Fatalf("dirty original repository: %v", err)
	}

	manager := workspace.Manager{
		Root: filepath.Join(root, "issue-workspaces"),
		Repositories: []workspace.Repository{
			{Name: "backend", Path: backend, BaseBranch: "main"},
			{Name: "frontend", Path: frontend, BaseBranch: "main"},
		},
		Policy: workspace.DefaultPolicy(),
	}
	request := workspace.Request{IssueRepository: "acme/backlog", IssueNumber: 7, Title: "Cross repository delivery"}

	created, err := manager.Create(context.Background(), request)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(created.Repositories) != 2 {
		t.Fatalf("worktrees = %d, want one per Target Repository", len(created.Repositories))
	}
	for _, repository := range created.Repositories {
		if branch := gitOutput(t, repository.Path, "branch", "--show-current"); branch != created.Branch {
			t.Errorf("%s worktree branch = %q, want %q", repository.Name, branch, created.Branch)
		}
	}
	backendWorktree := created.Repository("backend")
	baselineContents, err := os.ReadFile(filepath.Join(backendWorktree.Path, "tracked.txt"))
	if err != nil {
		t.Fatalf("read isolated baseline: %v", err)
	}
	if string(baselineContents) != "baseline\n" {
		t.Errorf("backend worktree contains original dirty state: %q", baselineContents)
	}
	if branch := gitOutput(t, backend, "branch", "--show-current"); branch != "main" {
		t.Errorf("original backend branch = %q, want main", branch)
	}
	if status := gitOutput(t, backend, "status", "--porcelain"); status == "" {
		t.Error("original backend dirty state was disturbed")
	}

	frontendWorktree := created.Repository("frontend")
	if err := os.WriteFile(filepath.Join(frontendWorktree.Path, "feature.txt"), []byte("delivered\n"), 0o644); err != nil {
		t.Fatalf("write worktree change: %v", err)
	}
	run(t, frontendWorktree.Path, "git", "add", "feature.txt")
	run(t, frontendWorktree.Path, "git", "commit", "-m", "deliver feature")
	if err := os.WriteFile(filepath.Join(backendWorktree.Path, "uncommitted.txt"), []byte("in progress\n"), 0o644); err != nil {
		t.Fatalf("write uncommitted worktree change: %v", err)
	}

	touched, err := manager.Touched(context.Background(), created)
	if err != nil {
		t.Fatalf("Touched() error = %v", err)
	}
	if !reflect.DeepEqual(touched, []string{"backend", "frontend"}) {
		t.Errorf("touched repositories = %#v, want committed and uncommitted changes", touched)
	}

	resumed, err := manager.Create(context.Background(), request)
	if err != nil {
		t.Fatalf("Create(resume) error = %v", err)
	}
	if resumed.Root != created.Root || resumed.Repository("frontend").Path != frontendWorktree.Path {
		t.Errorf("resumed workspace = %#v, want existing workspace", resumed)
	}

	if err := manager.Finalize(context.Background(), created, workspace.OutcomeBlocked); err != nil {
		t.Fatalf("Finalize(blocked) error = %v", err)
	}
	if _, err := os.Stat(created.Root); err != nil {
		t.Fatalf("blocked workspace was not preserved: %v", err)
	}
	if err := manager.Finalize(context.Background(), created, workspace.OutcomeSuccess); err != nil {
		t.Fatalf("Finalize(success) error = %v", err)
	}
	if _, err := os.Stat(created.Root); !os.IsNotExist(err) {
		t.Fatalf("successful workspace still exists: %v", err)
	}
}

func TestIssueWorkspaceCanUseHeraclesStateDirectoryInsideSourceRepository(t *testing.T) {
	t.Parallel()

	repository := createRepository(t, filepath.Join(t.TempDir(), "app"))
	manager := workspace.Manager{
		Root:         filepath.Join(repository, ".heracles", "workspaces"),
		Repositories: []workspace.Repository{{Name: "app", Path: repository, BaseBranch: "main"}},
		Policy:       workspace.DefaultPolicy(),
	}

	created, err := manager.Create(context.Background(), workspace.Request{IssueRepository: "acme/app", IssueNumber: 3, Title: "Nested state"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Repository("app").Path == "" {
		t.Fatal("nested `.heracles/` Issue Workspace was not created")
	}
}

func createRepository(t testing.TB, path string) string {
	t.Helper()
	run(t, "", "git", "init", "--initial-branch=main", path)
	run(t, path, "git", "config", "user.email", "heracles@example.com")
	run(t, path, "git", "config", "user.name", "Heracles Test")
	if err := os.WriteFile(filepath.Join(path, "tracked.txt"), []byte("baseline\n"), 0o644); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	run(t, path, "git", "add", "tracked.txt")
	run(t, path, "git", "commit", "-m", "baseline")
	return path
}

func gitOutput(t testing.TB, workingDirectory string, args ...string) string {
	t.Helper()
	process := exec.Command("git", args...)
	process.Dir = workingDirectory
	output, err := process.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func run(t testing.TB, workingDirectory, command string, args ...string) {
	t.Helper()
	process := exec.Command(command, args...)
	process.Dir = workingDirectory
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", command, args, err, output)
	}
}
