// Package changeset delivers one issue through linked repository pull requests.
package changeset

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/davidtobonm/heracles/internal/ci"
)

const (
	StatusOpen    = "open"
	StatusReview  = "review"
	StatusMerged  = "merged"
	StatusBlocked = "blocked"
)

// Repository describes one potential pull request in a Change Set.
type Repository struct {
	Name          string
	GitHub        string
	Head          string
	Base          string
	Touched       bool
	Verified      bool
	ReviewSummary string
	QASteps       []string
	Evidence      []string
}

// Request describes one issue delivery.
type Request struct {
	ID           string
	IssueURL     string
	Repositories []Repository
}

// Policy controls opt-in automatic merging.
type Policy struct {
	AutoMerge  bool
	MergeOrder []string
}

// PullRequestInput describes a pull request to create.
type PullRequestInput struct {
	Repository Repository
	Title      string
	Body       string
}

// PullRequest is one linked repository delivery.
type PullRequest struct {
	Repository string
	URL        string
	Number     int
	Body       string
	Merged     bool
}

// ChangeSet is the complete linked delivery for one issue.
type ChangeSet struct {
	ID           string
	IssueURL     string
	Status       string
	PullRequests []PullRequest
	// Correction describes a retryable delivery failure when Status is
	// StatusBlocked, per PRD.md's correction-cycle policy. It is nil for
	// failures that are not retryable through a correction cycle.
	Correction *Correction
}

// Correction describes one retryable delivery failure for a single
// repository's pull request.
type Correction struct {
	Repository       string
	RequestedChanges bool
	FailedChecks     []ci.Check
	Classification   ci.Classification
	Reason           string
}

// PullRequestStatus is the current merge, review, and CI state of one pull request.
type PullRequestStatus struct {
	Merged           bool
	ChangesRequested bool
	FailedChecks     []ci.Check
}

// Client provides pull request and CI operations.
type Client interface {
	CreatePullRequest(context.Context, PullRequestInput) (PullRequest, error)
	UpdatePullRequestBody(context.Context, PullRequest, string) error
	WaitForCI(context.Context, PullRequest) error
	Status(context.Context, PullRequest) (PullRequestStatus, error)
	Merge(context.Context, PullRequest) error
}

// Service delivers Change Sets.
type Service struct {
	Client Client
	Policy Policy
}

// Deliver prepares linked pull requests and optionally merges them in configured order.
func (service Service) Deliver(ctx context.Context, request Request) (ChangeSet, error) {
	if service.Client == nil {
		return ChangeSet{}, errors.New("Change Set delivery requires a Client")
	}
	set := ChangeSet{ID: request.ID, IssueURL: request.IssueURL, Status: StatusOpen}
	repositories := make(map[string]Repository)
	for _, repository := range request.Repositories {
		if !repository.Touched {
			continue
		}
		if !repository.Verified {
			return set, fmt.Errorf("touched Target Repository %s has not passed local verification", repository.Name)
		}
		repositories[repository.Name] = repository
		pullRequest, err := service.Client.CreatePullRequest(ctx, PullRequestInput{
			Repository: repository,
			Title:      "Deliver " + request.IssueURL,
			Body:       body(request, repository, nil),
		})
		if err != nil {
			return set, fmt.Errorf("create %s pull request: %w", repository.Name, err)
		}
		set.PullRequests = append(set.PullRequests, pullRequest)
	}
	slices.SortFunc(set.PullRequests, func(left, right PullRequest) int { return strings.Compare(left.Repository, right.Repository) })

	for index, pullRequest := range set.PullRequests {
		repository := repositories[pullRequest.Repository]
		linkedBody := body(request, repository, set.PullRequests)
		if err := service.Client.UpdatePullRequestBody(ctx, pullRequest, linkedBody); err != nil {
			return set, fmt.Errorf("link %s pull request: %w", pullRequest.Repository, err)
		}
		set.PullRequests[index].Body = linkedBody
	}
	if !service.Policy.AutoMerge {
		set.Status = StatusReview
		return set, nil
	}

	for _, name := range mergeOrder(service.Policy.MergeOrder, set.PullRequests) {
		index := slices.IndexFunc(set.PullRequests, func(pullRequest PullRequest) bool { return pullRequest.Repository == name })
		pullRequest := set.PullRequests[index]
		// WaitForCI's error is diagnosed through Status below; a failed
		// required check surfaces there as a FailedChecks entry rather than
		// a Go error so it can be classified for a correction cycle.
		_ = service.Client.WaitForCI(ctx, pullRequest)
		status, err := service.Client.Status(ctx, pullRequest)
		if err != nil {
			set.Status = StatusBlocked
			set.Correction = &Correction{Repository: name, Classification: ci.Infrastructure, Reason: err.Error()}
			return set, fmt.Errorf("check %s pull request status: %w", name, err)
		}
		if status.Merged {
			set.PullRequests[index].Merged = true
			continue
		}
		if status.ChangesRequested {
			set.Status = StatusBlocked
			set.Correction = &Correction{Repository: name, RequestedChanges: true, Classification: ci.Code, Reason: "reviewer requested changes"}
			return set, fmt.Errorf("%s pull request has requested changes", name)
		}
		if len(status.FailedChecks) > 0 {
			set.Status = StatusBlocked
			set.Correction = &Correction{Repository: name, FailedChecks: status.FailedChecks, Classification: ci.Classify(status.FailedChecks), Reason: "required CI checks failed"}
			return set, fmt.Errorf("%s pull request failed required CI checks", name)
		}
		if err := service.Client.Merge(ctx, pullRequest); err != nil {
			set.Status = StatusBlocked
			set.Correction = &Correction{Repository: name, Classification: ci.Infrastructure, Reason: err.Error()}
			return set, fmt.Errorf("partial Change Set merge failed at %s: %w", name, err)
		}
		set.PullRequests[index].Merged = true
	}
	set.Status = StatusMerged
	return set, nil
}

func body(request Request, repository Repository, related []PullRequest) string {
	return fmt.Sprintf(`## Review Summary
%s

## QA
%s

## Evidence
%s

## Change Set
- Issue: %s
- Change Set: %s

## Related Pull Requests
%s
`, repository.ReviewSummary, bullets(repository.QASteps), bullets(repository.Evidence), request.IssueURL, request.ID, relatedLinks(related))
}

func bullets(values []string) string {
	if len(values) == 0 {
		return "- None"
	}
	lines := make([]string, len(values))
	for index, value := range values {
		lines[index] = "- " + value
	}
	return strings.Join(lines, "\n")
}

func relatedLinks(pullRequests []PullRequest) string {
	if len(pullRequests) == 0 {
		return "- Pending"
	}
	lines := make([]string, len(pullRequests))
	for index, pullRequest := range pullRequests {
		lines[index] = "- " + pullRequest.Repository + ": " + pullRequest.URL
	}
	return strings.Join(lines, "\n")
}

func mergeOrder(configured []string, pullRequests []PullRequest) []string {
	available := make(map[string]bool, len(pullRequests))
	for _, pullRequest := range pullRequests {
		available[pullRequest.Repository] = true
	}
	var order []string
	for _, name := range configured {
		if available[name] {
			order = append(order, name)
			delete(available, name)
		}
	}
	var remaining []string
	for name := range available {
		remaining = append(remaining, name)
	}
	slices.Sort(remaining)
	return append(order, remaining...)
}
