// Package implementation composes one Ready Issue through delivery.
package implementation

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/davidtobonm/heracles/internal/changeset"
	"github.com/davidtobonm/heracles/internal/delivery"
	"github.com/davidtobonm/heracles/internal/tracker"
	"github.com/davidtobonm/heracles/internal/workspace"
)

const (
	StatusNew            = "new"
	StatusClaimed        = "claimed"
	StatusWorkspaceReady = "workspace_ready"
	StatusImplemented    = "implemented"
	StatusReviewed       = "reviewed"
	StatusVerified       = "verified"
	StatusDelivered      = "delivered"
	StatusCompleted      = "completed"
	StatusFailed         = "failed"
	StatusBlocked        = "blocked"
)

// ErrNotFound indicates that no durable attempt exists for an ID.
var ErrNotFound = errors.New("Implementation Stage attempt not found")

// Event records one durable attempt transition.
type Event struct {
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ImplementationResult is the Implementer's auditable output.
type ImplementationResult struct {
	Changes        string                  `json:"changes"`
	Evidence       []delivery.Evidence     `json:"evidence,omitempty"`
	EvidencePolicy delivery.EvidencePolicy `json:"evidence_policy"`
}

// State is the complete durable issue-attempt state.
type State struct {
	AttemptID             string                  `json:"attempt_id"`
	LaborID               string                  `json:"labor_id"`
	Issue                 tracker.Issue           `json:"issue"`
	PRD                   string                  `json:"prd,omitempty"`
	Status                string                  `json:"status"`
	Workspace             workspace.Workspace     `json:"workspace"`
	Implementation        ImplementationResult    `json:"implementation"`
	Review                delivery.ReviewOutcome  `json:"review"`
	Touched               []string                `json:"touched,omitempty"`
	Verification          []delivery.Verification `json:"verification,omitempty"`
	ChangeSetRepositories []changeset.Repository  `json:"change_set_repositories,omitempty"`
	ChangeSet             changeset.ChangeSet     `json:"change_set"`
	Events                []Event                 `json:"events,omitempty"`
}

// Request starts or resumes one issue attempt.
type Request struct {
	AttemptID             string
	LaborID               string
	Issue                 tracker.Issue
	PRD                   string
	ChangeSetRepositories []changeset.Repository
}

// ImplementContext is the complete isolated context presented to the Implementer.
type ImplementContext struct {
	Issue     tracker.Issue
	PRD       string
	Workspace workspace.Workspace
}

// Store persists complete attempt state.
type Store interface {
	Load(context.Context, string) (State, error)
	Save(context.Context, State) error
}

// Tracker coordinates shared GitHub state.
type Tracker interface {
	Claim(context.Context, tracker.Reference, string) (tracker.Issue, error)
	Block(context.Context, tracker.Reference, string) error
	Complete(context.Context, tracker.Reference, string) error
	Retry(context.Context, tracker.Reference, string) error
}

// Workspaces coordinates isolated issue worktrees.
type Workspaces interface {
	Create(context.Context, workspace.Request) (workspace.Workspace, error)
	Touched(context.Context, workspace.Workspace) ([]string, error)
	Finalize(context.Context, workspace.Workspace, workspace.Outcome) error
}

// Implementer makes and reports issue changes.
type Implementer interface {
	Implement(context.Context, ImplementContext) (ImplementationResult, error)
}

// Reviewer reviews the complete delivery contract.
type Reviewer interface {
	Review(context.Context, workspace.Workspace, delivery.ReviewContext) (delivery.ReviewOutcome, error)
}

// Verifier executes local gates for touched repositories.
type Verifier interface {
	Verify(context.Context, workspace.Workspace, []string) ([]delivery.Verification, error)
}

// Deliverer prepares one linked Change Set.
type Deliverer interface {
	Deliver(context.Context, changeset.Request) (changeset.ChangeSet, error)
}

// Service runs and resumes one independently executable Implementation Stage attempt.
type Service struct {
	Store       Store
	Tracker     Tracker
	Workspaces  Workspaces
	Implementer Implementer
	Reviewer    Reviewer
	Verifier    Verifier
	Deliverer   Deliverer
}

// Run starts or resumes an attempt until completion or a durable failure.
func (service Service) Run(ctx context.Context, request Request) (State, error) {
	if err := service.validate(); err != nil {
		return State{}, err
	}
	state, err := service.Store.Load(ctx, request.AttemptID)
	if errors.Is(err, ErrNotFound) {
		if request.AttemptID == "" || request.LaborID == "" || request.Issue.URL == "" {
			return State{}, errors.New("new Implementation Stage attempt requires attempt ID, labor ID, and issue")
		}
		state = State{
			AttemptID: request.AttemptID, LaborID: request.LaborID, Issue: request.Issue, PRD: request.PRD,
			Status: StatusNew, ChangeSetRepositories: append([]changeset.Repository(nil), request.ChangeSetRepositories...),
		}
		if err := service.record(ctx, &state, StatusNew, "Attempt created"); err != nil {
			return state, err
		}
	} else if err != nil {
		return State{}, err
	}

	for {
		if err := ctx.Err(); err != nil {
			return state, err
		}
		switch state.Status {
		case StatusNew:
			if _, err := service.Tracker.Claim(ctx, state.Issue.Reference, state.LaborID); err != nil {
				return service.fail(ctx, state, StatusFailed, workspace.OutcomeFailed, fmt.Errorf("claim issue: %w", err))
			}
			if err := service.record(ctx, &state, StatusClaimed, "Issue claimed"); err != nil {
				return state, err
			}
		case StatusClaimed:
			value, err := service.Workspaces.Create(ctx, workspace.Request{
				IssueRepository: state.Issue.Reference.Repository(), IssueNumber: state.Issue.Number, Title: state.Issue.Title,
			})
			if err != nil {
				return service.fail(ctx, state, StatusFailed, workspace.OutcomeFailed, fmt.Errorf("create Issue Workspace: %w", err))
			}
			state.Workspace = value
			if err := service.record(ctx, &state, StatusWorkspaceReady, "Issue Workspace ready"); err != nil {
				return state, err
			}
		case StatusWorkspaceReady:
			result, err := service.Implementer.Implement(ctx, ImplementContext{Issue: state.Issue, PRD: state.PRD, Workspace: state.Workspace})
			if err != nil {
				return service.fail(ctx, state, StatusFailed, workspace.OutcomeFailed, fmt.Errorf("Implementer: %w", err))
			}
			state.Implementation = result
			if err := delivery.ValidateEvidence(result.EvidencePolicy, result.Evidence); err != nil {
				return service.fail(ctx, state, StatusBlocked, workspace.OutcomeBlocked, fmt.Errorf("evidence gate: %w", err))
			}
			if err := service.record(ctx, &state, StatusImplemented, "Implementation and evidence complete"); err != nil {
				return state, err
			}
		case StatusImplemented:
			outcome, err := service.Reviewer.Review(ctx, state.Workspace, delivery.ReviewContext{
				Issue: state.Issue.Body, PRD: state.PRD, Changes: state.Implementation.Changes,
				Evidence: state.Implementation.Evidence, TDDExemption: state.Implementation.EvidencePolicy.Reason,
			})
			if err != nil {
				return service.fail(ctx, state, StatusFailed, workspace.OutcomeFailed, fmt.Errorf("Reviewer: %w", err))
			}
			if err := delivery.ValidateReviewOutcome(outcome); err != nil {
				return service.fail(ctx, state, StatusBlocked, workspace.OutcomeBlocked, fmt.Errorf("review gate: %w", err))
			}
			state.Review = outcome
			if outcome.Status == StatusBlocked {
				return service.fail(ctx, state, StatusBlocked, workspace.OutcomeBlocked, errors.New(outcome.Summary))
			}
			if err := service.record(ctx, &state, StatusReviewed, outcome.Summary); err != nil {
				return state, err
			}
		case StatusReviewed:
			touched, err := service.Workspaces.Touched(ctx, state.Workspace)
			if err != nil {
				return service.fail(ctx, state, StatusFailed, workspace.OutcomeFailed, fmt.Errorf("detect touched repositories: %w", err))
			}
			results, err := service.Verifier.Verify(ctx, state.Workspace, touched)
			state.Touched = touched
			state.Verification = results
			if err != nil {
				return service.fail(ctx, state, StatusBlocked, workspace.OutcomeBlocked, fmt.Errorf("local verification: %w", err))
			}
			if err := service.record(ctx, &state, StatusVerified, "Local verification passed"); err != nil {
				return state, err
			}
		case StatusVerified:
			changeSet, err := service.Deliverer.Deliver(ctx, service.changeSetRequest(state))
			state.ChangeSet = changeSet
			if err != nil || changeSet.Status == changeset.StatusBlocked {
				if err == nil {
					err = errors.New("Change Set delivery blocked")
				}
				return service.fail(ctx, state, StatusBlocked, workspace.OutcomeBlocked, fmt.Errorf("deliver Change Set: %w", err))
			}
			if err := service.record(ctx, &state, StatusDelivered, "Change Set delivered"); err != nil {
				return state, err
			}
		case StatusDelivered:
			if err := service.Tracker.Complete(ctx, state.Issue.Reference, "Delivered Change Set "+state.ChangeSet.ID); err != nil {
				return state, fmt.Errorf("mark issue complete: %w", err)
			}
			if err := service.Workspaces.Finalize(ctx, state.Workspace, workspace.OutcomeSuccess); err != nil {
				return state, fmt.Errorf("finalize successful Issue Workspace: %w", err)
			}
			if err := service.record(ctx, &state, StatusCompleted, "Issue completed"); err != nil {
				return state, err
			}
		case StatusCompleted, StatusFailed, StatusBlocked:
			return state, nil
		default:
			return state, fmt.Errorf("unknown Implementation Stage status %q", state.Status)
		}
	}
}

// Retry resumes a preserved failed or blocked attempt.
func (service Service) Retry(ctx context.Context, attemptID string) (State, error) {
	state, err := service.Store.Load(ctx, attemptID)
	if err != nil {
		return State{}, err
	}
	if state.Status != StatusFailed && state.Status != StatusBlocked {
		return state, fmt.Errorf("attempt %q is not failed or blocked", attemptID)
	}
	if err := service.Tracker.Retry(ctx, state.Issue.Reference, state.LaborID); err != nil {
		return state, fmt.Errorf("publish retry state: %w", err)
	}
	next := StatusClaimed
	if state.Workspace.Root != "" {
		next = StatusWorkspaceReady
	}
	if err := service.record(ctx, &state, next, "Attempt explicitly retried"); err != nil {
		return state, err
	}
	return service.Run(ctx, Request{AttemptID: attemptID})
}

func (service Service) validate() error {
	if service.Store == nil || service.Tracker == nil || service.Workspaces == nil || service.Implementer == nil || service.Reviewer == nil || service.Verifier == nil || service.Deliverer == nil {
		return errors.New("Implementation Stage requires Store, Tracker, Workspaces, Implementer, Reviewer, Verifier, and Deliverer")
	}
	return nil
}

func (service Service) record(ctx context.Context, state *State, status, message string) error {
	state.Status = status
	state.Events = append(state.Events, Event{Status: status, Message: message, CreatedAt: time.Now().UTC()})
	return service.Store.Save(ctx, *state)
}

func (service Service) fail(ctx context.Context, state State, status string, outcome workspace.Outcome, cause error) (State, error) {
	if err := service.record(ctx, &state, status, cause.Error()); err != nil {
		return state, errors.Join(cause, err)
	}
	var failures []error
	failures = append(failures, cause)
	if err := service.Tracker.Block(ctx, state.Issue.Reference, cause.Error()); err != nil {
		failures = append(failures, fmt.Errorf("publish blocked state: %w", err))
	}
	if state.Workspace.Root != "" {
		if err := service.Workspaces.Finalize(ctx, state.Workspace, outcome); err != nil {
			failures = append(failures, fmt.Errorf("finalize Issue Workspace: %w", err))
		}
	}
	return state, errors.Join(failures...)
}

func (service Service) changeSetRequest(state State) changeset.Request {
	touched := make(map[string]bool, len(state.Touched))
	for _, name := range state.Touched {
		touched[name] = true
	}
	verified := make(map[string]bool, len(state.Touched))
	for _, name := range state.Touched {
		verified[name] = true
	}
	qa := make(map[string][]string)
	for _, result := range state.Verification {
		qa[result.Repository] = append(qa[result.Repository], result.Command)
		if result.ExitCode != 0 {
			verified[result.Repository] = false
		}
	}
	var evidence []string
	for _, value := range state.Implementation.Evidence {
		if value.ArtifactPath != "" {
			evidence = append(evidence, value.ArtifactPath)
		}
	}
	repositories := append([]changeset.Repository(nil), state.ChangeSetRepositories...)
	for index := range repositories {
		repositories[index].Touched = touched[repositories[index].Name]
		repositories[index].Verified = verified[repositories[index].Name]
		repositories[index].ReviewSummary = state.Review.Summary
		repositories[index].QASteps = qa[repositories[index].Name]
		repositories[index].Evidence = append([]string(nil), evidence...)
	}
	slices.SortFunc(repositories, func(left, right changeset.Repository) int {
		return strings.Compare(left.Name, right.Name)
	})
	return changeset.Request{ID: state.AttemptID + "-change-set", IssueURL: state.Issue.URL, Repositories: repositories}
}

// VerificationAdapter exposes a delivery.Verifier to the Implementation Stage.
type VerificationAdapter struct {
	Verifier     delivery.Verifier
	Repositories []delivery.Repository
}

// Verify runs configured local gates in Issue Workspace paths.
func (adapter VerificationAdapter) Verify(ctx context.Context, issueWorkspace workspace.Workspace, touched []string) ([]delivery.Verification, error) {
	repositories := append([]delivery.Repository(nil), adapter.Repositories...)
	for index := range repositories {
		if worktree := issueWorkspace.Repository(repositories[index].Name); worktree.Path != "" {
			repositories[index].Path = worktree.Path
		}
	}
	return adapter.Verifier.Run(ctx, repositories, touched)
}

// ReviewFunc adapts a function to Reviewer.
type ReviewFunc func(context.Context, workspace.Workspace, delivery.ReviewContext) (delivery.ReviewOutcome, error)

// Review invokes the adapted function.
func (function ReviewFunc) Review(ctx context.Context, issueWorkspace workspace.Workspace, reviewContext delivery.ReviewContext) (delivery.ReviewOutcome, error) {
	return function(ctx, issueWorkspace, reviewContext)
}

// ImplementFunc adapts a function to Implementer.
type ImplementFunc func(context.Context, ImplementContext) (ImplementationResult, error)

// Implement invokes the adapted function.
func (function ImplementFunc) Implement(ctx context.Context, implementContext ImplementContext) (ImplementationResult, error) {
	return function(ctx, implementContext)
}

func issueSummary(issue tracker.Issue) string {
	return strings.TrimSpace(issue.Title + "\n\n" + issue.Body)
}
