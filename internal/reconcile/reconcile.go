// Package reconcile decides how a freshly proposed set of implementation
// issues maps onto the implementation issues already published for a Parent
// PRD, per ADR 0025.
package reconcile

// Existing describes one previously published implementation issue linked to
// a Parent PRD by its Issue Author-assigned semantic ID.
type Existing struct {
	SemanticID string
	URL        string
	InProgress bool
	Done       bool
}

// Action is the reconciliation outcome for one semantic issue ID.
type Action string

const (
	// ActionCreate publishes a new implementation issue.
	ActionCreate Action = "create"
	// ActionUpdate replaces the title, body, and labels of an untouched
	// existing implementation issue.
	ActionUpdate Action = "update"
	// ActionObsolete marks a removed, untouched implementation issue
	// heracles:obsolete without closing it.
	ActionObsolete Action = "obsolete"
	// ActionSkip leaves an in-progress or completed implementation issue
	// untouched.
	ActionSkip Action = "skip"
)

// Decision is the reconciliation outcome for one semantic issue ID.
type Decision struct {
	SemanticID string
	Action     Action
	Existing   Existing
}

// Plan reconciles existing implementation issues linked to a Parent PRD
// against the semantic IDs of a freshly proposed issue set. Proposed IDs not
// yet published are created; proposed IDs matching an untouched existing
// issue are updated; proposed IDs matching an in-progress or completed
// existing issue are skipped; existing issues with no matching proposal are
// marked obsolete unless they are in progress or completed, in which case
// they are skipped.
func Plan(existing []Existing, proposalIDs []string) []Decision {
	byID := make(map[string]Existing, len(existing))
	for _, item := range existing {
		byID[item.SemanticID] = item
	}
	proposed := make(map[string]bool, len(proposalIDs))
	for _, id := range proposalIDs {
		proposed[id] = true
	}

	decisions := make([]Decision, 0, len(proposalIDs)+len(existing))
	for _, id := range proposalIDs {
		item, ok := byID[id]
		switch {
		case !ok:
			decisions = append(decisions, Decision{SemanticID: id, Action: ActionCreate})
		case item.InProgress || item.Done:
			decisions = append(decisions, Decision{SemanticID: id, Action: ActionSkip, Existing: item})
		default:
			decisions = append(decisions, Decision{SemanticID: id, Action: ActionUpdate, Existing: item})
		}
	}
	for _, item := range existing {
		if proposed[item.SemanticID] {
			continue
		}
		if item.InProgress || item.Done {
			decisions = append(decisions, Decision{SemanticID: item.SemanticID, Action: ActionSkip, Existing: item})
			continue
		}
		decisions = append(decisions, Decision{SemanticID: item.SemanticID, Action: ActionObsolete, Existing: item})
	}
	return decisions
}
