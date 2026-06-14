package agent_test

import (
	"testing"

	"github.com/davidtobonm/heracles/internal/agent"
)

func TestDecodeStructuredJSONAcceptsFencedPayload(t *testing.T) {
	t.Parallel()

	var value struct {
		Status string `json:"status"`
	}
	if err := agent.DecodeStructuredJSON("```json\n{\"status\":\"completed\"}\n```", &value); err != nil {
		t.Fatalf("DecodeStructuredJSON() error = %v", err)
	}
	if value.Status != "completed" {
		t.Fatalf("value = %#v, want decoded fenced JSON", value)
	}
}

func TestDecodeStructuredJSONExtractsPayloadFromProse(t *testing.T) {
	t.Parallel()

	var value []struct {
		ID string `json:"id"`
	}
	if err := agent.DecodeStructuredJSON("Here you go:\n[{\"id\":\"slice\"}]\nDone.", &value); err != nil {
		t.Fatalf("DecodeStructuredJSON() error = %v", err)
	}
	if len(value) != 1 || value[0].ID != "slice" {
		t.Fatalf("value = %#v, want extracted JSON array", value)
	}
}

func TestDecodeStructuredJSONRejectsMissingJSON(t *testing.T) {
	t.Parallel()

	var value map[string]any
	if err := agent.DecodeStructuredJSON("no structured payload here", &value); err == nil {
		t.Fatal("DecodeStructuredJSON() error = nil, want invalid JSON failure")
	}
}
