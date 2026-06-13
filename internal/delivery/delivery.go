// Package delivery enforces verification, evidence, and review contracts.
package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/davidtobonm/heracles/internal/history"
)

// EvidenceKind identifies auditable TDD evidence.
type EvidenceKind string

const (
	RedEvidence   EvidenceKind = "red"
	GreenEvidence EvidenceKind = "green"
)

// NewEvidence describes evidence before its artifact is persisted.
type NewEvidence struct {
	ID             string
	LaborID        string
	IssueAttemptID string
	Kind           EvidenceKind
	Repository     string
	Command        string
	ExitCode       int
	Stdout         string
	Stderr         string
	StartedAt      time.Time
	FinishedAt     time.Time
}

// Evidence is an auditable command result linked to a durable artifact.
type Evidence struct {
	Kind         EvidenceKind `json:"kind"`
	Repository   string       `json:"repository"`
	Command      string       `json:"command"`
	ExitCode     int          `json:"exit_code"`
	Stdout       string       `json:"stdout"`
	Stderr       string       `json:"stderr"`
	StartedAt    time.Time    `json:"started_at"`
	FinishedAt   time.Time    `json:"finished_at"`
	ArtifactPath string       `json:"artifact_path"`
}

// EvidencePolicy declares a reasoned TDD Exemption when evidence is unsuitable.
type EvidencePolicy struct {
	Exempt bool
	Reason string
}

// ValidateEvidence blocks delivery without valid ordered Red and Green Evidence.
func ValidateEvidence(policy EvidencePolicy, evidence []Evidence) error {
	if policy.Exempt {
		if strings.TrimSpace(policy.Reason) == "" {
			return errors.New("TDD Exemption requires a reason")
		}
		return nil
	}

	var red *Evidence
	var green *Evidence
	redIndex := -1
	greenIndex := -1
	for index := range evidence {
		switch evidence[index].Kind {
		case RedEvidence:
			if red == nil {
				red = &evidence[index]
				redIndex = index
			}
		case GreenEvidence:
			if green == nil {
				green = &evidence[index]
				greenIndex = index
			}
		}
	}
	if red == nil || green == nil {
		return errors.New("delivery requires Red Evidence and Green Evidence")
	}
	if err := validateEvidenceRecord(*red); err != nil {
		return fmt.Errorf("invalid Red Evidence: %w", err)
	}
	if err := validateEvidenceRecord(*green); err != nil {
		return fmt.Errorf("invalid Green Evidence: %w", err)
	}
	if red.ExitCode == 0 {
		return errors.New("Red Evidence must record a failing result")
	}
	if green.ExitCode != 0 {
		return errors.New("Green Evidence must record a passing result")
	}
	if red.ArtifactPath == "" || green.ArtifactPath == "" {
		return errors.New("Red Evidence and Green Evidence require artifact references")
	}
	if redIndex >= greenIndex || !red.FinishedAt.Before(green.StartedAt) {
		return errors.New("Red Evidence must be recorded before Green Evidence")
	}
	return nil
}

// RecordEvidence persists a human-readable evidence artifact and returns its metadata.
func RecordEvidence(ctx context.Context, store *history.Store, input NewEvidence) (Evidence, error) {
	evidence := Evidence{
		Kind:       input.Kind,
		Repository: input.Repository,
		Command:    input.Command,
		ExitCode:   input.ExitCode,
		Stdout:     input.Stdout,
		Stderr:     input.Stderr,
		StartedAt:  input.StartedAt,
		FinishedAt: input.FinishedAt,
	}
	if err := validateEvidenceRecord(evidence); err != nil {
		return Evidence{}, err
	}
	if evidence.Kind == RedEvidence && evidence.ExitCode == 0 {
		return Evidence{}, errors.New("Red Evidence must record a failing result")
	}
	if evidence.Kind == GreenEvidence && evidence.ExitCode != 0 {
		return Evidence{}, errors.New("Green Evidence must record a passing result")
	}
	contents, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return Evidence{}, fmt.Errorf("encode evidence: %w", err)
	}
	artifact, err := store.WriteArtifact(ctx, history.NewArtifact{
		ID:             input.ID,
		LaborID:        input.LaborID,
		IssueAttemptID: input.IssueAttemptID,
		Kind:           string(input.Kind),
		Name:           string(input.Kind) + "-evidence.json",
		Contents:       append(contents, '\n'),
	})
	if err != nil {
		return Evidence{}, err
	}
	evidence.ArtifactPath = artifact.Path
	return evidence, nil
}

func validateEvidenceRecord(evidence Evidence) error {
	if evidence.Kind != RedEvidence && evidence.Kind != GreenEvidence {
		return fmt.Errorf("unsupported evidence kind %q", evidence.Kind)
	}
	if strings.TrimSpace(evidence.Command) == "" {
		return errors.New("evidence command is required")
	}
	if evidence.StartedAt.IsZero() || evidence.FinishedAt.IsZero() || evidence.FinishedAt.Before(evidence.StartedAt) {
		return errors.New("evidence requires valid command timing")
	}
	return nil
}

// Repository declares verification commands for one touched Target Repository.
type Repository struct {
	Name   string
	Path   string
	Verify []string
}

// Execution is one verification command result.
type Execution struct {
	ExitCode   int
	Stdout     string
	Stderr     string
	StartedAt  time.Time
	FinishedAt time.Time
}

// Verification is one repository-specific quality gate result.
type Verification struct {
	Repository string
	Command    string
	Execution
}

// CommandRunner executes a configured verification command.
type CommandRunner interface {
	Run(context.Context, string, string) (Execution, error)
}

// Verifier runs configured commands for touched Target Repositories.
type Verifier struct {
	Runner CommandRunner
}

// Run executes every configured command for every touched Target Repository.
func (verifier Verifier) Run(ctx context.Context, repositories []Repository, touched []string) ([]Verification, error) {
	if verifier.Runner == nil {
		return nil, errors.New("Verifier requires a CommandRunner")
	}
	touchedSet := make(map[string]bool, len(touched))
	for _, name := range touched {
		touchedSet[name] = true
	}

	var results []Verification
	var failures []error
	for _, repository := range repositories {
		if !touchedSet[repository.Name] {
			continue
		}
		for _, command := range repository.Verify {
			execution, err := verifier.Runner.Run(ctx, repository.Path, command)
			results = append(results, Verification{Repository: repository.Name, Command: command, Execution: execution})
			if err != nil {
				failures = append(failures, fmt.Errorf("%s verification %q: %w", repository.Name, command, err))
			} else if execution.ExitCode != 0 {
				failures = append(failures, fmt.Errorf("%s verification %q failed with exit code %d", repository.Name, command, execution.ExitCode))
			}
		}
	}
	slices.SortFunc(results, func(left, right Verification) int {
		if compared := strings.Compare(left.Repository, right.Repository); compared != 0 {
			return compared
		}
		return strings.Compare(left.Command, right.Command)
	})
	return results, errors.Join(failures...)
}

// ShellRunner executes user-configured verification commands.
type ShellRunner struct{}

// Run executes one verification command through the local shell.
func (ShellRunner) Run(ctx context.Context, workingDirectory, command string) (Execution, error) {
	startedAt := time.Now().UTC()
	var process *exec.Cmd
	if runtime.GOOS == "windows" {
		process = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		process = exec.CommandContext(ctx, "sh", "-lc", command)
	}
	process.Dir = workingDirectory
	stdout, err := process.Output()
	finishedAt := time.Now().UTC()
	execution := Execution{Stdout: string(stdout), StartedAt: startedAt, FinishedAt: finishedAt}
	if err == nil {
		return execution, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		execution.ExitCode = exitError.ExitCode()
		execution.Stderr = string(exitError.Stderr)
	}
	return execution, err
}

// ReviewContext is the complete delivery contract presented to the Reviewer.
type ReviewContext struct {
	Issue        string
	PRD          string
	Changes      string
	Evidence     []Evidence
	Verification []Verification
	TDDExemption string
}

// ReviewOutcome is the structured result of a Reviewer pass.
type ReviewOutcome struct {
	Status            string
	Summary           string
	CorrectiveChanges bool
	Verification      []Verification
}

// ValidateReviewOutcome requires verified corrective changes before completion.
func ValidateReviewOutcome(outcome ReviewOutcome) error {
	if outcome.Status != "completed" && outcome.Status != "blocked" {
		return fmt.Errorf("unsupported Reviewer status %q", outcome.Status)
	}
	if strings.TrimSpace(outcome.Summary) == "" {
		return errors.New("Reviewer outcome requires a summary")
	}
	if outcome.CorrectiveChanges && len(outcome.Verification) == 0 {
		return errors.New("Reviewer corrective changes require rerun verification")
	}
	if outcome.Status == "completed" {
		for _, verification := range outcome.Verification {
			if verification.ExitCode != 0 {
				return fmt.Errorf("Reviewer cannot complete with failing %s verification %q", verification.Repository, verification.Command)
			}
		}
	}
	return nil
}

// ReviewerPrompt builds a complete Reviewer contract that permits verified corrections.
func ReviewerPrompt(context ReviewContext) string {
	return fmt.Sprintf(`Review this Heracles delivery against the complete contract.

## Issue
%s

## PRD
%s

## Changes
%s

## TDD Evidence
Red Evidence and Green Evidence:
%s

TDD Exemption: %s

## Verification
%s

Check correctness and complete issue/PRD compliance. Reject unnecessary scope and premature abstractions (YAGNI), and reject meaningful duplication (DRY). Make corrective changes directly when appropriate, then rerun affected verification and report the verified result.
`, context.Issue, context.PRD, context.Changes, pretty(context.Evidence), context.TDDExemption, pretty(context.Verification))
}

func pretty(value any) string {
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(contents)
}
