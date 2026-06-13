// Package workspace creates isolated per-issue Git worktrees.
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Repository is one configured Target Repository.
type Repository struct {
	Name       string
	Path       string
	BaseBranch string
}

// Request identifies one issue that needs an isolated workspace.
type Request struct {
	IssueRepository string
	IssueNumber     int
	Title           string
}

// Worktree is one Target Repository's isolated issue worktree.
type Worktree struct {
	Name           string `json:"name"`
	SourcePath     string `json:"source_path"`
	Path           string `json:"path"`
	BaselineCommit string `json:"baseline_commit"`
}

// Workspace coordinates one issue across all Target Repositories.
type Workspace struct {
	Root         string     `json:"root"`
	Key          string     `json:"key"`
	Branch       string     `json:"branch"`
	Repositories []Worktree `json:"repositories"`
}

// Repository returns a named worktree.
func (workspace Workspace) Repository(name string) Worktree {
	for _, repository := range workspace.Repositories {
		if repository.Name == name {
			return repository
		}
	}
	return Worktree{}
}

// Policy controls workspace preservation and cleanup.
type Policy struct {
	CleanupSuccess  bool
	PreserveFailed  bool
	PreserveBlocked bool
}

// DefaultPolicy preserves failed and blocked work while cleaning successful work.
func DefaultPolicy() Policy {
	return Policy{CleanupSuccess: true, PreserveFailed: true, PreserveBlocked: true}
}

// Outcome is the result of an issue attempt.
type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeFailed  Outcome = "failed"
	OutcomeBlocked Outcome = "blocked"
)

// Manager creates and finalizes Issue Workspaces.
type Manager struct {
	Root         string
	Repositories []Repository
	Policy       Policy
}

// Create creates or resumes one coordinated Issue Workspace.
func (manager Manager) Create(ctx context.Context, request Request) (Workspace, error) {
	if manager.Root == "" || len(manager.Repositories) == 0 {
		return Workspace{}, errors.New("Issue Workspace requires a root and at least one Target Repository")
	}
	if request.IssueRepository == "" || request.IssueNumber < 1 {
		return Workspace{}, errors.New("Issue Workspace requires issue repository and positive issue number")
	}

	key := slug(request.IssueRepository + "-" + strconv.Itoa(request.IssueNumber) + "-" + request.Title)
	issueRoot := filepath.Join(manager.Root, key)
	workspace := Workspace{Root: issueRoot, Key: key, Branch: "heracles/" + key}
	metadataPath := filepath.Join(issueRoot, "workspace.json")
	if contents, err := os.ReadFile(metadataPath); err == nil {
		if err := json.Unmarshal(contents, &workspace); err != nil {
			return Workspace{}, fmt.Errorf("decode Issue Workspace metadata: %w", err)
		}
	}
	if err := os.MkdirAll(issueRoot, 0o755); err != nil {
		return workspace, fmt.Errorf("create Issue Workspace root: %w", err)
	}

	for _, repository := range manager.Repositories {
		if workspace.Repository(repository.Name).Name != "" {
			continue
		}
		worktree, err := manager.createWorktree(ctx, workspace, repository)
		if err != nil {
			return workspace, err
		}
		workspace.Repositories = append(workspace.Repositories, worktree)
		slices.SortFunc(workspace.Repositories, func(left, right Worktree) int { return strings.Compare(left.Name, right.Name) })
		if err := writeMetadata(metadataPath, workspace); err != nil {
			return workspace, err
		}
	}
	return workspace, nil
}

// Touched returns repositories with committed or uncommitted issue changes.
func (manager Manager) Touched(ctx context.Context, workspace Workspace) ([]string, error) {
	var touched []string
	for _, repository := range workspace.Repositories {
		status, err := gitOutput(ctx, repository.Path, "status", "--porcelain")
		if err != nil {
			return nil, fmt.Errorf("inspect %s worktree status: %w", repository.Name, err)
		}
		head, err := gitOutput(ctx, repository.Path, "rev-parse", "HEAD")
		if err != nil {
			return nil, fmt.Errorf("inspect %s worktree HEAD: %w", repository.Name, err)
		}
		if status != "" || head != repository.BaselineCommit {
			touched = append(touched, repository.Name)
		}
	}
	slices.Sort(touched)
	return touched, nil
}

// Finalize preserves or cleans a workspace according to policy and outcome.
func (manager Manager) Finalize(ctx context.Context, workspace Workspace, outcome Outcome) error {
	if manager.shouldPreserve(outcome) {
		return nil
	}
	for _, repository := range workspace.Repositories {
		if _, err := os.Stat(repository.Path); os.IsNotExist(err) {
			continue
		}
		if _, err := gitOutput(ctx, repository.SourcePath, "worktree", "remove", "--force", repository.Path); err != nil {
			return fmt.Errorf("remove %s worktree: %w", repository.Name, err)
		}
	}
	if err := os.RemoveAll(workspace.Root); err != nil {
		return fmt.Errorf("remove Issue Workspace metadata: %w", err)
	}
	return nil
}

func (manager Manager) createWorktree(ctx context.Context, workspace Workspace, repository Repository) (Worktree, error) {
	sourcePath, err := filepath.Abs(repository.Path)
	if err != nil {
		return Worktree{}, fmt.Errorf("resolve Target Repository %s: %w", repository.Name, err)
	}
	baseline, err := gitOutput(ctx, sourcePath, "rev-parse", repository.BaseBranch)
	if err != nil {
		return Worktree{}, fmt.Errorf("resolve %s base branch %q: %w", repository.Name, repository.BaseBranch, err)
	}
	path := filepath.Join(workspace.Root, slug(repository.Name))
	if _, err := os.Stat(path); err == nil {
		if _, err := gitOutput(ctx, path, "rev-parse", "--git-dir"); err != nil {
			return Worktree{}, fmt.Errorf("existing %s Issue Workspace path is not a Git worktree: %w", repository.Name, err)
		}
		return Worktree{Name: repository.Name, SourcePath: sourcePath, Path: path, BaselineCommit: baseline}, nil
	}

	branchExists := gitSuccess(ctx, sourcePath, "show-ref", "--verify", "--quiet", "refs/heads/"+workspace.Branch)
	args := []string{"worktree", "add"}
	if branchExists {
		args = append(args, path, workspace.Branch)
	} else {
		args = append(args, "-b", workspace.Branch, path, baseline)
	}
	if _, err := gitOutput(ctx, sourcePath, args...); err != nil {
		return Worktree{}, fmt.Errorf("create %s issue worktree: %w", repository.Name, err)
	}
	return Worktree{Name: repository.Name, SourcePath: sourcePath, Path: path, BaselineCommit: baseline}, nil
}

func (manager Manager) shouldPreserve(outcome Outcome) bool {
	switch outcome {
	case OutcomeSuccess:
		return !manager.Policy.CleanupSuccess
	case OutcomeFailed:
		return manager.Policy.PreserveFailed
	case OutcomeBlocked:
		return manager.Policy.PreserveBlocked
	default:
		return true
	}
}

func writeMetadata(path string, workspace Workspace) error {
	contents, err := json.MarshalIndent(workspace, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Issue Workspace metadata: %w", err)
	}
	contents = append(contents, '\n')
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		return fmt.Errorf("write Issue Workspace metadata: %w", err)
	}
	return nil
}

func gitOutput(ctx context.Context, workingDirectory string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = workingDirectory
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func gitSuccess(ctx context.Context, workingDirectory string, args ...string) bool {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = workingDirectory
	return command.Run() == nil
}

func slug(value string) string {
	value = strings.Trim(strings.Map(func(character rune) rune {
		switch {
		case character >= 'a' && character <= 'z':
			return character
		case character >= 'A' && character <= 'Z':
			return character + ('a' - 'A')
		case character >= '0' && character <= '9':
			return character
		case character == '-', character == '_', character == '.':
			return character
		default:
			return '-'
		}
	}, value), "-")
	if len(value) > 120 {
		value = value[:120]
	}
	return value
}
