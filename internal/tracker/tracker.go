// Package tracker coordinates Heracles-compatible issues behind a tracker boundary.
package tracker

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	LabelReady      = "heracles:ready"
	LabelBlocked    = "heracles:blocked"
	LabelInProgress = "heracles:in-progress"
	LabelDone       = "heracles:done"
	LabelHITL       = "heracles:hitl"
	LabelTDDExempt  = "heracles:tdd-exempt"
)

var (
	// ErrClaimed indicates a Ready Issue was claimed before this Labor acquired it.
	ErrClaimed = errors.New("Ready Issue is already claimed or no longer eligible")
	// ErrBlocked indicates an issue still has unresolved dependencies.
	ErrBlocked = errors.New("Ready Issue has unresolved dependencies")
	claimLocks sync.Map
)

// State is the remote GitHub issue state.
type State string

const (
	StateOpen   State = "OPEN"
	StateClosed State = "CLOSED"
)

// Reference is a full GitHub issue identity.
type Reference struct {
	Owner  string
	Repo   string
	Number int
}

// Repository returns owner/repository.
func (reference Reference) Repository() string {
	return reference.Owner + "/" + reference.Repo
}

// URL returns the full GitHub issue URL.
func (reference Reference) URL() string {
	return "https://github.com/" + reference.Repository() + "/issues/" + strconv.Itoa(reference.Number)
}

// Issue is the tracker-neutral Heracles issue contract.
type Issue struct {
	Reference
	Title     string
	Body      string
	URL       string
	Labels    []string
	State     State
	CreatedAt time.Time
}

// Client is the GitHub-specific operation boundary used by the tracker service.
type Client interface {
	ListOpenIssues(context.Context, string) ([]Issue, error)
	Issue(context.Context, Reference) (Issue, error)
	SetLabels(context.Context, Reference, []string) error
	Comment(context.Context, Reference, string) error
}

// Service coordinates the configured GitHub Issue Tracker.
type Service struct {
	repository string
	client     Client
}

// New creates a tracker service for owner/repository.
func New(repository string, client Client) *Service {
	return &Service{repository: repository, client: client}
}

// ParseReference parses a full GitHub issue URL.
func ParseReference(value string) (Reference, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "github.com" {
		return Reference{}, fmt.Errorf("dependency %q must be a full https://github.com issue URL", value)
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) != 4 || segments[2] != "issues" {
		return Reference{}, fmt.Errorf("dependency %q must be a full GitHub issue URL", value)
	}
	number, err := strconv.Atoi(segments[3])
	if err != nil || number < 1 {
		return Reference{}, fmt.Errorf("dependency %q has invalid issue number", value)
	}
	return Reference{Owner: segments[0], Repo: segments[1], Number: number}, nil
}

// Dependencies parses full issue URLs from an issue's Blocked by section.
func Dependencies(body string) ([]Reference, error) {
	var references []Reference
	inBlockedBy := false
	for _, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "## ") {
			inBlockedBy = strings.EqualFold(line, "## Blocked by")
			continue
		}
		if !inBlockedBy || !strings.HasPrefix(line, "-") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "-"))
		if value == "" || strings.HasPrefix(strings.ToLower(value), "none") {
			continue
		}
		reference, err := ParseReference(value)
		if err != nil {
			return nil, err
		}
		references = append(references, reference)
	}
	return references, nil
}

// ReadyIssues returns unblocked AFK Ready Issues in deterministic order.
func (service *Service) ReadyIssues(ctx context.Context) ([]Issue, error) {
	issues, err := service.client.ListOpenIssues(ctx, service.repository)
	if err != nil {
		return nil, fmt.Errorf("list Ready Issues: %w", err)
	}

	var ready []Issue
	for _, issue := range issues {
		if !isReady(issue) {
			continue
		}
		resolved, err := service.dependenciesResolved(ctx, issue)
		if err != nil {
			return nil, err
		}
		if resolved {
			ready = append(ready, issue)
		}
	}
	slices.SortFunc(ready, func(left, right Issue) int {
		if left.CreatedAt.Equal(right.CreatedAt) {
			return left.Number - right.Number
		}
		if left.CreatedAt.Before(right.CreatedAt) {
			return -1
		}
		return 1
	})
	return ready, nil
}

// Claim transitions one Ready Issue to the shared claimed state.
func (service *Service) Claim(ctx context.Context, reference Reference, laborID string) (Issue, error) {
	lock := claimLock(reference)
	lock.Lock()
	defer lock.Unlock()

	issue, err := service.client.Issue(ctx, reference)
	if err != nil {
		return Issue{}, fmt.Errorf("read Ready Issue before claim: %w", err)
	}
	if !isReady(issue) {
		return Issue{}, fmt.Errorf("%w: %s", ErrClaimed, reference.URL())
	}
	resolved, err := service.dependenciesResolved(ctx, issue)
	if err != nil {
		return Issue{}, err
	}
	if !resolved {
		return Issue{}, fmt.Errorf("%w: %s", ErrBlocked, reference.URL())
	}
	issue.Labels = withState(issue.Labels, LabelInProgress)
	if err := service.client.SetLabels(ctx, reference, issue.Labels); err != nil {
		return Issue{}, fmt.Errorf("claim Ready Issue: %w", err)
	}
	if err := service.client.Comment(ctx, reference, fmt.Sprintf("Claimed by Heracles Labor `%s`.", laborID)); err != nil {
		return Issue{}, fmt.Errorf("publish claim comment: %w", err)
	}
	return issue, nil
}

func claimLock(reference Reference) *sync.Mutex {
	lock, _ := claimLocks.LoadOrStore(reference.URL(), &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func (service *Service) dependenciesResolved(ctx context.Context, issue Issue) (bool, error) {
	dependencies, err := Dependencies(issue.Body)
	if err != nil {
		return false, fmt.Errorf("parse dependencies for %s: %w", issue.Reference.URL(), err)
	}
	for _, dependency := range dependencies {
		value, err := service.client.Issue(ctx, dependency)
		if err != nil {
			return false, fmt.Errorf("read dependency %s: %w", dependency.URL(), err)
		}
		if value.State != StateClosed && !hasLabel(value.Labels, LabelDone) {
			return false, nil
		}
	}
	return true, nil
}

// Block marks an issue blocked and publishes an actionable shared comment.
func (service *Service) Block(ctx context.Context, reference Reference, reason string) error {
	return service.transition(ctx, reference, LabelBlocked, "Blocked by Heracles: "+reason)
}

// Complete marks an issue done and publishes a shared completion comment.
func (service *Service) Complete(ctx context.Context, reference Reference, summary string) error {
	return service.transition(ctx, reference, LabelDone, "Completed by Heracles: "+summary)
}

func (service *Service) transition(ctx context.Context, reference Reference, label, comment string) error {
	issue, err := service.client.Issue(ctx, reference)
	if err != nil {
		return err
	}
	if err := service.client.SetLabels(ctx, reference, withState(issue.Labels, label)); err != nil {
		return err
	}
	return service.client.Comment(ctx, reference, comment)
}

func isReady(issue Issue) bool {
	return issue.State == StateOpen &&
		hasLabel(issue.Labels, LabelReady) &&
		!hasAnyLabel(issue.Labels, LabelBlocked, LabelInProgress, LabelDone, LabelHITL)
}

func withState(labels []string, state string) []string {
	result := make([]string, 0, len(labels)+1)
	for _, label := range labels {
		if !hasAnyLabel([]string{label}, LabelReady, LabelBlocked, LabelInProgress, LabelDone, LabelHITL) {
			result = append(result, label)
		}
	}
	result = append(result, state)
	slices.Sort(result)
	return result
}

func hasLabel(labels []string, expected string) bool {
	return slices.Contains(labels, expected)
}

func hasAnyLabel(labels []string, expected ...string) bool {
	for _, label := range labels {
		if slices.Contains(expected, label) {
			return true
		}
	}
	return false
}
