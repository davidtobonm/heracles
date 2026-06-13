package changeset

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// CommandRunner executes GitHub CLI operations.
type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

// GitHubClient implements Change Set delivery through the GitHub CLI.
type GitHubClient struct {
	runner CommandRunner
}

// NewGitHubClient creates a GitHub pull request client.
func NewGitHubClient(runner CommandRunner) *GitHubClient {
	return &GitHubClient{runner: runner}
}

// CreatePullRequest opens one pull request and decodes the URL returned by gh.
func (client *GitHubClient) CreatePullRequest(ctx context.Context, input PullRequestInput) (PullRequest, error) {
	output, err := client.runner.Run(ctx, "gh", "pr", "create",
		"--repo", input.Repository.GitHub,
		"--head", input.Repository.Head,
		"--base", input.Repository.Base,
		"--title", input.Title,
		"--body", input.Body,
	)
	if err != nil {
		return PullRequest{}, err
	}
	pullRequestURL := strings.TrimSpace(string(output))
	number, err := pullRequestNumber(pullRequestURL)
	if err != nil {
		return PullRequest{}, fmt.Errorf("decode created pull request URL: %w", err)
	}
	return PullRequest{
		Repository: input.Repository.Name,
		URL:        pullRequestURL,
		Number:     number,
		Body:       input.Body,
	}, nil
}

// UpdatePullRequestBody links a pull request to its complete Change Set.
func (client *GitHubClient) UpdatePullRequestBody(ctx context.Context, pullRequest PullRequest, body string) error {
	_, err := client.runner.Run(ctx, "gh", "pr", "edit", strconv.Itoa(pullRequest.Number),
		"--repo", repositoryFromPullRequestURL(pullRequest.URL),
		"--body", body,
	)
	return err
}

// WaitForCI waits for all required checks on a pull request.
func (client *GitHubClient) WaitForCI(ctx context.Context, pullRequest PullRequest) error {
	_, err := client.runner.Run(ctx, "gh", "pr", "checks", strconv.Itoa(pullRequest.Number),
		"--repo", repositoryFromPullRequestURL(pullRequest.URL),
		"--required",
		"--watch",
	)
	return err
}

// Merge merges a pull request after required checks pass.
func (client *GitHubClient) Merge(ctx context.Context, pullRequest PullRequest) error {
	_, err := client.runner.Run(ctx, "gh", "pr", "merge", strconv.Itoa(pullRequest.Number),
		"--repo", repositoryFromPullRequestURL(pullRequest.URL),
		"--merge",
	)
	return err
}

func pullRequestNumber(value string) (int, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return 0, err
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if parsed.Hostname() != "github.com" || len(parts) != 4 || parts[2] != "pull" {
		return 0, fmt.Errorf("%q is not a GitHub pull request URL", value)
	}
	number, err := strconv.Atoi(parts[3])
	if err != nil {
		return 0, fmt.Errorf("%q has an invalid pull request number", value)
	}
	return number, nil
}

func repositoryFromPullRequestURL(value string) string {
	parsed, _ := url.Parse(value)
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return ""
}
