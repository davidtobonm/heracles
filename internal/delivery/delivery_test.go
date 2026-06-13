package delivery_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/delivery"
	"github.com/davidtobonm/heracles/internal/history"
)

func TestEvidencePolicyRequiresOrderedFailingRedAndPassingGreen(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	valid := []delivery.Evidence{
		{Kind: delivery.RedEvidence, Command: "go test ./...", ExitCode: 1, StartedAt: start, FinishedAt: start.Add(time.Second), ArtifactPath: "artifacts/red.json"},
		{Kind: delivery.GreenEvidence, Command: "go test ./...", ExitCode: 0, StartedAt: start.Add(2 * time.Minute), FinishedAt: start.Add(3 * time.Minute), ArtifactPath: "artifacts/green.json"},
	}
	if err := delivery.ValidateEvidence(delivery.EvidencePolicy{}, valid); err != nil {
		t.Fatalf("ValidateEvidence(valid) error = %v", err)
	}

	for name, evidence := range map[string][]delivery.Evidence{
		"missing green": valid[:1],
		"passing red":   {withExit(valid[0], 0), valid[1]},
		"failing green": {valid[0], withExit(valid[1], 1)},
		"green first":   {valid[1], valid[0]},
	} {
		t.Run(name, func(t *testing.T) {
			if err := delivery.ValidateEvidence(delivery.EvidencePolicy{}, evidence); err == nil {
				t.Fatalf("ValidateEvidence(%s) error = nil, want delivery block", name)
			}
		})
	}

	if err := delivery.ValidateEvidence(delivery.EvidencePolicy{Exempt: true}, nil); err == nil {
		t.Fatal("unreasoned TDD Exemption was accepted")
	}
	if err := delivery.ValidateEvidence(delivery.EvidencePolicy{Exempt: true, Reason: "Documentation-only change; executable red behavior is unsuitable."}, nil); err != nil {
		t.Fatalf("reasoned TDD Exemption error = %v", err)
	}
}

func TestRecorderPersistsAuditableEvidenceArtifact(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := history.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.CreateLabor(ctx, history.NewLabor{ID: "labor-1", Status: "implementing"}); err != nil {
		t.Fatalf("CreateLabor() error = %v", err)
	}
	if _, err := store.CreateIssueAttempt(ctx, history.NewIssueAttempt{ID: "attempt-1", LaborID: "labor-1", IssueURL: "https://github.com/acme/app/issues/1", Attempt: 1, Status: "implementing"}); err != nil {
		t.Fatalf("CreateIssueAttempt() error = %v", err)
	}

	evidence, err := delivery.RecordEvidence(ctx, store, delivery.NewEvidence{
		ID:             "red-1",
		LaborID:        "labor-1",
		IssueAttemptID: "attempt-1",
		Kind:           delivery.RedEvidence,
		Repository:     "app",
		Command:        "go test ./...",
		ExitCode:       1,
		Stdout:         "FAIL\n",
		StartedAt:      time.Now().Add(-time.Second),
		FinishedAt:     time.Now(),
	})
	if err != nil {
		t.Fatalf("RecordEvidence() error = %v", err)
	}
	if evidence.ArtifactPath == "" {
		t.Fatal("evidence artifact path is empty")
	}
	snapshot, err := store.Snapshot(ctx, "labor-1")
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Artifacts) != 1 || snapshot.Artifacts[0].Kind != string(delivery.RedEvidence) {
		t.Errorf("artifacts = %#v, want linked Red Evidence", snapshot.Artifacts)
	}
}

func TestVerifierRunsEveryCommandForTouchedRepositoriesAndReviewerGetsFullContext(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	verifier := delivery.Verifier{Runner: runner}
	results, err := verifier.Run(context.Background(), []delivery.Repository{
		{Name: "backend", Path: "/work/backend", Verify: []string{"go test ./...", "go vet ./..."}},
		{Name: "frontend", Path: "/work/frontend", Verify: []string{"npm test"}},
	}, []string{"backend"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(results) != 2 || len(runner.calls) != 2 || strings.Contains(strings.Join(runner.calls, "\n"), "frontend") {
		t.Errorf("verification calls = %#v, want every backend command only", runner.calls)
	}

	prompt := delivery.ReviewerPrompt(delivery.ReviewContext{
		Issue:        "Implement evidence policy",
		PRD:          "Heracles requires Red and Green Evidence.",
		Changes:      "Added policy and tests.",
		Evidence:     []delivery.Evidence{{Kind: delivery.RedEvidence, Command: "go test", ExitCode: 1}, {Kind: delivery.GreenEvidence, Command: "go test", ExitCode: 0}},
		Verification: results,
		TDDExemption: "",
	})
	for _, expected := range []string{"Implement evidence policy", "Heracles requires", "Red Evidence", "Green Evidence", "correct", "YAGNI", "DRY", "corrective changes"} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("ReviewerPrompt() does not contain %q", expected)
		}
	}

	if err := delivery.ValidateReviewOutcome(delivery.ReviewOutcome{Status: "completed", Summary: "fixed", CorrectiveChanges: true}); err == nil {
		t.Fatal("corrective review without rerun verification was accepted")
	}
	if err := delivery.ValidateReviewOutcome(delivery.ReviewOutcome{Status: "completed", Summary: "fixed", CorrectiveChanges: true, Verification: results}); err != nil {
		t.Fatalf("verified corrective review error = %v", err)
	}
}

func withExit(evidence delivery.Evidence, exitCode int) delivery.Evidence {
	evidence.ExitCode = exitCode
	return evidence
}

type fakeRunner struct {
	calls []string
}

func (runner *fakeRunner) Run(_ context.Context, workingDirectory, command string) (delivery.Execution, error) {
	runner.calls = append(runner.calls, workingDirectory+": "+command)
	return delivery.Execution{ExitCode: 0, Stdout: "ok", StartedAt: time.Now(), FinishedAt: time.Now()}, nil
}
