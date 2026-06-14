package review_test

import (
	"context"
	"errors"
	"testing"

	"github.com/davidtobonm/heracles/internal/changeset"
	"github.com/davidtobonm/heracles/internal/review"
)

func TestReconcileReportsMergedOnceEveryPullRequestMerges(t *testing.T) {
	t.Parallel()

	client := &fakeClient{statusFor: map[string]changeset.PullRequestStatus{
		"backend":  {Merged: true},
		"frontend": {Merged: true},
	}}
	set := changeset.ChangeSet{
		Status: changeset.StatusReview,
		PullRequests: []changeset.PullRequest{
			{Repository: "backend", Number: 1},
			{Repository: "frontend", Number: 2},
		},
	}
	outcome, err := (review.Service{Client: client}).Reconcile(context.Background(), set)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !outcome.Merged {
		t.Fatalf("outcome = %#v, want merged once every pull request merges", outcome)
	}
	for _, pullRequest := range outcome.PullRequests {
		if !pullRequest.Merged {
			t.Errorf("pull request = %#v, want merged", pullRequest)
		}
	}
}

func TestReconcileReportsStillAwaitingReviewWhenAnyPullRequestIsOpen(t *testing.T) {
	t.Parallel()

	client := &fakeClient{statusFor: map[string]changeset.PullRequestStatus{
		"backend":  {Merged: true},
		"frontend": {Merged: false},
	}}
	set := changeset.ChangeSet{
		Status: changeset.StatusReview,
		PullRequests: []changeset.PullRequest{
			{Repository: "backend", Number: 1},
			{Repository: "frontend", Number: 2},
		},
	}
	outcome, err := (review.Service{Client: client}).Reconcile(context.Background(), set)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if outcome.Merged {
		t.Fatalf("outcome = %#v, want not merged while a pull request remains open", outcome)
	}
	if !outcome.PullRequests[0].Merged || outcome.PullRequests[1].Merged {
		t.Errorf("pull requests = %#v, want only backend merged", outcome.PullRequests)
	}
}

func TestReconcileSkipsAlreadyMergedPullRequests(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	set := changeset.ChangeSet{
		Status:       changeset.StatusReview,
		PullRequests: []changeset.PullRequest{{Repository: "backend", Number: 1, Merged: true}},
	}
	outcome, err := (review.Service{Client: client}).Reconcile(context.Background(), set)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !outcome.Merged || len(client.calls) != 0 {
		t.Errorf("outcome/calls = %#v / %v, want no status calls for already merged pull requests", outcome, client.calls)
	}
}

func TestReconcilePropagatesStatusErrors(t *testing.T) {
	t.Parallel()

	client := &fakeClient{err: errors.New("status check failed")}
	set := changeset.ChangeSet{PullRequests: []changeset.PullRequest{{Repository: "backend", Number: 1}}}
	if _, err := (review.Service{Client: client}).Reconcile(context.Background(), set); err == nil {
		t.Fatal("Reconcile() error = nil, want propagated status error")
	}
}

type fakeClient struct {
	statusFor map[string]changeset.PullRequestStatus
	calls     []string
	err       error
}

func (client *fakeClient) Status(_ context.Context, pullRequest changeset.PullRequest) (changeset.PullRequestStatus, error) {
	client.calls = append(client.calls, pullRequest.Repository)
	if client.err != nil {
		return changeset.PullRequestStatus{}, client.err
	}
	return client.statusFor[pullRequest.Repository], nil
}
