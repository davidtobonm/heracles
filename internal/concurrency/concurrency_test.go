package concurrency_test

import (
	"testing"

	"github.com/davidtobonm/heracles/internal/concurrency"
	"github.com/davidtobonm/heracles/internal/scheduler"
)

func TestCandidatesAllowIndependentIssuesConcurrently(t *testing.T) {
	t.Parallel()

	candidates := concurrency.Candidates([]concurrency.Issue{
		{Key: "issue-1", Repositories: []string{"backend"}},
		{Key: "issue-2", Repositories: []string{"frontend"}},
	})

	batch := (scheduler.Scheduler{Concurrency: 2}).Select(candidates, nil)
	if len(batch) != 2 {
		t.Fatalf("Select() batch = %#v, want both independent issues", batch)
	}
}

func TestCandidatesSerializeRepositoryConflicts(t *testing.T) {
	t.Parallel()

	candidates := concurrency.Candidates([]concurrency.Issue{
		{Key: "issue-1", Repositories: []string{"backend"}},
		{Key: "issue-2", Repositories: []string{"backend"}},
	})

	batch := (scheduler.Scheduler{Concurrency: 2}).Select(candidates, nil)
	if len(batch) != 1 {
		t.Fatalf("Select() batch = %#v, want repository-conflicting issues serialized", batch)
	}
}

func TestCandidatesSerializeConflictKeySharing(t *testing.T) {
	t.Parallel()

	candidates := concurrency.Candidates([]concurrency.Issue{
		{Key: "issue-1", ConflictKeys: []string{"database-migration"}},
		{Key: "issue-2", ConflictKeys: []string{"database-migration"}},
	})

	batch := (scheduler.Scheduler{Concurrency: 2}).Select(candidates, nil)
	if len(batch) != 1 {
		t.Fatalf("Select() batch = %#v, want conflict-key-sharing issues serialized", batch)
	}
}

func TestCandidatesSerializeDependencyLinkedIssues(t *testing.T) {
	t.Parallel()

	candidates := concurrency.Candidates([]concurrency.Issue{
		{Key: "issue-1"},
		{Key: "issue-2", Dependencies: []string{"issue-1"}},
	})

	batch := (scheduler.Scheduler{Concurrency: 2}).Select(candidates, nil)
	if len(batch) != 1 || batch[0].Key != "issue-1" {
		t.Fatalf("Select() batch = %#v, want only the unblocked dependency", batch)
	}
}

func TestCandidatesSerializeUncertainIssueAgainstEverything(t *testing.T) {
	t.Parallel()

	candidates := concurrency.Candidates([]concurrency.Issue{
		{Key: "issue-1", Repositories: []string{"backend"}},
		{Key: "issue-2", Repositories: []string{"frontend"}},
		{Key: "issue-3", Uncertain: true},
	})

	scheduled := scheduler.Scheduler{Concurrency: 3}

	// However the Uncertain issue is ordered, it never shares a batch with
	// any other issue.
	for _, ordering := range [][]int{{0, 1, 2}, {2, 0, 1}, {0, 2, 1}} {
		ordered := make([]scheduler.Candidate, len(candidates))
		for index, position := range ordering {
			ordered[index] = candidates[position]
		}
		batch := scheduled.Select(ordered, nil)
		hasUncertain := false
		for _, candidate := range batch {
			if candidate.Key == "issue-3" {
				hasUncertain = true
			}
		}
		if hasUncertain && len(batch) != 1 {
			t.Fatalf("Select(%v) batch = %#v, want Uncertain issue to run alone", ordering, batch)
		}
	}
}
