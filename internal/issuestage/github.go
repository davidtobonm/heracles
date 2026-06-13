package issuestage

import (
	"context"
	"fmt"
	"strings"
)

// CommandRunner executes GitHub CLI operations.
type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

// GitHubPublisher publishes approved proposals through the GitHub CLI.
type GitHubPublisher struct {
	runner CommandRunner
}

// NewGitHubPublisher creates a GitHub issue publisher.
func NewGitHubPublisher(runner CommandRunner) *GitHubPublisher {
	return &GitHubPublisher{runner: runner}
}

// CreateIssue publishes one approved issue.
func (publisher *GitHubPublisher) CreateIssue(ctx context.Context, input PublishInput) (string, error) {
	args := []string{"issue", "create", "--repo", input.Repository, "--title", input.Title, "--body", input.Body}
	for _, label := range input.Labels {
		args = append(args, "--label", label)
	}
	output, err := publisher.runner.Run(ctx, "gh", args...)
	if err != nil {
		return "", err
	}
	url := strings.TrimSpace(string(output))
	if !strings.HasPrefix(url, "https://github.com/"+input.Repository+"/issues/") {
		return "", fmt.Errorf("GitHub CLI returned invalid created issue URL %q", url)
	}
	return url, nil
}
