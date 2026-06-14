// Package ci classifies failed CI checks as infrastructure or code failures,
// per the correction-cycle retry policy in PRD.md.
package ci

// Classification distinguishes infrastructure failures, which are retried,
// from code failures, which require a correction cycle.
type Classification string

const (
	// Infrastructure marks a failure caused by the CI environment itself
	// (cancellation, timeout, required action, or runner unavailability)
	// rather than the delivered change.
	Infrastructure Classification = "infrastructure"
	// Code marks a failure attributable to the delivered change (a failing
	// test, build, or lint check, or an unclassified failure).
	Code Classification = "code"
)

// Check is one completed or incomplete CI check result.
type Check struct {
	Name       string
	Status     string
	Conclusion string
}

// infrastructureConclusions are GitHub check conclusions and incomplete
// statuses that indicate the CI environment failed the run rather than the
// delivered change, per PRD.md's CI failure classification table.
var infrastructureConclusions = map[string]bool{
	"cancelled":       true,
	"timed_out":       true,
	"action_required": true,
	"stale":           true,
	"startup_failure": true,
}

// Classify returns the overall classification for one or more failed checks.
// A single check attributable to the delivered change (a code failure)
// classifies the whole set as a code failure; only checks failed entirely
// for infrastructure reasons classify as an infrastructure failure. Unknown
// or empty input defaults to a code failure.
func Classify(checks []Check) Classification {
	if len(checks) == 0 {
		return Code
	}
	for _, check := range checks {
		if !isInfrastructure(check) {
			return Code
		}
	}
	return Infrastructure
}

func isInfrastructure(check Check) bool {
	if infrastructureConclusions[check.Conclusion] {
		return true
	}
	if check.Conclusion == "" && check.Status != "" && check.Status != "completed" {
		// A check that never completed (e.g. the runner never picked it up)
		// is a runner-unavailable infrastructure pattern.
		return true
	}
	return false
}
