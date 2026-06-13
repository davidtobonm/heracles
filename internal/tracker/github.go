package tracker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

// CommandRunner executes GitHub CLI operations.
type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

// GitHubClient implements the tracker Client through the GitHub CLI.
type GitHubClient struct {
	runner CommandRunner
}

// NewGitHubClient creates a GitHub Issues client.
func NewGitHubClient(runner CommandRunner) *GitHubClient {
	return &GitHubClient{runner: runner}
}

// ListOpenIssues returns open issues from owner/repository.
func (client *GitHubClient) ListOpenIssues(ctx context.Context, repository string) ([]Issue, error) {
	output, err := client.runner.Run(ctx, "gh", "issue", "list",
		"--repo", repository,
		"--state", "open",
		"--limit", "200",
		"--json", "number,title,body,url,createdAt,labels,state",
	)
	if err != nil {
		return nil, err
	}
	var values []githubIssue
	if err := json.Unmarshal(output, &values); err != nil {
		return nil, fmt.Errorf("decode GitHub issue list: %w", err)
	}
	issues := make([]Issue, 0, len(values))
	for _, value := range values {
		issue, err := value.issue()
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

// Issue returns a single GitHub issue.
func (client *GitHubClient) Issue(ctx context.Context, reference Reference) (Issue, error) {
	output, err := client.runner.Run(ctx, "gh", "issue", "view", strconv.Itoa(reference.Number),
		"--repo", reference.Repository(),
		"--json", "number,title,body,url,createdAt,labels,state",
	)
	if err != nil {
		return Issue{}, err
	}
	var value githubIssue
	if err := json.Unmarshal(output, &value); err != nil {
		return Issue{}, fmt.Errorf("decode GitHub issue: %w", err)
	}
	return value.issue()
}

// SetLabels replaces Heracles-visible labels while preserving the requested complete label set.
func (client *GitHubClient) SetLabels(ctx context.Context, reference Reference, labels []string) error {
	current, err := client.Issue(ctx, reference)
	if err != nil {
		return err
	}
	desired := append([]string(nil), labels...)
	slices.Sort(desired)
	existing := append([]string(nil), current.Labels...)
	slices.Sort(existing)

	args := []string{"issue", "edit", strconv.Itoa(reference.Number), "--repo", reference.Repository()}
	for _, label := range existing {
		if !slices.Contains(desired, label) {
			args = append(args, "--remove-label", label)
		}
	}
	for _, label := range desired {
		if !slices.Contains(existing, label) {
			args = append(args, "--add-label", label)
		}
	}
	if len(args) == 5 {
		return nil
	}
	_, err = client.runner.Run(ctx, "gh", args...)
	return err
}

// Comment publishes shared issue status.
func (client *GitHubClient) Comment(ctx context.Context, reference Reference, body string) error {
	_, err := client.runner.Run(ctx, "gh", "issue", "comment", strconv.Itoa(reference.Number), "--repo", reference.Repository(), "--body", body)
	return err
}

type githubIssue struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
	State     State  `json:"state"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func (value githubIssue) issue() (Issue, error) {
	reference, err := ParseReference(value.URL)
	if err != nil {
		return Issue{}, err
	}
	createdAt, err := time.Parse(time.RFC3339, value.CreatedAt)
	if err != nil {
		return Issue{}, fmt.Errorf("parse GitHub issue creation time: %w", err)
	}
	labels := make([]string, 0, len(value.Labels))
	for _, label := range value.Labels {
		labels = append(labels, label.Name)
	}
	return Issue{
		Reference: reference,
		Title:     value.Title,
		Body:      value.Body,
		URL:       value.URL,
		Labels:    labels,
		State:     value.State,
		CreatedAt: createdAt,
	}, nil
}

// OSCommandRunner executes the GitHub CLI on the current machine.
type OSCommandRunner struct{}

// Run executes a command and returns stdout or an actionable failure.
func (OSCommandRunner) Run(ctx context.Context, command string, args ...string) ([]byte, error) {
	process := exec.CommandContext(ctx, command, args...)
	output, err := process.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w: %s", command, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
