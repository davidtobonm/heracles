// Package correction decides whether a blocked Change Set delivery should be
// retried through a preserved-workspace correction cycle or block the issue,
// per PRD.md's correction-cycle policy.
package correction

import (
	"time"

	"github.com/davidtobonm/heracles/internal/ci"
)

// DefaultMaxCycles is the default number of correction cycles attempted
// before an issue blocks, per PRD.md.
const DefaultMaxCycles = 3

// Decision is the outcome of evaluating one correction cycle.
type Decision string

const (
	// Retry reruns implementation and review against the preserved Issue
	// Workspace with failure or review context, then redelivers.
	Retry Decision = "retry"
	// Block exhausts the correction budget; the issue and Labor block,
	// preserving work and evidence.
	Block Decision = "block"
)

// Policy bounds correction cycles for one issue.
type Policy struct {
	// MaxCycles is the maximum number of correction cycles attempted before
	// blocking. Zero uses DefaultMaxCycles.
	MaxCycles int
	// RetryUntilPass permits unbounded correction cycles for trusted,
	// unattended launches.
	RetryUntilPass bool
}

// Decide returns whether a blocked delivery should be retried and, for
// infrastructure failures, how long to wait before retrying. Requested
// changes and code failures retry immediately; infrastructure failures use a
// short exponential backoff before the retry that consumes the cycle.
func Decide(cyclesUsed int, classification ci.Classification, policy Policy) (Decision, time.Duration) {
	maxCycles := policy.MaxCycles
	if maxCycles <= 0 {
		maxCycles = DefaultMaxCycles
	}
	if !policy.RetryUntilPass && cyclesUsed >= maxCycles {
		return Block, 0
	}
	if classification == ci.Infrastructure {
		return Retry, Backoff(cyclesUsed)
	}
	return Retry, 0
}

// maxBackoff caps the exponential backoff applied between infrastructure
// failure retries.
const maxBackoff = 30 * time.Second

// Backoff returns the exponential backoff delay for the given retry attempt
// (zero-indexed), capped at maxBackoff.
func Backoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt > 5 {
		return maxBackoff
	}
	delay := time.Second << attempt
	if delay > maxBackoff {
		return maxBackoff
	}
	return delay
}
