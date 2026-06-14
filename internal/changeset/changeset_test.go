package changeset_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/changeset"
	"github.com/davidtobonm/heracles/internal/ci"
)

func TestDeliverCreatesLinkedPRsForTouchedRepositoriesWithoutAutoMerge(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	set, err := (changeset.Service{Client: client}).Deliver(context.Background(), changeset.Request{
		ID:       "change-1",
		IssueURL: "https://github.com/acme/backlog/issues/7",
		Repositories: []changeset.Repository{
			{Name: "backend", GitHub: "acme/backend", Head: "heracles/issue-7", Base: "main", Touched: true, Verified: true, ReviewSummary: "Backend work", QASteps: []string{"go test ./..."}, Evidence: []string{"red.json", "green.json"}},
			{Name: "frontend", GitHub: "acme/frontend", Head: "heracles/issue-7", Base: "main", Touched: false},
			{Name: "shared", GitHub: "acme/shared", Head: "heracles/issue-7", Base: "main", Touched: true, Verified: true, ReviewSummary: "Shared work", QASteps: []string{"npm test"}, Evidence: []string{"evidence.json"}},
		},
	})
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if len(set.PullRequests) != 2 || len(client.created) != 2 || len(client.merged) != 0 {
		t.Errorf("Change Set = %#v, want two open PRs and no automatic merge", set)
	}
	if set.Status != changeset.StatusReview {
		t.Errorf("status = %q, want %q for a Change Set awaiting manual review", set.Status, changeset.StatusReview)
	}
	for _, body := range client.updatedBodies {
		for _, expected := range []string{"## Review Summary", "## QA", "## Evidence", "## Related Pull Requests", "acme/backend", "acme/shared"} {
			if !strings.Contains(body, expected) {
				t.Errorf("PR body does not contain %q: %s", expected, body)
			}
		}
	}
}

func TestOrderedAutoMergeBlocksHonestlyAfterPartialFailure(t *testing.T) {
	t.Parallel()

	client := &fakeClient{mergeFailure: "shared"}
	set, err := (changeset.Service{Client: client, Policy: changeset.Policy{AutoMerge: true, MergeOrder: []string{"backend", "shared", "frontend"}}}).Deliver(context.Background(), changeset.Request{
		ID:       "change-1",
		IssueURL: "https://github.com/acme/backlog/issues/7",
		Repositories: []changeset.Repository{
			{Name: "frontend", GitHub: "acme/frontend", Touched: true, Verified: true},
			{Name: "shared", GitHub: "acme/shared", Touched: true, Verified: true},
			{Name: "backend", GitHub: "acme/backend", Touched: true, Verified: true},
		},
	})
	if err == nil || set.Status != changeset.StatusBlocked {
		t.Fatalf("Deliver() = %#v, %v; want blocked partial merge", set, err)
	}
	if strings.Join(client.merged, ",") != "backend" {
		t.Errorf("merged repositories = %#v, want only backend before failure", client.merged)
	}
	if strings.Join(client.waited, ",") != "backend,shared" {
		t.Errorf("CI wait order = %#v, want configured merge order until failure", client.waited)
	}
}

func TestDeliverBlocksWithCorrectionOnRequestedChanges(t *testing.T) {
	t.Parallel()

	client := &fakeClient{statusFor: map[string]changeset.PullRequestStatus{
		"backend": {ChangesRequested: true},
	}}
	set, err := (changeset.Service{Client: client, Policy: changeset.Policy{AutoMerge: true}}).Deliver(context.Background(), changeset.Request{
		ID:       "change-1",
		IssueURL: "https://github.com/acme/backlog/issues/7",
		Repositories: []changeset.Repository{
			{Name: "backend", GitHub: "acme/backend", Touched: true, Verified: true},
		},
	})
	if err == nil || set.Status != changeset.StatusBlocked {
		t.Fatalf("Deliver() = %#v, %v; want blocked on requested changes", set, err)
	}
	if set.Correction == nil || !set.Correction.RequestedChanges || set.Correction.Classification != ci.Code {
		t.Errorf("Correction = %#v, want requested changes classified as code", set.Correction)
	}
	if len(client.merged) != 0 {
		t.Errorf("merged repositories = %#v, want none while changes are requested", client.merged)
	}
}

func TestDeliverBlocksWithCorrectionOnFailedRequiredChecks(t *testing.T) {
	t.Parallel()

	client := &fakeClient{statusFor: map[string]changeset.PullRequestStatus{
		"backend": {FailedChecks: []ci.Check{{Name: "test", Status: "completed", Conclusion: "failure"}}},
	}}
	set, err := (changeset.Service{Client: client, Policy: changeset.Policy{AutoMerge: true}}).Deliver(context.Background(), changeset.Request{
		ID:       "change-1",
		IssueURL: "https://github.com/acme/backlog/issues/7",
		Repositories: []changeset.Repository{
			{Name: "backend", GitHub: "acme/backend", Touched: true, Verified: true},
		},
	})
	if err == nil || set.Status != changeset.StatusBlocked {
		t.Fatalf("Deliver() = %#v, %v; want blocked on failed required checks", set, err)
	}
	if set.Correction == nil || set.Correction.Classification != ci.Code {
		t.Errorf("Correction = %#v, want failed test classified as code", set.Correction)
	}
}

func TestDeliverBlocksWithInfrastructureCorrectionOnFailedRequiredChecks(t *testing.T) {
	t.Parallel()

	client := &fakeClient{statusFor: map[string]changeset.PullRequestStatus{
		"backend": {FailedChecks: []ci.Check{{Name: "build", Status: "completed", Conclusion: "cancelled"}}},
	}}
	set, err := (changeset.Service{Client: client, Policy: changeset.Policy{AutoMerge: true}}).Deliver(context.Background(), changeset.Request{
		ID:       "change-1",
		IssueURL: "https://github.com/acme/backlog/issues/7",
		Repositories: []changeset.Repository{
			{Name: "backend", GitHub: "acme/backend", Touched: true, Verified: true},
		},
	})
	if err == nil || set.Status != changeset.StatusBlocked {
		t.Fatalf("Deliver() = %#v, %v; want blocked on failed required checks", set, err)
	}
	if set.Correction == nil || set.Correction.Classification != ci.Infrastructure {
		t.Errorf("Correction = %#v, want cancelled check classified as infrastructure", set.Correction)
	}
}

func TestDeliverRejectsUnverifiedTouchedRepository(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	set, err := (changeset.Service{Client: client}).Deliver(context.Background(), changeset.Request{
		ID:       "change-1",
		IssueURL: "https://github.com/acme/backlog/issues/7",
		Repositories: []changeset.Repository{
			{Name: "backend", GitHub: "acme/backend", Touched: true, Verified: false},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "local verification") {
		t.Fatalf("Deliver() = %#v, %v; want local verification failure", set, err)
	}
	if len(client.created) != 0 {
		t.Errorf("created PRs = %#v, want none", client.created)
	}
}

type fakeClient struct {
	created       []string
	updatedBodies []string
	waited        []string
	merged        []string
	mergeFailure  string
	statusFor     map[string]changeset.PullRequestStatus
}

func (client *fakeClient) CreatePullRequest(_ context.Context, input changeset.PullRequestInput) (changeset.PullRequest, error) {
	client.created = append(client.created, input.Repository.Name)
	return changeset.PullRequest{Repository: input.Repository.Name, URL: "https://github.com/" + input.Repository.GitHub + "/pull/1", Number: 1, Body: input.Body}, nil
}
func (client *fakeClient) UpdatePullRequestBody(_ context.Context, pullRequest changeset.PullRequest, body string) error {
	client.updatedBodies = append(client.updatedBodies, body)
	return nil
}
func (client *fakeClient) WaitForCI(_ context.Context, pullRequest changeset.PullRequest) error {
	client.waited = append(client.waited, pullRequest.Repository)
	return nil
}
func (client *fakeClient) Status(_ context.Context, pullRequest changeset.PullRequest) (changeset.PullRequestStatus, error) {
	if status, ok := client.statusFor[pullRequest.Repository]; ok {
		return status, nil
	}
	return changeset.PullRequestStatus{}, nil
}
func (client *fakeClient) Merge(_ context.Context, pullRequest changeset.PullRequest) error {
	if pullRequest.Repository == client.mergeFailure {
		return errors.New("merge failed")
	}
	client.merged = append(client.merged, pullRequest.Repository)
	return nil
}
