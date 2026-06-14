package control

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/history"
	"github.com/davidtobonm/heracles/internal/implementation"
	"github.com/davidtobonm/heracles/internal/tracker"
)

func TestResumableBacklogSourceIncludesInProgressAttempts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := history.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.CreateLabor(ctx, history.NewLabor{ID: "labor-1", Problem: "Deliver backlog", Status: "implementing"}); err != nil {
		t.Fatalf("CreateLabor() error = %v", err)
	}
	if _, err := store.CreateIssueAttempt(ctx, history.NewIssueAttempt{
		ID: "attempt-1", LaborID: "labor-1", IssueURL: "https://github.com/acme/backlog/issues/7", Attempt: 1, Status: implementation.StatusWorkspaceReady,
	}); err != nil {
		t.Fatalf("CreateIssueAttempt() error = %v", err)
	}

	source := resumableBacklogSource{
		Source: fakeBacklogSource{
			ready: []tracker.Issue{backlogIssue(8, []string{tracker.LabelReady}, time.Unix(20, 0))},
			open: []tracker.Issue{
				backlogIssue(7, []string{tracker.LabelInProgress}, time.Unix(10, 0)),
				backlogIssue(8, []string{tracker.LabelReady}, time.Unix(20, 0)),
			},
		},
		History: store,
		LaborID: "labor-1",
	}

	issues, err := source.ReadyIssues(ctx)
	if err != nil {
		t.Fatalf("ReadyIssues() error = %v", err)
	}
	if len(issues) != 2 || issues[0].Number != 7 || issues[1].Number != 8 {
		t.Fatalf("issues = %#v, want resumable in-progress issue before ready issue", issues)
	}
}

func TestResumableBacklogSourceRejectsOrphanedInProgressIssue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := history.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	source := resumableBacklogSource{
		Source: fakeBacklogSource{
			open: []tracker.Issue{backlogIssue(7, []string{tracker.LabelInProgress}, time.Unix(10, 0))},
		},
		History: store,
		LaborID: "labor-1",
	}

	_, err = source.ReadyIssues(ctx)
	if err == nil || !strings.Contains(err.Error(), "no local Labor state exists") {
		t.Fatalf("ReadyIssues() error = %v, want orphaned in-progress issue guidance", err)
	}
}

type fakeBacklogSource struct {
	ready []tracker.Issue
	open  []tracker.Issue
}

func (source fakeBacklogSource) ReadyIssues(context.Context) ([]tracker.Issue, error) {
	return source.ready, nil
}
func (source fakeBacklogSource) OpenIssues(context.Context) ([]tracker.Issue, error) {
	return source.open, nil
}

func backlogIssue(number int, labels []string, createdAt time.Time) tracker.Issue {
	reference := tracker.Reference{Owner: "acme", Repo: "backlog", Number: number}
	return tracker.Issue{
		Reference: reference,
		URL:       reference.URL(),
		Labels:    labels,
		State:     tracker.StateOpen,
		CreatedAt: createdAt,
	}
}
