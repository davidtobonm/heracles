package ci_test

import (
	"testing"

	"github.com/davidtobonm/heracles/internal/ci"
)

func TestClassifyInfrastructureFailures(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		checks []ci.Check
	}{
		{"cancelled", []ci.Check{{Name: "build", Status: "completed", Conclusion: "cancelled"}}},
		{"timed out", []ci.Check{{Name: "build", Status: "completed", Conclusion: "timed_out"}}},
		{"action required", []ci.Check{{Name: "deploy", Status: "completed", Conclusion: "action_required"}}},
		{"runner unavailable", []ci.Check{{Name: "build", Status: "queued", Conclusion: ""}}},
		{"all infrastructure failures", []ci.Check{
			{Name: "build", Status: "completed", Conclusion: "cancelled"},
			{Name: "lint", Status: "completed", Conclusion: "timed_out"},
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := ci.Classify(testCase.checks); got != ci.Infrastructure {
				t.Errorf("Classify(%#v) = %q, want %q", testCase.checks, got, ci.Infrastructure)
			}
		})
	}
}

func TestClassifyCodeFailures(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		checks []ci.Check
	}{
		{"failing test", []ci.Check{{Name: "test", Status: "completed", Conclusion: "failure"}}},
		{"empty checks default to code", nil},
		{"mixed infrastructure and code failures", []ci.Check{
			{Name: "build", Status: "completed", Conclusion: "cancelled"},
			{Name: "test", Status: "completed", Conclusion: "failure"},
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := ci.Classify(testCase.checks); got != ci.Code {
				t.Errorf("Classify(%#v) = %q, want %q", testCase.checks, got, ci.Code)
			}
		})
	}
}
