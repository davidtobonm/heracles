// Package issuestage authors and publishes durable Heracles-compatible issue plans.
package issuestage

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/davidtobonm/heracles/internal/tracker"
)

const (
	StatusActive           = "active"
	StatusAwaitingApproval = "awaiting_approval"
	StatusApproved         = "approved"
	StatusRejected         = "rejected"
	StatusPublishing       = "publishing"
	StatusPublished        = "published"

	GatePending  = "pending"
	GateApproved = "approved"
	GateRejected = "rejected"

	DecisionApprove = "approve"
	DecisionReject  = "reject"

	TypeAFK  = "AFK"
	TypeHITL = "HITL"
)

// ErrNotFound indicates that no durable Issue Stage exists for an ID.
var ErrNotFound = errors.New("Issue Stage not found")

// Proposal is one tracer-bullet issue proposed from an approved PRD.
type Proposal struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Type               string   `json:"type"`
	UserStories        []int    `json:"user_stories"`
	WhatToBuild        string   `json:"what_to_build"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	BlockedBy          []string `json:"blocked_by,omitempty"`
	ExclusiveScopes    []string `json:"exclusive_scopes,omitempty"`
	TDDExemptionReason string   `json:"tdd_exemption_reason,omitempty"`
}

// Gate is the durable publication approval decision.
type Gate struct {
	Status   string `json:"status"`
	Decision string `json:"decision,omitempty"`
}

// State is the complete durable Issue Stage state.
type State struct {
	ID                string            `json:"id"`
	ApprovedPRDPath   string            `json:"approved_prd_path,omitempty"`
	ApprovedPRD       string            `json:"approved_prd"`
	TrackerRepository string            `json:"tracker_repository"`
	Status            string            `json:"status"`
	Proposals         []Proposal        `json:"proposals,omitempty"`
	Gate              Gate              `json:"gate"`
	Published         map[string]string `json:"published,omitempty"`
}

// RunRequest starts or resumes an Issue Stage.
type RunRequest struct {
	ID                string
	ApprovedPRDPath   string
	ApprovedPRD       string
	TrackerRepository string
}

// AuthorRequest is the approved product context sent to the Issue Author.
type AuthorRequest struct {
	ApprovedPRDPath   string
	ApprovedPRD       string
	TrackerRepository string
	RevisionFeedback  string
}

// IssueAuthor proposes executable tracer-bullet issues.
type IssueAuthor interface {
	Propose(context.Context, AuthorRequest) ([]Proposal, error)
}

// Store persists Issue Stage state.
type Store interface {
	Load(context.Context, string) (State, error)
	Save(context.Context, State) error
}

// PublishInput is one GitHub issue publication request.
type PublishInput struct {
	Repository string
	Title      string
	Body       string
	Labels     []string
}

// Publisher creates approved issues.
type Publisher interface {
	CreateIssue(context.Context, PublishInput) (string, error)
}

// Service runs, decides, and publishes Issue Stages.
type Service struct {
	Author    IssueAuthor
	Store     Store
	Publisher Publisher
}

// Run starts or resumes an Issue Stage until its Approval Gate.
func (service Service) Run(ctx context.Context, request RunRequest) (State, error) {
	if request.ID == "" {
		return State{}, errors.New("Issue Stage requires an ID")
	}
	if service.Author == nil || service.Store == nil {
		return State{}, errors.New("Issue Stage requires an Issue Author and Store")
	}
	state, err := service.Store.Load(ctx, request.ID)
	if errors.Is(err, ErrNotFound) {
		if request.ApprovedPRD == "" || request.TrackerRepository == "" {
			return State{}, errors.New("new Issue Stage requires an approved PRD and Issue Tracker repository")
		}
		state = State{
			ID: request.ID, ApprovedPRDPath: request.ApprovedPRDPath, ApprovedPRD: request.ApprovedPRD,
			TrackerRepository: request.TrackerRepository, Status: StatusActive, Published: make(map[string]string),
		}
		if err := service.Store.Save(ctx, state); err != nil {
			return State{}, err
		}
	} else if err != nil {
		return State{}, err
	}

	switch state.Status {
	case StatusAwaitingApproval, StatusApproved, StatusPublishing, StatusPublished:
		return state, nil
	case StatusRejected, StatusActive:
	default:
		return state, fmt.Errorf("unknown Issue Stage status %q", state.Status)
	}
	proposals, err := service.Author.Propose(ctx, AuthorRequest{
		ApprovedPRDPath: state.ApprovedPRDPath, ApprovedPRD: state.ApprovedPRD,
		TrackerRepository: state.TrackerRepository, RevisionFeedback: state.Gate.Decision,
	})
	if err != nil {
		return state, fmt.Errorf("Issue Author: %w", err)
	}
	if err := ValidateProposals(proposals); err != nil {
		return state, err
	}
	state.Proposals = append([]Proposal(nil), proposals...)
	state.Status = StatusAwaitingApproval
	state.Gate = Gate{Status: GatePending}
	return state, service.Store.Save(ctx, state)
}

// Decide approves or rejects proposed issues.
func (service Service) Decide(ctx context.Context, id, decision, reason string) (State, error) {
	state, err := service.Store.Load(ctx, id)
	if err != nil {
		return State{}, err
	}
	if (state.Status == StatusApproved && decision == DecisionApprove) || (state.Status == StatusRejected && decision == DecisionReject) {
		return state, nil
	}
	if state.Status != StatusAwaitingApproval {
		return state, fmt.Errorf("Issue Stage %q is not awaiting approval", id)
	}
	switch decision {
	case DecisionApprove:
		state.Status = StatusApproved
		state.Gate = Gate{Status: GateApproved, Decision: reason}
	case DecisionReject:
		state.Status = StatusRejected
		state.Gate = Gate{Status: GateRejected, Decision: reason}
	default:
		return state, fmt.Errorf("unknown Issue Stage decision %q", decision)
	}
	return state, service.Store.Save(ctx, state)
}

// Publish idempotently publishes approved proposals.
func (service Service) Publish(ctx context.Context, id string) (State, error) {
	if service.Publisher == nil || service.Store == nil {
		return State{}, errors.New("Issue publication requires a Publisher and Store")
	}
	state, err := service.Store.Load(ctx, id)
	if err != nil {
		return State{}, err
	}
	if state.Status == StatusPublished {
		return state, nil
	}
	if state.Status != StatusApproved && state.Status != StatusPublishing {
		return state, fmt.Errorf("Issue Stage %q is not approved for publication", id)
	}
	if state.Published == nil {
		state.Published = make(map[string]string)
	}
	if state.Status == StatusApproved {
		state.Status = StatusPublishing
		if err := service.Store.Save(ctx, state); err != nil {
			return state, err
		}
	}
	for _, proposal := range state.Proposals {
		if state.Published[proposal.ID] != "" {
			continue
		}
		url, err := service.Publisher.CreateIssue(ctx, PublishInput{
			Repository: state.TrackerRepository,
			Title:      proposal.Title,
			Body:       Body(proposal),
			Labels:     Labels(proposal),
		})
		if err != nil {
			return state, fmt.Errorf("publish proposal %q: %w", proposal.ID, err)
		}
		state.Published[proposal.ID] = url
		if err := service.Store.Save(ctx, state); err != nil {
			return state, err
		}
	}
	state.Status = StatusPublished
	return state, service.Store.Save(ctx, state)
}

// ValidateProposals validates the Heracles issue contract.
func ValidateProposals(proposals []Proposal) error {
	if len(proposals) == 0 {
		return errors.New("Issue Author must propose at least one tracer-bullet issue")
	}
	ids := make(map[string]struct{}, len(proposals))
	for index, proposal := range proposals {
		if proposal.ID == "" || proposal.Title == "" || proposal.WhatToBuild == "" || len(proposal.UserStories) == 0 || len(proposal.AcceptanceCriteria) == 0 {
			return fmt.Errorf("proposal %d requires id, title, user stories, what to build, and acceptance criteria", index+1)
		}
		if proposal.Type != TypeAFK && proposal.Type != TypeHITL {
			return fmt.Errorf("proposal %q must classify as AFK or HITL", proposal.ID)
		}
		if _, exists := ids[proposal.ID]; exists {
			return fmt.Errorf("duplicate proposal ID %q", proposal.ID)
		}
		ids[proposal.ID] = struct{}{}
		for _, dependency := range proposal.BlockedBy {
			if _, err := tracker.ParseReference(dependency); err != nil {
				return fmt.Errorf("proposal %q dependency: %w", proposal.ID, err)
			}
		}
	}
	return nil
}

// Labels returns the executable shared-state labels for a proposal.
func Labels(proposal Proposal) []string {
	var labels []string
	switch {
	case proposal.Type == TypeHITL:
		labels = append(labels, tracker.LabelHITL)
	default:
		labels = append(labels, tracker.LabelReady)
	}
	if proposal.TDDExemptionReason != "" {
		labels = append(labels, tracker.LabelTDDExempt)
	}
	slices.Sort(labels)
	return labels
}

// Body renders one Heracles-compatible GitHub issue.
func Body(proposal Proposal) string {
	return fmt.Sprintf(`## Type

%s

## User stories covered

%s

## What to build

%s

## Acceptance criteria

%s

## Blocked by

%s

## Exclusive Scopes

%s
`, proposal.Type, numbers(proposal.UserStories), proposal.WhatToBuild, bullets(proposal.AcceptanceCriteria), bullets(proposal.BlockedBy), bullets(proposal.ExclusiveScopes))
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
