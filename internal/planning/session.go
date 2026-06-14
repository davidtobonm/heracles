package planning

import (
	"context"
	"errors"
	"fmt"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/prd"
)

// ErrSessionNotFound indicates that no durable interactive Planning session
// exists for an ID.
var ErrSessionNotFound = errors.New("Planning session not found")

// SessionState is the durable state of one interactive Planning session run
// per ADR 0013.
type SessionState struct {
	ID             string              `json:"id"`
	Problem        string              `json:"problem"`
	Status         string              `json:"status"`
	Repositories   []RepositoryContext `json:"repositories,omitempty"`
	Documents      []Document          `json:"documents,omitempty"`
	QuestionBudget int                 `json:"question_budget"`
	PRDIssueURL    string              `json:"prd_issue_url,omitempty"`
	PRDPath        string              `json:"prd_path,omitempty"`
	Gate           Gate                `json:"gate"`
	IssuesStarted  bool                `json:"issues_started,omitempty"`
}

// SessionStore persists durable interactive Planning session state.
type SessionStore interface {
	LoadSession(context.Context, string) (SessionState, error)
	SaveSession(context.Context, SessionState) error
}

// InteractiveRunner launches a Planner attached to the controlling terminal
// for one Grilling Session, per ADR 0013.
type InteractiveRunner interface {
	RunInteractive(ctx context.Context, profile agent.Profile, workspaces []string, brief string) error
}

// IssueGenerator launches background, non-interactive issue generation for
// an approved PRD Issue, per ADR 0015.
type IssueGenerator interface {
	Generate(ctx context.Context, id, prdPath string) error
}

// SessionRequest starts or resumes an interactive Planning session.
type SessionRequest struct {
	ID             string
	Problem        string
	Repositories   []RepositoryContext
	Documents      []Document
	QuestionBudget int
}

// SessionService runs interactive Planning sessions through a provider-owned
// Grilling Session and durable PRD Issue approval, per ADR 0013 and ADR 0014.
type SessionService struct {
	Runner         InteractiveRunner
	Store          SessionStore
	Generator      IssueGenerator
	QuestionBudget int
	Profile        agent.Profile
}

// Run launches or resumes one interactive Grilling Session until it exits,
// then reports the durable session state. The session itself is responsible
// for publishing and revising the PRD Issue and for recording approval
// through `heracles plan` and `heracles approve planning`.
func (service SessionService) Run(ctx context.Context, request SessionRequest) (SessionState, error) {
	if request.ID == "" {
		return SessionState{}, errors.New("Planning session requires an ID")
	}
	if service.Runner == nil || service.Store == nil {
		return SessionState{}, errors.New("Planning session requires a Runner and Store")
	}

	state, err := service.Store.LoadSession(ctx, request.ID)
	if errors.Is(err, ErrSessionNotFound) {
		if request.Problem == "" {
			return SessionState{}, errors.New("new Planning session requires a problem description")
		}
		budget := request.QuestionBudget
		if budget == 0 {
			budget = service.QuestionBudget
		}
		if budget == 0 {
			budget = DefaultQuestionBudget
		}
		state = SessionState{
			ID:             request.ID,
			Problem:        request.Problem,
			Status:         StatusActive,
			Repositories:   append([]RepositoryContext(nil), request.Repositories...),
			Documents:      append([]Document(nil), request.Documents...),
			QuestionBudget: budget,
			Gate:           Gate{Status: GatePending},
		}
		if err := service.Store.SaveSession(ctx, state); err != nil {
			return SessionState{}, err
		}
	} else if err != nil {
		return SessionState{}, err
	}

	if state.Status == StatusApproved || state.Status == StatusRejected {
		return state, nil
	}

	workspaces := make([]string, 0, len(state.Repositories))
	for _, repository := range state.Repositories {
		workspaces = append(workspaces, repository.Path)
	}
	if len(workspaces) == 0 {
		return SessionState{}, errors.New("Planning session requires at least one Target Repository")
	}

	documents := make(map[string]string, len(state.Documents))
	repositories := make([]string, len(state.Repositories))
	for index, repository := range state.Repositories {
		repositories[index] = repository.Name
	}
	for _, document := range state.Documents {
		documents[document.Path] = document.Contents
	}
	brief := prd.SessionBrief(prd.Brief{
		ID:             state.ID,
		Problem:        state.Problem,
		Repositories:   repositories,
		Documents:      documents,
		QuestionBudget: state.QuestionBudget,
	})

	if err := service.Runner.RunInteractive(ctx, service.Profile, workspaces, brief); err != nil {
		return state, fmt.Errorf("Planner: %w", err)
	}

	state, err = service.Store.LoadSession(ctx, request.ID)
	if err != nil {
		return SessionState{}, err
	}
	if err := service.startIssueGeneration(ctx, &state); err != nil {
		return state, err
	}
	return state, nil
}

// RecordPRDIssue records the durable PRD Issue URL and a local PRD mirror
// published or revised during an interactive Grilling Session, per ADR 0014.
func (service SessionService) RecordPRDIssue(ctx context.Context, id, prdIssueURL, prdPath string) (SessionState, error) {
	if prdIssueURL == "" {
		return SessionState{}, errors.New("Planning session requires a PRD Issue URL")
	}
	state, err := service.Store.LoadSession(ctx, id)
	if err != nil {
		return SessionState{}, err
	}
	state.PRDIssueURL = prdIssueURL
	state.PRDPath = prdPath
	if state.Status == StatusActive {
		state.Status = StatusAwaitingApproval
		state.Gate = Gate{Status: GatePending}
	}
	if err := service.Store.SaveSession(ctx, state); err != nil {
		return SessionState{}, err
	}
	return state, nil
}

// Decide approves or rejects a published PRD Issue's Planning Approval Gate,
// from inside or outside the interactive session, per ADR 0014 and ADR 0015.
func (service SessionService) Decide(ctx context.Context, id, decision, reason string) (SessionState, error) {
	state, err := service.Store.LoadSession(ctx, id)
	if err != nil {
		return SessionState{}, err
	}
	if (state.Status == StatusApproved && decision == DecisionApprove) || (state.Status == StatusRejected && decision == DecisionReject) {
		return state, nil
	}
	if state.PRDIssueURL == "" {
		return state, errors.New("Planning session has no published PRD Issue")
	}
	if state.Status != StatusAwaitingApproval {
		return state, fmt.Errorf("Planning session %q is not awaiting approval", id)
	}
	switch decision {
	case DecisionApprove:
		state.Status = StatusApproved
		state.Gate = Gate{Status: GateApproved, Decision: reason}
	case DecisionReject:
		state.Status = StatusRejected
		state.Gate = Gate{Status: GateRejected, Decision: reason}
	default:
		return state, fmt.Errorf("unknown Planning decision %q", decision)
	}
	if err := service.Store.SaveSession(ctx, state); err != nil {
		return SessionState{}, err
	}
	if err := service.startIssueGeneration(ctx, &state); err != nil {
		return state, err
	}
	return state, nil
}

// startIssueGeneration launches the background Issue Author once and only
// once a PRD Issue's Planning Approval Gate is approved, per ADR 0015.
func (service SessionService) startIssueGeneration(ctx context.Context, state *SessionState) error {
	if state.Status != StatusApproved || state.IssuesStarted || state.PRDPath == "" || service.Generator == nil {
		return nil
	}
	if err := service.Generator.Generate(ctx, state.ID, state.PRDPath); err != nil {
		return fmt.Errorf("launch background issue generation: %w", err)
	}
	state.IssuesStarted = true
	return service.Store.SaveSession(ctx, *state)
}
