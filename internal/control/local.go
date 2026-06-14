package control

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/changeset"
	"github.com/davidtobonm/heracles/internal/delivery"
	"github.com/davidtobonm/heracles/internal/doctor"
	"github.com/davidtobonm/heracles/internal/history"
	"github.com/davidtobonm/heracles/internal/implementation"
	"github.com/davidtobonm/heracles/internal/issuestage"
	"github.com/davidtobonm/heracles/internal/labor"
	"github.com/davidtobonm/heracles/internal/planning"
	"github.com/davidtobonm/heracles/internal/project"
	"github.com/davidtobonm/heracles/internal/scheduler"
	"github.com/davidtobonm/heracles/internal/tracker"
	"github.com/davidtobonm/heracles/internal/workspace"
)

// Local is the reusable local application core behind Control Surfaces.
type Local struct {
	root               string
	loaded             project.LoadedConfig
	history            *history.Store
	trackerClient      *tracker.GitHubClient
	tracker            *tracker.Service
	trackerRepo        string
	planning           planning.Service
	issues             issuestage.Service
	workspaces         workspace.Manager
	implementer        implementation.Implementer
	reviewer           implementation.Reviewer
	verifier           implementation.Verifier
	deliverer          implementation.Deliverer
	changeRepositories []changeset.Repository
	scheduler          scheduler.Scheduler
	profile            string
}

// NewLocal wires the configured application services.
func NewLocal(ctx context.Context, loaded project.LoadedConfig) (*Local, error) {
	profiles, err := agent.ResolveProfiles(loaded.Config.Agents)
	if err != nil {
		return nil, err
	}
	root := filepath.Dir(loaded.Path)
	executionHistory, err := history.Open(ctx, root)
	if err != nil {
		return nil, err
	}
	runner := agent.NewRunner(agent.DefaultRegistry(), nil)
	commandRunner := tracker.OSCommandRunner{}
	trackerClient := tracker.NewGitHubClient(commandRunner)

	var repositoryContexts []planning.RepositoryContext
	var workspaceRepositories []workspace.Repository
	var verificationRepositories []delivery.Repository
	var changeRepositories []changeset.Repository
	for _, repository := range loaded.Config.Repositories {
		path, err := loaded.RepositoryPath(repository.Name)
		if err != nil {
			_ = executionHistory.Close()
			return nil, err
		}
		repositoryContexts = append(repositoryContexts, planning.RepositoryContext{Name: repository.Name, Path: path})
		workspaceRepositories = append(workspaceRepositories, workspace.Repository{Name: repository.Name, Path: path, BaseBranch: repository.BaseBranch})
		verificationRepositories = append(verificationRepositories, delivery.Repository{Name: repository.Name, Path: path, Verify: repository.Verify})
		changeRepositories = append(changeRepositories, changeset.Repository{Name: repository.Name, GitHub: repository.GitHub, Base: repository.BaseBranch})
	}
	local := &Local{
		root:          root,
		loaded:        loaded,
		history:       executionHistory,
		trackerClient: trackerClient,
		tracker:       tracker.New(loaded.Config.IssueTracker.GitHub, trackerClient),
		trackerRepo:   loaded.Config.IssueTracker.GitHub,
		planning: planning.Service{
			Planner: planning.AgentPlanner{Runner: runner, Profile: profiles.Roles[agent.RolePlanner]},
			Store:   planning.NewFileStore(root), QuestionBudget: loaded.Config.Planning.QuestionBudget,
		},
		issues: issuestage.Service{
			Author: issuestage.AgentIssueAuthor{Runner: runner, Profile: profiles.Roles[agent.RoleIssueAuthor], Workspaces: contextPaths(repositoryContexts)},
			Store:  issuestage.NewFileStore(root), Publisher: issuestage.NewGitHubPublisher(commandRunner),
		},
		workspaces: workspace.Manager{
			Root: loaded.WorkspaceRoot(), Repositories: workspaceRepositories,
			Policy: workspace.Policy{
				CleanupSuccess: loaded.Config.Workspaces.CleanupSuccess, PreserveFailed: loaded.Config.Workspaces.PreserveFailed, PreserveBlocked: loaded.Config.Workspaces.PreserveBlocked,
			},
		},
		implementer: implementation.AgentImplementer{Runner: runner, Profile: profiles.Roles[agent.RoleImplementer]},
		reviewer:    implementation.AgentReviewer{Runner: runner, Profile: profiles.Roles[agent.RoleReviewer]},
		verifier: implementation.VerificationAdapter{
			Verifier: delivery.Verifier{Runner: delivery.ShellRunner{}}, Repositories: verificationRepositories,
		},
		deliverer: changeset.Service{
			Client: changeset.NewGitHubClient(commandRunner),
			Policy: changeset.Policy{AutoMerge: loaded.Config.Delivery.AutoMerge, MergeOrder: loaded.Config.Delivery.MergeOrder},
		},
		changeRepositories: changeRepositories,
		scheduler: scheduler.Scheduler{
			Concurrency:   loaded.Config.Labor.IssueConcurrency,
			ProfileLimits: map[string]int{profiles.Roles[agent.RoleImplementer].Name: profiles.Roles[agent.RoleImplementer].Concurrency},
		},
		profile: profiles.Roles[agent.RoleImplementer].Name,
	}
	return local, nil
}

// Close closes local durable resources.
func (local *Local) Close() error {
	return local.history.Close()
}

// Execute runs one high-level operation.
func (local *Local) Execute(ctx context.Context, operation Operation) (Result, error) {
	switch operation.Name {
	case "init":
		return result(operation, "initialized", local.loaded), nil
	case "doctor":
		report := doctor.Check(ctx, local.loaded, agent.DefaultRegistry(), doctor.OSSystem{})
		if !report.OK {
			return result(operation, "failed", report), errors.New("project diagnostics failed")
		}
		return result(operation, "ok", report), nil
	case "plan":
		state, err := local.planning.Run(ctx, planning.RunRequest{ID: operation.ID, Problem: operation.Problem, Repositories: local.repositoryContexts()})
		return result(operation, state.Status, state), err
	case "issues":
		state, err := local.issues.Run(ctx, issuestage.RunRequest{ID: operation.ID, ApprovedPRD: operation.PRD, TrackerRepository: local.trackerRepository()})
		return result(operation, state.Status, state), err
	case "run":
		backlog := local.backlog("implementation-direct", "")
		backlog.Limit = operation.Limit
		value, err := backlog.Run(ctx)
		status := "completed"
		if err != nil {
			status = "blocked"
		}
		return result(operation, status, value), err
	case "labor":
		state, err := local.labor(operation.ID).Run(ctx, labor.Request{
			ID: operation.ID, Problem: operation.Problem, TrackerRepository: local.trackerRepository(), Repositories: local.repositoryContexts(),
		})
		return result(operation, state.Status, state), err
	case "approve", "reject":
		return local.decide(ctx, operation)
	case "resume":
		state, err := local.labor(operation.ID).Resume(ctx, operation.ID)
		return result(operation, state.Status, state), err
	case "cancel":
		state, err := local.labor(operation.ID).Cancel(ctx, operation.ID, operation.Reason)
		return result(operation, state.Status, state), err
	case "retry":
		service, err := local.implementationForAttempt(ctx, operation.ID)
		if err != nil {
			return Result{}, err
		}
		state, err := service.Retry(ctx, operation.ID)
		return result(operation, state.Status, state), err
	case "list":
		value, err := local.list(ctx, operation.Kind)
		return result(operation, "ok", value), err
	case "inspect":
		value, err := local.inspect(ctx, operation.Kind, operation.ID)
		return result(operation, "ok", value), err
	default:
		return Result{}, fmt.Errorf("%w %q", ErrUnsupported, operation.Name)
	}
}

func (local *Local) decide(ctx context.Context, operation Operation) (Result, error) {
	if operation.Kind != "planning" && operation.Kind != "issues" {
		return Result{}, fmt.Errorf("approval kind must be planning or issues")
	}
	decision := operation.Decision
	if decision == "" {
		decision = operation.Name
	}
	laborStore := labor.NewFileStore(local.root)
	if _, err := laborStore.Load(ctx, operation.ID); err == nil {
		var state labor.State
		if operation.Kind == "planning" {
			state, err = local.labor(operation.ID).DecidePlanning(ctx, operation.ID, decision, operation.Reason)
		} else {
			state, err = local.labor(operation.ID).DecideIssues(ctx, operation.ID, decision, operation.Reason)
		}
		return result(operation, state.Status, state), err
	}
	if operation.Kind == "planning" {
		state, err := local.planning.Decide(ctx, operation.ID, decision, operation.Reason)
		return result(operation, state.Status, state), err
	}
	state, err := local.issues.Decide(ctx, operation.ID, decision, operation.Reason)
	if err == nil && decision == issuestage.DecisionApprove {
		state, err = local.issues.Publish(ctx, operation.ID)
	}
	return result(operation, state.Status, state), err
}

func (local *Local) labor(id string) labor.Service {
	prd := ""
	if state, err := labor.NewFileStore(local.root).Load(context.Background(), id); err == nil {
		prd = state.Planning.PRD
	}
	return labor.Service{
		Store: labor.NewHistoryStore(local.root, local.history), Planning: local.planning, Issues: local.issues, Implementation: local.backlog(id, prd),
	}
}

func (local *Local) backlog(laborID, prd string) implementation.BacklogRunner {
	return implementation.BacklogRunner{
		Source:    resumableBacklogSource{Source: local.tracker, History: local.history, LaborID: laborID},
		Scheduler: local.scheduler, Executor: &attemptExecutor{local: local, laborID: laborID, prd: prd}, Profile: local.profile,
	}
}

func (local *Local) implementation() implementation.Service {
	return implementation.Service{
		Store: implementation.NewHistoryStore(local.root, local.history), Tracker: local.tracker, Workspaces: local.workspaces,
		Implementer: local.implementer, Reviewer: local.reviewer, Verifier: local.verifier, Deliverer: local.deliverer,
	}
}

func (local *Local) implementationForAttempt(ctx context.Context, attemptID string) (implementation.Service, error) {
	labors, err := local.history.Labors(ctx)
	if err != nil {
		return implementation.Service{}, err
	}
	for _, laborRecord := range labors {
		snapshot, err := local.history.Snapshot(ctx, laborRecord.ID)
		if err != nil {
			return implementation.Service{}, err
		}
		for _, attempt := range snapshot.IssueAttempts {
			if attempt.ID == attemptID {
				return local.implementation(), nil
			}
		}
	}
	return implementation.Service{}, fmt.Errorf("Issue Attempt %q not found", attemptID)
}

type attemptExecutor struct {
	local   *Local
	laborID string
	prd     string
}

type resumableBacklogSource struct {
	Source  implementation.BacklogSource
	History *history.Store
	LaborID string
}

func (source resumableBacklogSource) ReadyIssues(ctx context.Context) ([]tracker.Issue, error) {
	ready, err := source.Source.ReadyIssues(ctx)
	if err != nil {
		return nil, err
	}
	open, err := source.Source.OpenIssues(ctx)
	if err != nil {
		return nil, err
	}
	resumable, err := source.resumableIssues(ctx, open)
	if err != nil {
		return nil, err
	}
	merged := append([]tracker.Issue(nil), resumable...)
	for _, issue := range ready {
		if !containsIssue(merged, issue.URL) {
			merged = append(merged, issue)
		}
	}
	slices.SortFunc(merged, func(left, right tracker.Issue) int {
		if left.CreatedAt.Equal(right.CreatedAt) {
			return left.Number - right.Number
		}
		if left.CreatedAt.Before(right.CreatedAt) {
			return -1
		}
		return 1
	})
	return merged, nil
}

func (source resumableBacklogSource) OpenIssues(ctx context.Context) ([]tracker.Issue, error) {
	return source.Source.OpenIssues(ctx)
}

func (source resumableBacklogSource) resumableIssues(ctx context.Context, open []tracker.Issue) ([]tracker.Issue, error) {
	if source.History == nil || source.LaborID == "" {
		return nil, nil
	}
	snapshot, err := source.History.Snapshot(ctx, source.LaborID)
	if err != nil {
		if strings.Contains(err.Error(), "sql: no rows in result set") {
			for _, issue := range open {
				if slices.Contains(issue.Labels, tracker.LabelInProgress) {
					return nil, fmt.Errorf("issue %s is marked %s but no local Labor state exists", issue.URL, tracker.LabelInProgress)
				}
			}
			return nil, nil
		}
		return nil, err
	}
	attempts := make(map[string]history.IssueAttempt, len(snapshot.IssueAttempts))
	for _, attempt := range snapshot.IssueAttempts {
		attempts[attempt.IssueURL] = attempt
	}
	var resumable []tracker.Issue
	for _, issue := range open {
		if !slices.Contains(issue.Labels, tracker.LabelInProgress) {
			continue
		}
		attempt, ok := attempts[issue.URL]
		if !ok {
			return nil, fmt.Errorf("issue %s is marked %s but has no resumable local attempt", issue.URL, tracker.LabelInProgress)
		}
		if !isResumableAttempt(attempt.Status) {
			return nil, fmt.Errorf("issue %s is marked %s but local attempt %q is %s", issue.URL, tracker.LabelInProgress, attempt.ID, attempt.Status)
		}
		resumable = append(resumable, issue)
	}
	return resumable, nil
}

func isResumableAttempt(status string) bool {
	switch status {
	case implementation.StatusNew, implementation.StatusClaimed, implementation.StatusWorkspaceReady, implementation.StatusImplemented, implementation.StatusReviewed, implementation.StatusVerified, implementation.StatusDelivered:
		return true
	default:
		return false
	}
}

func containsIssue(issues []tracker.Issue, url string) bool {
	for _, issue := range issues {
		if issue.URL == url {
			return true
		}
	}
	return false
}

func (executor *attemptExecutor) Execute(ctx context.Context, candidate scheduler.Candidate) error {
	reference, err := tracker.ParseReference(candidate.Key)
	if err != nil {
		return err
	}
	issue, err := executor.local.trackerClient.Issue(ctx, reference)
	if err != nil {
		return err
	}
	attemptID := fmt.Sprintf("%s-%s-%s-%d", executor.laborID, reference.Owner, reference.Repo, reference.Number)
	_, err = executor.local.implementation().Run(ctx, implementation.Request{
		AttemptID: attemptID, LaborID: executor.laborID, Issue: issue, PRD: executor.prd,
		ChangeSetRepositories: append([]changeset.Repository(nil), executor.local.changeRepositories...),
	})
	return err
}

func (local *Local) list(ctx context.Context, kind string) (any, error) {
	labors, err := local.history.Labors(ctx)
	if err != nil {
		return nil, err
	}
	if kind == "labors" {
		return labors, nil
	}
	var values []any
	for _, laborRecord := range labors {
		snapshot, err := local.history.Snapshot(ctx, laborRecord.ID)
		if err != nil {
			return nil, err
		}
		switch kind {
		case "issues":
			for _, value := range snapshot.IssueAttempts {
				values = append(values, value)
			}
		case "change-sets":
			for _, value := range snapshot.ChangeSets {
				values = append(values, value)
			}
		case "gates":
			for _, value := range snapshot.ApprovalGates {
				values = append(values, value)
			}
		case "logs":
			for _, value := range snapshot.Events {
				values = append(values, value)
			}
		case "evidence":
			for _, value := range snapshot.Artifacts {
				values = append(values, value)
			}
		default:
			return nil, fmt.Errorf("unknown list kind %q", kind)
		}
	}
	return values, nil
}

func (local *Local) inspect(ctx context.Context, kind, id string) (any, error) {
	if kind == "labor" {
		return local.history.Snapshot(ctx, id)
	}
	values, err := local.list(ctx, pluralKind(kind))
	if err != nil {
		return nil, err
	}
	for _, value := range values.([]any) {
		if strings.Contains(fmt.Sprint(value), id) {
			return value, nil
		}
	}
	return nil, fmt.Errorf("%s %q not found", kind, id)
}

func pluralKind(kind string) string {
	switch kind {
	case "issue":
		return "issues"
	case "change-set":
		return "change-sets"
	case "gate":
		return "gates"
	case "log":
		return "logs"
	case "evidence":
		return "evidence"
	default:
		return kind
	}
}

func result(operation Operation, status string, data any) Result {
	return Result{Operation: operation.Name, Kind: operation.Kind, ID: operation.ID, Status: status, Data: data}
}

func (local *Local) trackerRepository() string {
	return local.trackerRepo
}

func (local *Local) repositoryContexts() []planning.RepositoryContext {
	contexts := make([]planning.RepositoryContext, len(local.workspaces.Repositories))
	for index, repository := range local.workspaces.Repositories {
		contexts[index] = planning.RepositoryContext{Name: repository.Name, Path: repository.Path}
	}
	return contexts
}

func contextPaths(contexts []planning.RepositoryContext) []string {
	paths := make([]string, len(contexts))
	for index, context := range contexts {
		paths[index] = context.Path
	}
	return paths
}
