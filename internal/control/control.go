// Package control exposes high-level Heracles application operations.
package control

import (
	"context"
	"errors"
)

// Operation is one typed Control Surface request shared by CLI and MCP.
type Operation struct {
	Name         string   `json:"name"`
	Kind         string   `json:"kind,omitempty"`
	ID           string   `json:"id,omitempty"`
	Problem      string   `json:"problem,omitempty"`
	PRD          string   `json:"prd,omitempty"`
	Decision     string   `json:"decision,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	PRDIssueURL  string   `json:"prd_issue_url,omitempty"`
	Limit        int      `json:"limit,omitempty"`
	Tracker      string   `json:"tracker,omitempty"`
	Repositories []string `json:"repositories,omitempty"`
	// RetryUntilPass permits unbounded correction cycles for trusted,
	// unattended launches, per PRD.md's correction-cycle policy.
	RetryUntilPass bool `json:"retry_until_pass,omitempty"`
}

// Result is one stable machine-readable Control Surface outcome.
type Result struct {
	Operation string `json:"operation"`
	Kind      string `json:"kind,omitempty"`
	ID        string `json:"id,omitempty"`
	Status    string `json:"status"`
	Data      any    `json:"data,omitempty"`
}

// Surface executes policy-preserving high-level operations.
type Surface interface {
	Execute(context.Context, Operation) (Result, error)
	Close() error
}

// ErrUnsupported indicates an unknown high-level operation.
var ErrUnsupported = errors.New("unsupported Control Surface operation")
