package planning_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/planning"
)

func TestPlanningFinishesEarlyWithCompleteContextAndApprovalGate(t *testing.T) {
	t.Parallel()

	store := planning.NewMemoryStore()
	planner := &scriptedPlanner{responses: []planning.Response{{
		PRD: "# PRD\n\nBuild the useful thing.",
		DocumentationUpdates: []planning.DocumentationUpdate{
			{Path: "CONTEXT.md", Reason: "Clarifies domain language", Needed: true},
			{Path: "README.md", Reason: "Unrelated cleanup", Needed: false},
		},
	}}}
	service := planning.Service{Planner: planner, Store: store}
	state, err := service.Run(context.Background(), planning.RunRequest{
		ID:      "plan-1",
		Problem: "Make delivery reliable",
		Repositories: []planning.RepositoryContext{
			{Name: "backend", Path: "/work/backend"},
			{Name: "frontend", Path: "/work/frontend"},
		},
		Documents: []planning.Document{
			{Path: "backend/CONTEXT.md", Contents: "Backend vocabulary"},
			{Path: "frontend/README.md", Contents: "Frontend notes"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if state.Status != planning.StatusAwaitingApproval || state.Gate.Status != planning.GatePending || state.PRDPath == "" {
		t.Fatalf("state = %#v, want persisted PRD and pending approval", state)
	}
	if len(state.DocumentationUpdates) != 1 || state.DocumentationUpdates[0].Path != "CONTEXT.md" {
		t.Errorf("documentation updates = %#v, want only needed update", state.DocumentationUpdates)
	}
	request := planner.requests[0]
	if len(request.Repositories) != 2 || len(request.Documents) != 2 || request.QuestionBudget != planning.DefaultQuestionBudget {
		t.Errorf("planner request = %#v, want all target context and default budget", request)
	}
	if contents := store.Artifact(state.PRDPath); !strings.Contains(contents, "Build the useful thing") {
		t.Errorf("persisted PRD = %q", contents)
	}
}

func TestPlanningQuestionBudgetRequiresDurablePermission(t *testing.T) {
	t.Parallel()

	store := planning.NewMemoryStore()
	planner := &scriptedPlanner{responses: []planning.Response{{
		Questions: []string{"q1", "q2", "q3"},
	}}}
	service := planning.Service{Planner: planner, Store: store, QuestionBudget: 2}

	state, err := service.Run(context.Background(), planning.RunRequest{ID: "plan-1", Problem: "Clarify"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if state.Status != planning.StatusPermissionRequired || state.QuestionsAsked != 0 || len(state.PendingQuestions) != 3 {
		t.Fatalf("state = %#v, want pending questions and permission requirement", state)
	}

	state, err = service.Run(context.Background(), planning.RunRequest{ID: "plan-1", PermitBeyondBudget: true})
	if err != nil {
		t.Fatalf("Run(permission) error = %v", err)
	}
	if state.Status != planning.StatusAwaitingAnswers || state.QuestionsAsked != 3 || len(planner.requests) != 1 {
		t.Errorf("state = %#v, planner calls = %d; want permission without replaying agent", state, len(planner.requests))
	}
}

func TestPlanningApprovalRejectionAndResumeDoNotRepeatCommittedWork(t *testing.T) {
	t.Parallel()

	store := planning.NewMemoryStore()
	firstPlanner := &scriptedPlanner{responses: []planning.Response{{PRD: "first"}}}
	service := planning.Service{Planner: firstPlanner, Store: store}
	_, err := service.Run(context.Background(), planning.RunRequest{ID: "plan-1", Problem: "Plan"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	resumePlanner := &scriptedPlanner{}
	resumed, err := (planning.Service{Planner: resumePlanner, Store: store}).Run(context.Background(), planning.RunRequest{ID: "plan-1"})
	if err != nil {
		t.Fatalf("Run(resume) error = %v", err)
	}
	if resumed.Status != planning.StatusAwaitingApproval || len(resumePlanner.requests) != 0 {
		t.Errorf("resume = %#v, calls = %d; want durable pause without replay", resumed, len(resumePlanner.requests))
	}

	rejected, err := service.Decide(context.Background(), "plan-1", planning.DecisionReject, "Cover rollback behavior")
	if err != nil {
		t.Fatalf("Decide(reject) error = %v", err)
	}
	if rejected.Status != planning.StatusRejected || rejected.Gate.Decision != "Cover rollback behavior" {
		t.Errorf("rejected state = %#v", rejected)
	}

	revisionPlanner := &scriptedPlanner{responses: []planning.Response{{PRD: "revised"}}}
	revised, err := (planning.Service{Planner: revisionPlanner, Store: store}).Run(context.Background(), planning.RunRequest{ID: "plan-1"})
	if err != nil {
		t.Fatalf("Run(revision) error = %v", err)
	}
	if revised.Status != planning.StatusAwaitingApproval || revised.Gate.Status != planning.GatePending {
		t.Errorf("revised state = %#v, want new approval pause", revised)
	}

	approved, err := service.Decide(context.Background(), "plan-1", planning.DecisionApprove, "Looks good")
	if err != nil {
		t.Fatalf("Decide(approve) error = %v", err)
	}
	if approved.Status != planning.StatusApproved || approved.Gate.Status != planning.GateApproved {
		t.Errorf("approved state = %#v", approved)
	}
}

func TestFileStoreDurablyResumesStateAndPRDArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := planning.NewFileStore(root)
	service := planning.Service{Planner: &scriptedPlanner{responses: []planning.Response{{PRD: "durable PRD"}}}, Store: store}
	state, err := service.Run(context.Background(), planning.RunRequest{ID: "plan-1", Problem: "Persist"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	reopened := planning.NewFileStore(root)
	resumed, err := (planning.Service{Planner: &scriptedPlanner{}, Store: reopened}).Run(context.Background(), planning.RunRequest{ID: "plan-1"})
	if err != nil {
		t.Fatalf("Run(reopened) error = %v", err)
	}
	if resumed.Status != planning.StatusAwaitingApproval || resumed.PRDPath != state.PRDPath {
		t.Errorf("resumed state = %#v, want durable approval pause", resumed)
	}
	if !strings.HasSuffix(state.PRDPath, filepath.Join("plan-1", "PRD.md")) {
		t.Errorf("PRD path = %q, want stable artifact path", state.PRDPath)
	}
}

type scriptedPlanner struct {
	responses []planning.Response
	requests  []planning.PlannerRequest
}

func (planner *scriptedPlanner) Plan(_ context.Context, request planning.PlannerRequest) (planning.Response, error) {
	planner.requests = append(planner.requests, request)
	if len(planner.responses) == 0 {
		return planning.Response{}, errors.New("unexpected planner call")
	}
	response := planner.responses[0]
	planner.responses = planner.responses[1:]
	return response, nil
}
