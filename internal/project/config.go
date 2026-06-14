package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the portable declaration of a Heracles project.
type Config struct {
	Version      int                `yaml:"version"`
	IssueTracker IssueTrackerConfig `yaml:"issue_tracker"`
	Repositories []RepositoryConfig `yaml:"repositories"`
	Agents       AgentConfig        `yaml:"agents,omitempty"`
	Workspaces   WorkspaceConfig    `yaml:"workspaces,omitempty"`
	Labor        LaborConfig        `yaml:"labor,omitempty"`
	Delivery     DeliveryConfig     `yaml:"delivery,omitempty"`
	Planning     PlanningConfig     `yaml:"planning,omitempty"`
}

// IssueTrackerConfig identifies the GitHub repository whose issues define work.
type IssueTrackerConfig struct {
	GitHub string `yaml:"github"`
}

// RepositoryConfig identifies a Target Repository.
type RepositoryConfig struct {
	Name       string   `yaml:"name"`
	Path       string   `yaml:"path"`
	GitHub     string   `yaml:"github"`
	BaseBranch string   `yaml:"base_branch"`
	Verify     []string `yaml:"verify,omitempty"`
	VerifyEnv  []string `yaml:"verify_env,omitempty"`
}

// AgentConfig declares reusable profiles and Agent Role assignments.
type AgentConfig struct {
	DefaultProfile string                   `yaml:"default_profile,omitempty"`
	Profiles       map[string]ProfileConfig `yaml:"profiles,omitempty"`
	Roles          RoleConfig               `yaml:"roles,omitempty"`
}

// ProfileConfig declares one possibly inherited Agent Profile.
type ProfileConfig struct {
	Extends      string   `yaml:"extends,omitempty"`
	Provider     string   `yaml:"provider,omitempty"`
	Model        string   `yaml:"model,omitempty"`
	Effort       string   `yaml:"effort,omitempty"`
	Variant      string   `yaml:"variant,omitempty"`
	Timeout      string   `yaml:"timeout,omitempty"`
	ExtraArgs    []string `yaml:"extra_args,omitempty"`
	EnvAllowlist []string `yaml:"env_allowlist,omitempty"`
	Concurrency  int      `yaml:"concurrency,omitempty"`
}

// RoleConfig assigns Agent Profiles to Labor responsibilities.
type RoleConfig struct {
	Planner     string `yaml:"planner,omitempty"`
	IssueAuthor string `yaml:"issue_author,omitempty"`
	Implementer string `yaml:"implementer,omitempty"`
	Reviewer    string `yaml:"reviewer,omitempty"`
}

// WorkspaceConfig declares Issue Workspace location and lifecycle policy.
type WorkspaceConfig struct {
	Root            string `yaml:"root,omitempty"`
	CleanupSuccess  bool   `yaml:"cleanup_success"`
	PreserveFailed  bool   `yaml:"preserve_failed"`
	PreserveBlocked bool   `yaml:"preserve_blocked"`
}

// LaborConfig declares end-to-end orchestration policy.
type LaborConfig struct {
	IssueConcurrency int `yaml:"issue_concurrency,omitempty"`
}

// DeliveryConfig declares pull request and automatic merge policy.
type DeliveryConfig struct {
	AutoMerge  bool     `yaml:"auto_merge"`
	MergeOrder []string `yaml:"merge_order,omitempty"`
}

// PlanningConfig declares bounded clarification policy.
type PlanningConfig struct {
	QuestionBudget int `yaml:"question_budget,omitempty"`
}

// LoadedConfig is a validated Project Configuration and its location.
type LoadedConfig struct {
	Path   string
	Config Config
}

// Discover searches upward for heracles.yaml or resolves an explicit path.
func Discover(workingDirectory, explicitPath string) (string, error) {
	if workingDirectory == "" {
		var err error
		workingDirectory, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
	}

	if explicitPath != "" {
		if !filepath.IsAbs(explicitPath) {
			explicitPath = filepath.Join(workingDirectory, explicitPath)
		}
		return existingCanonicalPath(explicitPath)
	}

	current, err := filepath.Abs(workingDirectory)
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	for {
		candidate := filepath.Join(current, "heracles.yaml")
		if path, err := existingCanonicalPath(candidate); err == nil {
			return path, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("heracles.yaml not found; run `heracles init` or pass --config")
		}
		current = parent
	}
}

// Load reads and validates a Project Configuration.
func Load(path string) (LoadedConfig, error) {
	canonicalPath, err := existingCanonicalPath(path)
	if err != nil {
		return LoadedConfig{}, err
	}

	file, err := os.Open(canonicalPath)
	if err != nil {
		return LoadedConfig{}, fmt.Errorf("open Project Configuration: %w", err)
	}
	defer file.Close()

	var config Config
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return LoadedConfig{}, fmt.Errorf("decode Project Configuration: %w", err)
	}
	if err := validate(config); err != nil {
		return LoadedConfig{}, err
	}
	return LoadedConfig{Path: canonicalPath, Config: config}, nil
}

// RepositoryPath resolves a named Target Repository relative to the configuration.
func (loaded LoadedConfig) RepositoryPath(name string) (string, error) {
	for _, repository := range loaded.Config.Repositories {
		if repository.Name != name {
			continue
		}

		path := repository.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(loaded.Path), path)
		}
		path = filepath.Clean(path)
		if canonical, err := filepath.EvalSymlinks(path); err == nil {
			path = canonical
		}
		return path, nil
	}
	return "", fmt.Errorf("unknown Target Repository %q", name)
}

// WorkspaceRoot resolves the configured Issue Workspace root.
func (loaded LoadedConfig) WorkspaceRoot() string {
	root := loaded.Config.Workspaces.Root
	if root == "" {
		root = ".heracles/workspaces"
	}
	if !filepath.IsAbs(root) {
		root = filepath.Join(filepath.Dir(loaded.Path), root)
	}
	return filepath.Clean(root)
}

func validate(config Config) error {
	if config.Version != 1 {
		return fmt.Errorf("unsupported Project Configuration version %d", config.Version)
	}
	if _, err := parseGitHubRepository(config.IssueTracker.GitHub); err != nil {
		return fmt.Errorf("invalid Issue Tracker: %w", err)
	}
	if len(config.Repositories) == 0 {
		return errors.New("Project Configuration requires at least one Target Repository")
	}

	names := make(map[string]struct{}, len(config.Repositories))
	for index, repository := range config.Repositories {
		if repository.Name == "" || repository.Path == "" || repository.GitHub == "" || repository.BaseBranch == "" {
			return fmt.Errorf("Target Repository %d requires name, path, github, and base_branch", index+1)
		}
		if _, exists := names[repository.Name]; exists {
			return fmt.Errorf("duplicate Target Repository name %q", repository.Name)
		}
		names[repository.Name] = struct{}{}
		if _, err := parseGitHubRepository(repository.GitHub); err != nil {
			return fmt.Errorf("invalid Target Repository %q GitHub repository: %w", repository.Name, err)
		}
	}
	mergeNames := make(map[string]struct{}, len(config.Delivery.MergeOrder))
	for _, name := range config.Delivery.MergeOrder {
		if _, exists := names[name]; !exists {
			return fmt.Errorf("delivery merge_order references unknown Target Repository %q", name)
		}
		if _, exists := mergeNames[name]; exists {
			return fmt.Errorf("delivery merge_order contains duplicate Target Repository %q", name)
		}
		mergeNames[name] = struct{}{}
	}
	if config.Planning.QuestionBudget < 0 {
		return errors.New("planning.question_budget cannot be negative")
	}
	return nil
}

func existingCanonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve Project Configuration path: %w", err)
	}
	if _, err := os.Stat(absolute); err != nil {
		return "", fmt.Errorf("Project Configuration %q: %w", absolute, err)
	}
	if canonical, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = canonical
	}
	return absolute, nil
}
