// Package labor composes Planning, Issue, and Implementation Stages.
package labor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/davidtobonm/heracles/internal/implementation"
	"github.com/davidtobonm/heracles/internal/issuestage"
	"github.com/davidtobonm/heracles/internal/planning"
)

const (
	StatusNew                      = "new"
	StatusPlanning                 = "planning"
	StatusAwaitingPlanningApproval = "awaiting_planning_approval"
	StatusIssues                   = "issues"
	StatusAwaitingIssueApproval    = "awaiting_issue_approval"
	StatusImplementing             = "implementing"
	StatusBlocked                  = "blocked"
	StatusCompleted                = "completed"
	StatusCancelled                = "cancelled"

	GatePending  = "pending"
	GateApproved = "approved"
	GateRejected = "rejected"

	DecisionApprove = "approve"
	DecisionReject  = "reject"
)

// ErrNotFound indicates that no durable Labor exists for an ID.
var ErrNotFound = errors.New("Labor not found")

// Gate is one distinct durable human approval.
type Gate struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`
	Decision string `json:"decision,omitempty"`
}

// Event records one durable Labor transition.
type Event struct {
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// State is the complete durable cross-stage Labor state.
type State struct {
	ID                string                       `json:"id"`
	Problem           string                       `json:"problem"`
	TrackerRepository string                       `json:"tracker_repository"`
	Repositories      []planning.RepositoryContext `json:"repositories,omitempty"`
	Documents         []planning.Document          `json:"documents,omitempty"`
	Status            string                       `json:"status"`
	Planning          planning.State               `json:"planning"`
	Issues            issuestage.State             `json:"issues"`
	Backlog           implementation.BacklogResult `json:"backlog"`
	PlanningGate      Gate                         `json:"planning_gate"`
	IssueGate         Gate                         `json:"issue_gate"`
	Events            []Event                      `json:"events,omitempty"`
}

// Request starts or resumes a Labor.
type Request struct {
	ID                string
	Problem           string
	TrackerRepository string
	Repositories      []planning.RepositoryContext
	Documents         []planning.Document
}

// Store persists Labor state.
type Store interface {
	Load(context.Context, string) (State, error)
	Save(context.Context, State) error
}

// PlanningStage is the Planning application boundary.
type PlanningStage interface {
	Run(context.Context, planning.RunRequest) (planning.State, error)
	Decide(context.Context, string, string, string) (planning.State, error)
}

// IssueStage is the Issue application boundary.
type IssueStage interface {
	Run(context.Context, issuestage.RunRequest) (issuestage.State, error)
	Decide(context.Context, string, string, string) (issuestage.State, error)
	Publish(context.Context, string) (issuestage.State, error)
}

// ImplementationStage is the defined-backlog application boundary.
type ImplementationStage interface {
	Run(context.Context) (implementation.BacklogResult, error)
}

// Service composes the three stages into one durable Labor.
type Service struct {
	Store          Store
	Planning       PlanningStage
	Issues         IssueStage
	Implementation ImplementationStage
}

// Run starts or resumes a Labor until its next Approval Gate or terminal state.
func (service Service) Run(ctx context.Context, request Request) (State, error) {
	if service.Store == nil || service.Planning == nil || service.Issues == nil || service.Implementation == nil {
		return State{}, errors.New("Labor requires Store, Planning, Issue, and Implementation Stages")
	}
	state, err := service.Store.Load(ctx, request.ID)
	if errors.Is(err, ErrNotFound) {
		if request.ID == "" || request.Problem == "" || request.TrackerRepository == "" {
			return State{}, errors.New("new Labor requires ID, problem, and Issue Tracker repository")
		}
		state = State{
			ID: request.ID, Problem: request.Problem, TrackerRepository: request.TrackerRepository,
			Repositories: append([]planning.RepositoryContext(nil), request.Repositories...),
			Documents:    append([]planning.Document(nil), request.Documents...),
			Status:       StatusNew,
			PlanningGate: Gate{ID: request.ID + "-planning-approval", Kind: "planning"},
			IssueGate:    Gate{ID: request.ID + "-issue-publication-approval", Kind: "issue_publication"},
		}
		if err := service.record(ctx, &state, StatusNew, "Labor created"); err != nil {
			return state, err
		}
	} else if err != nil {
		return State{}, err
	}
	return service.progress(ctx, state)
}

// ContinuePlanning supplies answers or explicit permission beyond the Question Budget.
func (service Service) ContinuePlanning(ctx context.Context, id string, answers []string, permitBeyondBudget bool) (State, error) {
	state, err := service.Store.Load(ctx, id)
	if err != nil {
		return State{}, err
	}
	if state.Status != StatusPlanning {
		return state, fmt.Errorf("Labor %q is not actively planning", id)
	}
	planningState, err := service.Planning.Run(ctx, planning.RunRequest{
		ID: state.ID + "-planning", Answers: answers, PermitBeyondBudget: permitBeyondBudget,
	})
	state.Planning = planningState
	if saveErr := service.Store.Save(ctx, state); saveErr != nil {
		return state, errors.Join(err, saveErr)
	}
	if err != nil {
		return state, err
	}
	return service.progress(ctx, state)
}

// DecidePlanning approves or rejects the Planning PRD.
func (service Service) DecidePlanning(ctx context.Context, id, decision, reason string) (State, error) {
	state, err := service.Store.Load(ctx, id)
	if err != nil {
		return State{}, err
	}
	if state.Status != StatusAwaitingPlanningApproval {
		return state, fmt.Errorf("Labor %q is not awaiting Planning approval", id)
	}
	planningState, err := service.Planning.Decide(ctx, state.Planning.ID, decision, reason)
	if err != nil {
		return state, err
	}
	state.Planning = planningState
	if decision == DecisionApprove {
		state.PlanningGate = Gate{ID: state.PlanningGate.ID, Kind: state.PlanningGate.Kind, Status: GateApproved, Decision: reason}
		if err := service.record(ctx, &state, StatusIssues, "Planning PRD approved"); err != nil {
			return state, err
		}
	} else if decision == DecisionReject {
		state.PlanningGate = Gate{ID: state.PlanningGate.ID, Kind: state.PlanningGate.Kind, Status: GateRejected, Decision: reason}
		if err := service.record(ctx, &state, StatusPlanning, "Planning PRD rejected for revision"); err != nil {
			return state, err
		}
	} else {
		return state, fmt.Errorf("unknown Planning decision %q", decision)
	}
	return service.progress(ctx, state)
}

// DecideIssues approves or rejects issue publication.
func (service Service) DecideIssues(ctx context.Context, id, decision, reason string) (State, error) {
	state, err := service.Store.Load(ctx, id)
	if err != nil {
		return State{}, err
	}
	if state.Status != StatusAwaitingIssueApproval {
		return state, fmt.Errorf("Labor %q is not awaiting Issue publication approval", id)
	}
	issueState, err := service.Issues.Decide(ctx, state.Issues.ID, decision, reason)
	if err != nil {
		return state, err
	}
	state.Issues = issueState
	if decision == DecisionApprove {
		state.IssueGate = Gate{ID: state.IssueGate.ID, Kind: state.IssueGate.Kind, Status: GateApproved, Decision: reason}
		if err := service.record(ctx, &state, StatusIssues, "Issue publication approved"); err != nil {
			return state, err
		}
	} else if decision == DecisionReject {
		state.IssueGate = Gate{ID: state.IssueGate.ID, Kind: state.IssueGate.Kind, Status: GateRejected, Decision: reason}
		if err := service.record(ctx, &state, StatusIssues, "Issue proposals rejected for revision"); err != nil {
			return state, err
		}
	} else {
		return state, fmt.Errorf("unknown Issue decision %q", decision)
	}
	return service.progress(ctx, state)
}

// Resume continues an interrupted or blocked Labor from its durable boundary.
func (service Service) Resume(ctx context.Context, id string) (State, error) {
	state, err := service.Store.Load(ctx, id)
	if err != nil {
		return State{}, err
	}
	if state.Status == StatusBlocked {
		if err := service.record(ctx, &state, StatusImplementing, "Labor explicitly resumed"); err != nil {
			return state, err
		}
	}
	return service.progress(ctx, state)
}

// Cancel marks a non-terminal Labor cancelled.
func (service Service) Cancel(ctx context.Context, id, reason string) (State, error) {
	state, err := service.Store.Load(ctx, id)
	if err != nil {
		return State{}, err
	}
	if state.Status == StatusCompleted || state.Status == StatusCancelled {
		return state, fmt.Errorf("Labor %q is already terminal", id)
	}
	err = service.record(ctx, &state, StatusCancelled, reason)
	return state, err
}

func (service Service) progress(ctx context.Context, state State) (State, error) {
	for {
		switch state.Status {
		case StatusNew:
			if err := service.record(ctx, &state, StatusPlanning, "Planning Stage started"); err != nil {
				return state, err
			}
		case StatusPlanning:
			value, err := service.Planning.Run(ctx, planning.RunRequest{
				ID: state.ID + "-planning", Problem: state.Problem, Repositories: state.Repositories, Documents: state.Documents,
			})
			state.Planning = value
			if saveErr := service.Store.Save(ctx, state); saveErr != nil {
				return state, errors.Join(err, saveErr)
			}
			if err != nil {
				return state, err
			}
			if value.Status == planning.StatusAwaitingApproval {
				state.PlanningGate.Status = GatePending
				if err := service.record(ctx, &state, StatusAwaitingPlanningApproval, "Planning PRD awaits approval"); err != nil {
					return state, err
				}
				return state, nil
			}
			if value.Status != planning.StatusApproved {
				return state, nil
			}
			if err := service.record(ctx, &state, StatusIssues, "Planning Stage approved"); err != nil {
				return state, err
			}
		case StatusAwaitingPlanningApproval, StatusAwaitingIssueApproval, StatusCompleted, StatusCancelled:
			return state, nil
		case StatusIssues:
			value := state.Issues
			var err error
			switch value.Status {
			case issuestage.StatusApproved, issuestage.StatusPublishing, issuestage.StatusPublished:
			default:
				value, err = service.Issues.Run(ctx, issuestage.RunRequest{
					ID: state.ID + "-issues", ApprovedPRDPath: state.Planning.PRDPath,
					ApprovedPRD: state.Planning.PRD, TrackerRepository: state.TrackerRepository,
				})
			}
			state.Issues = value
			if saveErr := service.Store.Save(ctx, state); saveErr != nil {
				return state, errors.Join(err, saveErr)
			}
			if err != nil {
				return state, err
			}
			if value.Status == issuestage.StatusApproved || value.Status == issuestage.StatusPublishing {
				value, err = service.Issues.Publish(ctx, value.ID)
				state.Issues = value
				if saveErr := service.Store.Save(ctx, state); saveErr != nil {
					return state, errors.Join(err, saveErr)
				}
				if err != nil {
					return state, err
				}
			}
			if value.Status == issuestage.StatusAwaitingApproval {
				state.IssueGate.Status = GatePending
				if err := service.record(ctx, &state, StatusAwaitingIssueApproval, "Issue proposals await publication approval"); err != nil {
					return state, err
				}
				return state, nil
			}
			if value.Status != issuestage.StatusPublished {
				return state, nil
			}
			if err := service.record(ctx, &state, StatusImplementing, "Issue Stage published"); err != nil {
				return state, err
			}
		case StatusImplementing:
			result, err := service.Implementation.Run(ctx)
			state.Backlog = result
			if err != nil {
				if saveErr := service.record(ctx, &state, StatusBlocked, err.Error()); saveErr != nil {
					return state, errors.Join(err, saveErr)
				}
				return state, err
			}
			if !result.Exhausted {
				return state, nil
			}
			if err := service.record(ctx, &state, StatusCompleted, completionMessage(result)); err != nil {
				return state, err
			}
			return state, nil
		case StatusBlocked:
			return state, nil
		default:
			return state, fmt.Errorf("unknown Labor status %q", state.Status)
		}
	}
}

func (service Service) record(ctx context.Context, state *State, status, message string) error {
	state.Status = status
	state.Events = append(state.Events, Event{Status: status, Message: message, CreatedAt: time.Now().UTC()})
	return service.Store.Save(ctx, *state)
}

func completionMessage(result implementation.BacklogResult) string {
	if len(result.PendingHITL) > 0 {
		return fmt.Sprintf("Defined Implementation Stage backlog exhausted; %d HITL issue(s) remain for manual completion", len(result.PendingHITL))
	}
	return "Defined Implementation Stage backlog exhausted"
}
