package changeset

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/davidtobonm/heracles/internal/ci"
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

// Status reports a pull request's merge, review, and required-check state.
func (client *GitHubClient) Status(ctx context.Context, pullRequest PullRequest) (PullRequestStatus, error) {
	output, err := client.runner.Run(ctx, "gh", "pr", "view", strconv.Itoa(pullRequest.Number),
		"--repo", repositoryFromPullRequestURL(pullRequest.URL),
		"--json", "mergedAt,reviewDecision,statusCheckRollup",
	)
	if err != nil {
		return PullRequestStatus{}, err
	}
	var decoded struct {
		MergedAt          *string `json:"mergedAt"`
		ReviewDecision    string  `json:"reviewDecision"`
		StatusCheckRollup []struct {
			Name       string `json:"name"`
			Context    string `json:"context"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			State      string `json:"state"`
		} `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal(output, &decoded); err != nil {
		return PullRequestStatus{}, fmt.Errorf("decode pull request status: %w", err)
	}
	status := PullRequestStatus{
		Merged:           decoded.MergedAt != nil && *decoded.MergedAt != "",
		ChangesRequested: decoded.ReviewDecision == "CHANGES_REQUESTED",
	}
	for _, check := range decoded.StatusCheckRollup {
		name := check.Name
		if name == "" {
			name = check.Context
		}
		conclusion := strings.ToLower(check.Conclusion)
		if conclusion == "" {
			conclusion = strings.ToLower(statusContextConclusion(check.State))
		}
		if conclusion == "success" || conclusion == "neutral" || conclusion == "skipped" {
			continue
		}
		status.FailedChecks = append(status.FailedChecks, ci.Check{
			Name: name, Status: strings.ToLower(check.Status), Conclusion: conclusion,
		})
	}
	return status, nil
}

// statusContextConclusion maps a legacy commit status context's state to a
// check-run-style conclusion.
func statusContextConclusion(state string) string {
	switch strings.ToUpper(state) {
	case "SUCCESS":
		return "success"
	case "ERROR", "FAILURE":
		return "failure"
	default:
		return ""
	}
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
