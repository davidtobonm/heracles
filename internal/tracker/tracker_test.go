package tracker_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/davidtobonm/heracles/internal/tracker"
)

func TestDependenciesUseFullGitHubURLsFromBlockedBySection(t *testing.T) {
	t.Parallel()

	dependencies, err := tracker.Dependencies(`## What to build
Ignore https://github.com/acme/ignored/issues/1 here.

## Blocked by
- https://github.com/acme/backend/issues/12
- https://github.com/other/frontend/issues/9

## Exclusive Scopes
- api
`)
	if err != nil {
		t.Fatalf("Dependencies() error = %v", err)
	}
	if len(dependencies) != 2 || dependencies[0].Repository() != "acme/backend" || dependencies[0].Number != 12 || dependencies[1].Repository() != "other/frontend" {
		t.Errorf("dependencies = %#v, want cross-repository full URLs", dependencies)
	}
}

func TestReadyIssuesExcludeHITLAndUnresolvedDependencies(t *testing.T) {
	t.Parallel()

	client := newFakeClient(
		issue("acme/backlog", 1, []string{tracker.LabelReady}, ""),
		issue("acme/backlog", 2, []string{tracker.LabelReady, tracker.LabelHITL}, ""),
		issue("acme/backlog", 3, []string{tracker.LabelReady}, "## Blocked by\n- https://github.com/acme/backlog/issues/1\n"),
		issue("acme/backlog", 4, []string{tracker.LabelReady}, "## Blocked by\n- https://github.com/other/repo/issues/8\n"),
		closedIssue("other/repo", 8),
	)
	service := tracker.New("acme/backlog", client)

	ready, err := service.ReadyIssues(context.Background())
	if err != nil {
		t.Fatalf("ReadyIssues() error = %v", err)
	}
	if len(ready) != 2 || ready[0].Number != 1 || ready[1].Number != 4 {
		t.Errorf("ready issues = %#v, want only unblocked AFK issues", ready)
	}
}

func TestClaimAllowsOnlyOneConcurrentLaborAndPublishesSharedState(t *testing.T) {
	t.Parallel()

	client := newFakeClient(issue("acme/backlog", 7, []string{tracker.LabelReady, "enhancement"}, ""))
	services := []*tracker.Service{tracker.New("acme/backlog", client), tracker.New("acme/backlog", client)}
	reference := tracker.Reference{Owner: "acme", Repo: "backlog", Number: 7}

	var wait sync.WaitGroup
	wait.Add(2)
	errorsFound := make(chan error, 2)
	for index, laborID := range []string{"labor-a", "labor-b"} {
		go func() {
			defer wait.Done()
			_, err := services[index].Claim(context.Background(), reference, laborID)
			errorsFound <- err
		}()
	}
	wait.Wait()
	close(errorsFound)

	var successes, conflicts int
	for err := range errorsFound {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, tracker.ErrClaimed):
			conflicts++
		default:
			t.Fatalf("Claim() unexpected error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("claim outcomes = %d success, %d conflicts; want one each", successes, conflicts)
	}

	claimed, _ := client.Issue(context.Background(), reference)
	if !slices.Contains(claimed.Labels, tracker.LabelInProgress) || slices.Contains(claimed.Labels, tracker.LabelReady) || !slices.Contains(claimed.Labels, "enhancement") {
		t.Errorf("claimed labels = %#v, want in-progress plus unrelated labels", claimed.Labels)
	}
	if len(client.comments[reference.URL()]) != 1 {
		t.Errorf("claim comments = %#v, want one shared status comment", client.comments)
	}
}

func TestClaimRechecksDependenciesBeforeTransitioning(t *testing.T) {
	t.Parallel()

	client := newFakeClient(
		issue("acme/backlog", 1, []string{tracker.LabelReady}, ""),
		issue("acme/backlog", 2, []string{tracker.LabelReady}, "## Blocked by\n- https://github.com/acme/backlog/issues/1\n"),
	)
	service := tracker.New("acme/backlog", client)
	_, err := service.Claim(context.Background(), tracker.Reference{Owner: "acme", Repo: "backlog", Number: 2}, "labor-a")
	if !errors.Is(err, tracker.ErrBlocked) {
		t.Fatalf("Claim() error = %v, want ErrBlocked", err)
	}
}

type fakeClient struct {
	mu       sync.Mutex
	issues   map[string]tracker.Issue
	comments map[string][]string
}

func newFakeClient(issues ...tracker.Issue) *fakeClient {
	client := &fakeClient{issues: make(map[string]tracker.Issue), comments: make(map[string][]string)}
	for _, value := range issues {
		client.issues[value.Reference.URL()] = value
	}
	return client
}

func (client *fakeClient) ListOpenIssues(_ context.Context, repository string) ([]tracker.Issue, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	var issues []tracker.Issue
	for _, value := range client.issues {
		if value.Reference.Repository() == repository && value.State == tracker.StateOpen {
			issues = append(issues, value)
		}
	}
	slices.SortFunc(issues, func(left, right tracker.Issue) int { return left.Number - right.Number })
	return issues, nil
}

func (client *fakeClient) Issue(_ context.Context, reference tracker.Reference) (tracker.Issue, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	value, exists := client.issues[reference.URL()]
	if !exists {
		return tracker.Issue{}, errors.New("not found")
	}
	return value, nil
}

func (client *fakeClient) SetLabels(_ context.Context, reference tracker.Reference, labels []string) error {
	client.mu.Lock()
	defer client.mu.Unlock()
	value := client.issues[reference.URL()]
	value.Labels = append([]string(nil), labels...)
	client.issues[reference.URL()] = value
	return nil
}

func (client *fakeClient) Comment(_ context.Context, reference tracker.Reference, body string) error {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.comments[reference.URL()] = append(client.comments[reference.URL()], body)
	return nil
}

func issue(repository string, number int, labels []string, body string) tracker.Issue {
	reference, _ := tracker.ParseReference("https://github.com/" + repository + "/issues/" + strconv.Itoa(number))
	return tracker.Issue{Reference: reference, URL: reference.URL(), Labels: labels, Body: body, State: tracker.StateOpen}
}

func closedIssue(repository string, number int) tracker.Issue {
	value := issue(repository, number, nil, "")
	value.State = tracker.StateClosed
	return value
}
