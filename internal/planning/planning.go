// Package planning runs the durable Planning Stage.
package planning

import (
	"context"
	"errors"
	"fmt"
)

const (
	DefaultQuestionBudget = 20

	StatusActive             = "active"
	StatusAwaitingAnswers    = "awaiting_answers"
	StatusPermissionRequired = "permission_required"
	StatusAwaitingApproval   = "awaiting_approval"
	StatusApproved           = "approved"
	StatusRejected           = "rejected"

	GatePending  = "pending"
	GateApproved = "approved"
	GateRejected = "rejected"

	DecisionApprove = "approve"
	DecisionReject  = "reject"
)

// ErrNotFound indicates that no durable Planning Stage exists for an ID.
var ErrNotFound = errors.New("Planning Stage not found")

// RepositoryContext identifies one Target Repository available to the Planner.
type RepositoryContext struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Document is relevant existing documentation available to the Planner.
type Document struct {
	Path     string `json:"path"`
	Contents string `json:"contents"`
}

// DocumentationUpdate is a lazy documentation request needed by later work.
type DocumentationUpdate struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	Needed bool   `json:"needed"`
}

// Gate is the durable Planning approval decision.
type Gate struct {
	Status   string `json:"status"`
	Decision string `json:"decision,omitempty"`
}

// State is the complete durable Planning Stage state.
type State struct {
	ID                   string                `json:"id"`
	Problem              string                `json:"problem"`
	Status               string                `json:"status"`
	Repositories         []RepositoryContext   `json:"repositories,omitempty"`
	Documents            []Document            `json:"documents,omitempty"`
	Questions            []string              `json:"questions,omitempty"`
	PendingQuestions     []string              `json:"pending_questions,omitempty"`
	Answers              []string              `json:"answers,omitempty"`
	QuestionsAsked       int                   `json:"questions_asked"`
	QuestionBudget       int                   `json:"question_budget"`
	PermissionBeyond     bool                  `json:"permission_beyond_budget"`
	DocumentationUpdates []DocumentationUpdate `json:"documentation_updates,omitempty"`
	PRDPath              string                `json:"prd_path,omitempty"`
	PRD                  string                `json:"prd,omitempty"`
	Gate                 Gate                  `json:"gate"`
}

// RunRequest starts or resumes a Planning Stage.
type RunRequest struct {
	ID                 string
	Problem            string
	Repositories       []RepositoryContext
	Documents          []Document
	Answers            []string
	PermitBeyondBudget bool
}

// PlannerRequest is the complete bounded context sent to the Planner.
type PlannerRequest struct {
	Problem          string
	Repositories     []RepositoryContext
	Documents        []Document
	Questions        []string
	Answers          []string
	RevisionFeedback string
	QuestionsAsked   int
	QuestionBudget   int
	PermissionBeyond bool
}

// Response is one structured Planner result.
type Response struct {
	Questions            []string
	PRD                  string
	DocumentationUpdates []DocumentationUpdate
}

// Planner performs one bounded Planning Stage turn.
type Planner interface {
	Plan(context.Context, PlannerRequest) (Response, error)
}

// Store persists Planning Stage state and PRD artifacts.
type Store interface {
	Load(context.Context, string) (State, error)
	Save(context.Context, State) error
	WritePRD(context.Context, string, string) (string, error)
}

// Service runs and decides Planning Stages.
type Service struct {
	Planner        Planner
	Store          Store
	QuestionBudget int
}

// Run starts or resumes a Planning Stage until its next durable pause.
func (service Service) Run(ctx context.Context, request RunRequest) (State, error) {
	if request.ID == "" {
		return State{}, errors.New("Planning Stage requires an ID")
	}
	if service.Planner == nil || service.Store == nil {
		return State{}, errors.New("Planning Stage requires a Planner and Store")
	}

	state, err := service.Store.Load(ctx, request.ID)
	if errors.Is(err, ErrNotFound) {
		budget := service.QuestionBudget
		if budget == 0 {
			budget = DefaultQuestionBudget
		}
		if request.Problem == "" {
			return State{}, errors.New("new Planning Stage requires a problem description")
		}
		state = State{
			ID:             request.ID,
			Problem:        request.Problem,
			Status:         StatusActive,
			Repositories:   append([]RepositoryContext(nil), request.Repositories...),
			Documents:      append([]Document(nil), request.Documents...),
			QuestionBudget: budget,
		}
		if err := service.Store.Save(ctx, state); err != nil {
			return State{}, err
		}
	} else if err != nil {
		return State{}, err
	}

	switch state.Status {
	case StatusApproved, StatusAwaitingApproval:
		return state, nil
	case StatusPermissionRequired:
		if !request.PermitBeyondBudget {
			return state, nil
		}
		state.PermissionBeyond = true
		state.Questions = append(state.Questions, state.PendingQuestions...)
		state.QuestionsAsked += len(state.PendingQuestions)
		state.PendingQuestions = nil
		state.Status = StatusAwaitingAnswers
		return state, service.Store.Save(ctx, state)
	case StatusAwaitingAnswers:
		if len(request.Answers) == 0 {
			return state, nil
		}
		state.Answers = append(state.Answers, request.Answers...)
		state.Status = StatusActive
	case StatusRejected:
		state.Status = StatusActive
	case StatusActive:
	default:
		return State{}, fmt.Errorf("unknown Planning Stage status %q", state.Status)
	}

	response, err := service.Planner.Plan(ctx, PlannerRequest{
		Problem:          state.Problem,
		Repositories:     append([]RepositoryContext(nil), state.Repositories...),
		Documents:        append([]Document(nil), state.Documents...),
		Questions:        append([]string(nil), state.Questions...),
		Answers:          append([]string(nil), state.Answers...),
		RevisionFeedback: state.Gate.Decision,
		QuestionsAsked:   state.QuestionsAsked,
		QuestionBudget:   state.QuestionBudget,
		PermissionBeyond: state.PermissionBeyond,
	})
	if err != nil {
		return state, fmt.Errorf("Planner: %w", err)
	}
	if response.PRD != "" && len(response.Questions) > 0 {
		return state, errors.New("Planner response cannot contain both questions and a PRD")
	}
	state.DocumentationUpdates = neededUpdates(response.DocumentationUpdates)
	if len(response.Questions) > 0 {
		if state.QuestionsAsked+len(response.Questions) > state.QuestionBudget && !state.PermissionBeyond {
			state.PendingQuestions = append([]string(nil), response.Questions...)
			state.Status = StatusPermissionRequired
		} else {
			state.Questions = append(state.Questions, response.Questions...)
			state.QuestionsAsked += len(response.Questions)
			state.Status = StatusAwaitingAnswers
		}
		return state, service.Store.Save(ctx, state)
	}
	if response.PRD == "" {
		return state, errors.New("Planner response requires questions or a PRD")
	}
	state.PRDPath, err = service.Store.WritePRD(ctx, state.ID, response.PRD)
	if err != nil {
		return state, err
	}
	state.PRD = response.PRD
	state.Status = StatusAwaitingApproval
	state.Gate = Gate{Status: GatePending}
	return state, service.Store.Save(ctx, state)
}

// Decide approves or rejects a persisted Planning PRD.
func (service Service) Decide(ctx context.Context, id, decision, reason string) (State, error) {
	state, err := service.Store.Load(ctx, id)
	if err != nil {
		return State{}, err
	}
	if (state.Status == StatusApproved && decision == DecisionApprove) || (state.Status == StatusRejected && decision == DecisionReject) {
		return state, nil
	}
	if state.Status != StatusAwaitingApproval {
		return state, fmt.Errorf("Planning Stage %q is not awaiting approval", id)
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
	return state, service.Store.Save(ctx, state)
}

func neededUpdates(updates []DocumentationUpdate) []DocumentationUpdate {
	var needed []DocumentationUpdate
	for _, update := range updates {
		if update.Needed {
			needed = append(needed, update)
		}
	}
	return needed
}
