package project

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// InitOptions controls Project Configuration initialization.
type InitOptions struct {
	WorkingDirectory string
	ConfigPath       string
	Tracker          string
	Repositories     []string
}

// InitResult is the initialized Project Configuration and its location.
type InitResult struct {
	Path   string
	Config Config
}

// Initialize detects the containing Git repository and writes its portable configuration.
func Initialize(ctx context.Context, options InitOptions) (InitResult, error) {
	workingDirectory := options.WorkingDirectory
	if workingDirectory == "" {
		var err error
		workingDirectory, err = os.Getwd()
		if err != nil {
			return InitResult{}, fmt.Errorf("get working directory: %w", err)
		}
	}

	containingRoot, containingErr := gitOutput(ctx, workingDirectory, "rev-parse", "--show-toplevel")
	if containingErr != nil && (options.Tracker == "" || len(options.Repositories) == 0) {
		return InitResult{}, errors.New("no containing Git repository; provide --tracker and at least one --repo")
	}

	configPath := options.ConfigPath
	if configPath == "" {
		if containingErr == nil && len(options.Repositories) == 0 {
			configPath = filepath.Join(containingRoot, "heracles.yaml")
		} else {
			configPath = filepath.Join(workingDirectory, "heracles.yaml")
		}
	} else if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(workingDirectory, configPath)
	}
	configPath = filepath.Clean(configPath)
	configDirectory := filepath.Dir(configPath)
	repositoryPathBase := configDirectory
	if canonical, err := filepath.EvalSymlinks(configDirectory); err == nil {
		repositoryPathBase = canonical
	}

	tracker := options.Tracker
	if tracker == "" {
		detected, err := detectGitHubRepository(ctx, containingRoot)
		if err != nil {
			return InitResult{}, fmt.Errorf("detect Issue Tracker: %w", err)
		}
		tracker = detected
	} else {
		var err error
		tracker, err = parseGitHubRepository(tracker)
		if err != nil {
			return InitResult{}, fmt.Errorf("invalid Issue Tracker: %w", err)
		}
	}

	repositoryInputs := options.Repositories
	if len(repositoryInputs) == 0 {
		repositoryInputs = []string{containingRoot}
	}
	repositories := make([]RepositoryConfig, 0, len(repositoryInputs))
	for _, input := range repositoryInputs {
		repository, err := inspectRepository(ctx, workingDirectory, repositoryPathBase, input)
		if err != nil {
			return InitResult{}, err
		}
		if len(options.Repositories) == 0 {
			repository.Path = "."
		}
		repositories = append(repositories, repository)
	}

	config := Config{
		Version:      1,
		IssueTracker: IssueTrackerConfig{GitHub: tracker},
		Repositories: repositories,
		Agents: AgentConfig{
			DefaultProfile: "default",
			Profiles: map[string]ProfileConfig{
				"default": {
					Provider:     "codex",
					Timeout:      "1h",
					EnvAllowlist: []string{"PATH", "HOME"},
					Concurrency:  1,
				},
			},
		},
		Workspaces: WorkspaceConfig{
			Root:            ".heracles/workspaces",
			CleanupSuccess:  true,
			PreserveFailed:  true,
			PreserveBlocked: true,
		},
	}
	if err := validate(config); err != nil {
		return InitResult{}, err
	}
	if err := writeConfig(configPath, config); err != nil {
		return InitResult{}, err
	}

	return InitResult{Path: configPath, Config: config}, nil
}

func writeConfig(path string, config Config) error {
	contents, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode Project Configuration: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create Project Configuration directory: %w", err)
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		return fmt.Errorf("write Project Configuration: %w", err)
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

func parseGitHubRepository(origin string) (string, error) {
	if !strings.Contains(origin, ":") {
		return normalizeGitHubRepository(origin)
	}

	const sshPrefix = "git@github.com:"
	if strings.HasPrefix(origin, sshPrefix) {
		return normalizeGitHubRepository(strings.TrimPrefix(origin, sshPrefix))
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Hostname() != "github.com" {
		return "", fmt.Errorf("origin %q is not a GitHub repository", origin)
	}
	return normalizeGitHubRepository(parsed.Path)
}

func detectGitHubRepository(ctx context.Context, root string) (string, error) {
	origin, err := gitOutput(ctx, root, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	return parseGitHubRepository(origin)
}

func inspectRepository(ctx context.Context, workingDirectory, configDirectory, input string) (RepositoryConfig, error) {
	resolvedPath := input
	if !filepath.IsAbs(resolvedPath) {
		resolvedPath = filepath.Join(workingDirectory, resolvedPath)
	}
	resolvedPath = filepath.Clean(resolvedPath)

	root, err := gitOutput(ctx, resolvedPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return RepositoryConfig{}, fmt.Errorf("inspect Target Repository %q: %w", input, err)
	}
	githubRepository, err := detectGitHubRepository(ctx, root)
	if err != nil {
		return RepositoryConfig{}, fmt.Errorf("inspect Target Repository %q origin: %w", input, err)
	}
	baseBranch, err := gitOutput(ctx, root, "branch", "--show-current")
	if err != nil || baseBranch == "" {
		baseBranch = "main"
	}

	portablePath := root
	if filepath.IsAbs(input) {
		portablePath = filepath.Clean(input)
	} else {
		portablePath, err = filepath.Rel(configDirectory, root)
		if err != nil {
			return RepositoryConfig{}, fmt.Errorf("make Target Repository %q path portable: %w", input, err)
		}
	}

	return RepositoryConfig{
		Name:       filepath.Base(root),
		Path:       portablePath,
		GitHub:     githubRepository,
		BaseBranch: baseBranch,
	}, nil
}

func normalizeGitHubRepository(repository string) (string, error) {
	repository = strings.Trim(strings.TrimSuffix(repository, ".git"), "/")
	if len(strings.Split(repository, "/")) != 2 {
		return "", fmt.Errorf("invalid GitHub repository %q", repository)
	}
	return repository, nil
}
