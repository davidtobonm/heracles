package correction_test

import (
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/ci"
	"github.com/davidtobonm/heracles/internal/correction"
)

func TestDecideRetriesRequestedChangesImmediately(t *testing.T) {
	t.Parallel()

	decision, wait := correction.Decide(0, ci.Code, correction.Policy{})
	if decision != correction.Retry || wait != 0 {
		t.Errorf("Decide() = %v, %v; want immediate retry", decision, wait)
	}
}

func TestDecideBacksOffInfrastructureFailures(t *testing.T) {
	t.Parallel()

	decision, wait := correction.Decide(1, ci.Infrastructure, correction.Policy{})
	if decision != correction.Retry || wait != 2*time.Second {
		t.Errorf("Decide() = %v, %v; want retry after exponential backoff", decision, wait)
	}
}

func TestDecideBlocksAfterDefaultCycles(t *testing.T) {
	t.Parallel()

	decision, _ := correction.Decide(correction.DefaultMaxCycles, ci.Code, correction.Policy{})
	if decision != correction.Block {
		t.Errorf("Decide() = %v, want block after %d cycles", decision, correction.DefaultMaxCycles)
	}
}

func TestDecideHonorsConfiguredMaxCycles(t *testing.T) {
	t.Parallel()

	decision, _ := correction.Decide(1, ci.Code, correction.Policy{MaxCycles: 1})
	if decision != correction.Block {
		t.Errorf("Decide() = %v, want block after configured max cycles", decision)
	}
}

func TestDecideRetryUntilPassNeverBlocks(t *testing.T) {
	t.Parallel()

	decision, _ := correction.Decide(100, ci.Code, correction.Policy{MaxCycles: 1, RetryUntilPass: true})
	if decision != correction.Retry {
		t.Errorf("Decide() = %v, want retry-until-pass to never block", decision)
	}
}

func TestBackoffCapsAtMaximum(t *testing.T) {
	t.Parallel()

	if got := correction.Backoff(10); got != 30*time.Second {
		t.Errorf("Backoff(10) = %v, want capped at 30s", got)
	}
	if got := correction.Backoff(0); got != time.Second {
		t.Errorf("Backoff(0) = %v, want 1s", got)
	}
}
