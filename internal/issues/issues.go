// Package issues generates and reconciles the implementation issues for one
// approved PRD, per ADR 0015 and ADR 0025.
package issues

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/davidtobonm/heracles/internal/reconcile"
	"github.com/davidtobonm/heracles/internal/tracker"
)

const (
	// TypeAFK marks a proposal as agent-deliverable without human review.
	TypeAFK = "AFK"
	// TypeHITL marks a proposal as requiring human-in-the-loop work.
	TypeHITL = "HITL"

	// StatusStarted marks a Parent PRD revision whose implementation issue
	// generation has started in the background and not yet completed.
	StatusStarted = "started"
	// StatusGenerated marks a Parent PRD revision whose implementation
	// issues have been published and reconciled.
	StatusGenerated = "generated"
	// StatusBlocked marks a Parent PRD revision the Issue Author could not
	// safely decompose; no implementation issues were published.
	StatusBlocked = "blocked"
)

// ErrNotFound indicates that no durable issue generation state exists for an ID.
var ErrNotFound = errors.New("issue generation state not found")

// Proposal is one implementation issue proposed from an approved PRD.
type Proposal struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Type               string   `json:"type"`
	UserStories        []int    `json:"user_stories"`
	WhatToBuild        string   `json:"what_to_build"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	TargetRepositories []string `json:"target_repositories"`
	ConflictKeys       []string `json:"conflict_keys,omitempty"`
	BlockedBy          []string `json:"blocked_by,omitempty"`
	TDDExemptionReason string   `json:"tdd_exemption_reason,omitempty"`
}

// AuthorRequest is the approved PRD context sent to the Issue Author.
type AuthorRequest struct {
	ParentPRDURL      string `json:"parent_prd_url"`
	ApprovedPRD       string `json:"approved_prd"`
	TrackerRepository string `json:"tracker_repository"`
}

// AuthorResponse is the Issue Author's proposed implementation issues, or an
// exceptional-ambiguity block reason.
type AuthorResponse struct {
	Proposals []Proposal `json:"proposals,omitempty"`
	Blocked   string     `json:"blocked,omitempty"`
}

// Author proposes executable implementation issues from an approved PRD.
type Author interface {
	Propose(context.Context, AuthorRequest) (AuthorResponse, error)
}

// State is the durable issue generation state for one Parent PRD.
type State struct {
	ID           string            `json:"id"`
	ParentPRDURL string            `json:"parent_prd_url"`
	Revision     string            `json:"revision"`
	Status       string            `json:"status"`
	Blocked      string            `json:"blocked,omitempty"`
	Published    map[string]string `json:"published,omitempty"`
}

// Store persists issue generation state.
type Store interface {
	Load(context.Context, string) (State, error)
	Save(context.Context, State) error
}

// TrackerClient is the Issue Tracker boundary used to reconcile and publish
// implementation issues.
type TrackerClient interface {
	ListOpenIssues(context.Context, string) ([]tracker.Issue, error)
	CreateIssue(context.Context, string, string, string, []string) (string, error)
	UpdateIssue(context.Context, tracker.Reference, string, string, []string) error
	SetLabels(context.Context, tracker.Reference, []string) error
	Comment(context.Context, tracker.Reference, string) error
}

// GenerateRequest starts or resumes issue generation for one approved PRD.
type GenerateRequest struct {
	ID                string
	ParentPRDURL      string
	ApprovedPRD       string
	TrackerRepository string
}

// Service generates and reconciles implementation issues.
type Service struct {
	Author  Author
	Tracker TrackerClient
	Store   Store
}

// Generate proposes, validates, reconciles, and publishes the implementation
// issues for one approved PRD revision. Regenerating an unchanged revision
// performs no mutations.
func (service Service) Generate(ctx context.Context, request GenerateRequest) (State, error) {
	if request.ID == "" || request.ParentPRDURL == "" || request.ApprovedPRD == "" || request.TrackerRepository == "" {
		return State{}, errors.New("issue generation requires an ID, Parent PRD URL, approved PRD, and Issue Tracker repository")
	}
	if service.Author == nil || service.Tracker == nil || service.Store == nil {
		return State{}, errors.New("issue generation requires an Issue Author, Tracker client, and Store")
	}

	revision := Revision(request.ApprovedPRD)
	state, err := service.Store.Load(ctx, request.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return State{}, err
	}
	if err == nil && state.Revision == revision && (state.Status == StatusGenerated || state.Status == StatusBlocked) {
		return state, nil
	}

	response, err := service.Author.Propose(ctx, AuthorRequest{
		ParentPRDURL: request.ParentPRDURL, ApprovedPRD: request.ApprovedPRD, TrackerRepository: request.TrackerRepository,
	})
	if err != nil {
		return State{}, fmt.Errorf("Issue Author: %w", err)
	}
	if response.Blocked != "" {
		if reference, parseErr := tracker.ParseReference(request.ParentPRDURL); parseErr == nil {
			_ = service.Tracker.Comment(ctx, reference, "Heracles Issue Author blocked: "+response.Blocked)
		}
		state = State{ID: request.ID, ParentPRDURL: request.ParentPRDURL, Revision: revision, Status: StatusBlocked, Blocked: response.Blocked}
		return state, service.Store.Save(ctx, state)
	}

	order, err := ValidateProposals(response.Proposals)
	if err != nil {
		return State{}, err
	}

	existingIssues, err := service.Tracker.ListOpenIssues(ctx, request.TrackerRepository)
	if err != nil {
		return State{}, fmt.Errorf("list existing implementation issues: %w", err)
	}
	var existing []reconcile.Existing
	existingByID := make(map[string]tracker.Issue)
	for _, issue := range existingIssues {
		parentURL, ok := tracker.ParentPRDURL(issue.Body)
		if !ok || parentURL != request.ParentPRDURL {
			continue
		}
		semanticID, ok := SemanticID(issue.Body)
		if !ok {
			continue
		}
		existing = append(existing, reconcile.Existing{
			SemanticID: semanticID, URL: issue.URL,
			InProgress: slices.Contains(issue.Labels, tracker.LabelInProgress),
			Done:       slices.Contains(issue.Labels, tracker.LabelDone),
		})
		existingByID[semanticID] = issue
	}

	proposalsByID := make(map[string]Proposal, len(response.Proposals))
	for _, proposal := range response.Proposals {
		proposalsByID[proposal.ID] = proposal
	}

	decisions := reconcile.Plan(existing, order)
	decisionByID := make(map[string]reconcile.Decision, len(decisions))
	for _, decision := range decisions {
		decisionByID[decision.SemanticID] = decision
	}

	published := make(map[string]string, len(order))
	for id, decision := range decisionByID {
		if decision.Action == reconcile.ActionUpdate || decision.Action == reconcile.ActionSkip {
			published[id] = decision.Existing.URL
		}
	}

	for _, id := range order {
		proposal := proposalsByID[id]
		decision := decisionByID[id]
		switch decision.Action {
		case reconcile.ActionCreate:
			body := Body(proposal, request.ParentPRDURL, revision, published)
			url, err := service.Tracker.CreateIssue(ctx, request.TrackerRepository, proposal.Title, body, Labels(proposal))
			if err != nil {
				return state, fmt.Errorf("publish proposal %q: %w", id, err)
			}
			published[id] = url
		case reconcile.ActionUpdate:
			body := Body(proposal, request.ParentPRDURL, revision, published)
			reference, err := tracker.ParseReference(decision.Existing.URL)
			if err != nil {
				return state, err
			}
			if err := service.Tracker.UpdateIssue(ctx, reference, proposal.Title, body, Labels(proposal)); err != nil {
				return state, fmt.Errorf("update proposal %q: %w", id, err)
			}
		case reconcile.ActionSkip:
			// Leave in-progress or completed work untouched.
		}
	}

	for _, decision := range decisions {
		if decision.Action != reconcile.ActionObsolete {
			continue
		}
		reference, err := tracker.ParseReference(decision.Existing.URL)
		if err != nil {
			return state, err
		}
		issue := existingByID[decision.SemanticID]
		if err := service.Tracker.SetLabels(ctx, reference, obsoleteLabels(issue.Labels)); err != nil {
			return state, fmt.Errorf("mark %q obsolete: %w", decision.SemanticID, err)
		}
		if err := service.Tracker.Comment(ctx, reference, "Superseded by PRD revision "+revision+"."); err != nil {
			return state, fmt.Errorf("comment obsolete %q: %w", decision.SemanticID, err)
		}
	}

	state = State{ID: request.ID, ParentPRDURL: request.ParentPRDURL, Revision: revision, Status: StatusGenerated, Published: published}
	return state, service.Store.Save(ctx, state)
}

// ValidateProposals validates the implementation issue contract and returns
// proposal IDs in dependency order, with dependencies before dependents.
func ValidateProposals(proposals []Proposal) ([]string, error) {
	if len(proposals) == 0 {
		return nil, errors.New("Issue Author must propose at least one implementation issue or report a blocking ambiguity")
	}
	ids := make(map[string]struct{}, len(proposals))
	order := make([]string, len(proposals))
	for index, proposal := range proposals {
		if proposal.ID == "" || proposal.Title == "" || proposal.WhatToBuild == "" || len(proposal.UserStories) == 0 || len(proposal.AcceptanceCriteria) == 0 || len(proposal.TargetRepositories) == 0 {
			return nil, fmt.Errorf("proposal %d requires id, title, user stories, what to build, acceptance criteria, and target repositories", index+1)
		}
		if proposal.Type != TypeAFK && proposal.Type != TypeHITL {
			return nil, fmt.Errorf("proposal %q must classify as AFK or HITL", proposal.ID)
		}
		if _, exists := ids[proposal.ID]; exists {
			return nil, fmt.Errorf("duplicate semantic issue ID %q", proposal.ID)
		}
		ids[proposal.ID] = struct{}{}
		order[index] = proposal.ID
	}
	indegree := make(map[string]int, len(proposals))
	edges := make(map[string][]string, len(proposals))
	for _, proposal := range proposals {
		for _, dependency := range proposal.BlockedBy {
			if _, ok := ids[dependency]; ok {
				edges[dependency] = append(edges[dependency], proposal.ID)
				indegree[proposal.ID]++
				continue
			}
			if _, err := tracker.ParseReference(dependency); err != nil {
				return nil, fmt.Errorf("proposal %q dependency %q is neither a known semantic ID nor a full GitHub issue URL", proposal.ID, dependency)
			}
		}
	}
	var queue []string
	for _, id := range order {
		if indegree[id] == 0 {
			queue = append(queue, id)
		}
	}
	sorted := make([]string, 0, len(order))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		sorted = append(sorted, id)
		for _, next := range edges[id] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if len(sorted) != len(order) {
		return nil, errors.New("Issue Author dependency graph must be acyclic")
	}
	return sorted, nil
}

// Labels returns the executable shared-state labels for a proposal.
func Labels(proposal Proposal) []string {
	labels := []string{tracker.LabelImplementation}
	if proposal.Type == TypeHITL {
		labels = append(labels, tracker.LabelHITL)
	} else {
		labels = append(labels, tracker.LabelReady)
	}
	if proposal.TDDExemptionReason != "" {
		labels = append(labels, tracker.LabelTDDExempt)
	}
	slices.Sort(labels)
	return labels
}

// Body renders one Heracles-compatible implementation issue, substituting
// full GitHub URLs for semantic-ID dependencies already published in
// dependencyURLs and embedding the Parent PRD URL, semantic issue ID, and
// PRD revision marker.
func Body(proposal Proposal, parentPRDURL, revision string, dependencyURLs map[string]string) string {
	blockedBy := make([]string, len(proposal.BlockedBy))
	for index, dependency := range proposal.BlockedBy {
		if url, ok := dependencyURLs[dependency]; ok {
			blockedBy[index] = url
		} else {
			blockedBy[index] = dependency
		}
	}
	return fmt.Sprintf(`## Type

%s

## User stories covered

%s

## What to build

%s

## Acceptance criteria

%s

## TDD Exemption

%s

## Target Repositories

%s

## Conflict Keys

%s

## Blocked by

%s

## Parent PRD

%s

<!-- heracles:issue-id=%s -->
<!-- heracles:prd-revision=%s -->
`, proposal.Type, numbers(proposal.UserStories), proposal.WhatToBuild, bullets(proposal.AcceptanceCriteria),
		tddExemption(proposal.TDDExemptionReason), bullets(proposal.TargetRepositories), bullets(proposal.ConflictKeys),
		bullets(blockedBy), parentPRDURL, proposal.ID, revision)
}

// SemanticID returns the Issue Author-assigned semantic ID embedded in an
// implementation issue's body, if present.
func SemanticID(body string) (string, bool) {
	const prefix = "<!-- heracles:issue-id="
	start := strings.Index(body, prefix)
	if start == -1 {
		return "", false
	}
	rest := body[start+len(prefix):]
	end := strings.Index(rest, " -->")
	if end == -1 {
		return "", false
	}
	return strings.TrimSpace(rest[:end]), true
}

// Revision returns the stable revision marker for an approved PRD's content,
// excluding any previously embedded PRD revision marker.
func Revision(prd string) string {
	sum := sha256.Sum256([]byte(stripRevisionMarker(prd)))
	return hex.EncodeToString(sum[:])
}

func stripRevisionMarker(body string) string {
	const prefix = "<!-- heracles:prd-revision"
	start := strings.Index(body, prefix)
	if start == -1 {
		return body
	}
	rest := body[start:]
	end := strings.Index(rest, " -->")
	if end == -1 {
		return body
	}
	return strings.TrimSpace(body[:start] + rest[end+len(" -->"):])
}

func obsoleteLabels(labels []string) []string {
	result := make([]string, 0, len(labels)+1)
	for _, label := range labels {
		if label != tracker.LabelReady {
			result = append(result, label)
		}
	}
	result = append(result, tracker.LabelObsolete)
	slices.Sort(result)
	return slices.Compact(result)
}

func numbers(values []int) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.Itoa(value)
	}
	return strings.Join(parts, ", ")
}

func bullets(values []string) string {
	if len(values) == 0 {
		return "- None"
	}
	lines := make([]string, len(values))
	for index, value := range values {
		lines[index] = "- " + value
	}
	return strings.Join(lines, "\n")
}

// tddExemption renders the Issue Author's stated rationale for exempting a
// proposal from Red and Green Evidence, or an explicit non-exemption notice
// when reason is empty, per skills/to-issues-for-heracles/SKILL.md.
func tddExemption(reason string) string {
	if reason == "" {
		return "Not exempt; Red and Green Evidence required."
	}
	return reason
}
