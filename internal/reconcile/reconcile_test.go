package reconcile_test

import (
	"testing"

	"github.com/davidtobonm/heracles/internal/reconcile"
)

func TestPlanCreatesNewProposals(t *testing.T) {
	t.Parallel()

	decisions := reconcile.Plan(nil, []string{"auth-login-flow"})
	if len(decisions) != 1 || decisions[0].Action != reconcile.ActionCreate || decisions[0].SemanticID != "auth-login-flow" {
		t.Fatalf("Plan() = %#v, want one create decision", decisions)
	}
}

func TestPlanUpdatesUntouchedExistingIssues(t *testing.T) {
	t.Parallel()

	existing := []reconcile.Existing{{SemanticID: "auth-login-flow", URL: "https://github.com/acme/backlog/issues/10"}}
	decisions := reconcile.Plan(existing, []string{"auth-login-flow"})
	if len(decisions) != 1 || decisions[0].Action != reconcile.ActionUpdate || decisions[0].Existing.URL != existing[0].URL {
		t.Fatalf("Plan() = %#v, want one update decision", decisions)
	}
}

func TestPlanSkipsInProgressAndDoneMatches(t *testing.T) {
	t.Parallel()

	existing := []reconcile.Existing{
		{SemanticID: "auth-login-flow", InProgress: true},
		{SemanticID: "auth-logout-flow", Done: true},
	}
	decisions := reconcile.Plan(existing, []string{"auth-login-flow", "auth-logout-flow"})
	for _, decision := range decisions {
		if decision.Action != reconcile.ActionSkip {
			t.Errorf("decision = %#v, want skip for in-progress/done matches", decision)
		}
	}
}

func TestPlanMarksRemovedUntouchedIssuesObsolete(t *testing.T) {
	t.Parallel()

	existing := []reconcile.Existing{{SemanticID: "old-feature", URL: "https://github.com/acme/backlog/issues/4"}}
	decisions := reconcile.Plan(existing, nil)
	if len(decisions) != 1 || decisions[0].Action != reconcile.ActionObsolete || decisions[0].SemanticID != "old-feature" {
		t.Fatalf("Plan() = %#v, want one obsolete decision", decisions)
	}
}

func TestPlanSkipsRemovedInProgressOrDoneIssues(t *testing.T) {
	t.Parallel()

	existing := []reconcile.Existing{
		{SemanticID: "in-flight", InProgress: true},
		{SemanticID: "shipped", Done: true},
	}
	decisions := reconcile.Plan(existing, nil)
	for _, decision := range decisions {
		if decision.Action != reconcile.ActionSkip {
			t.Errorf("decision = %#v, want skip for removed in-progress/done issues", decision)
		}
	}
}
