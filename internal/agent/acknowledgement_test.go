package agent_test

import (
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/agent"
)

func TestFullPermissionAcknowledgementRequiredUntilProviderIsAcknowledged(t *testing.T) {
	t.Parallel()

	if !agent.FullPermissionAcknowledgementRequired(nil, "claude") {
		t.Error("FullPermissionAcknowledgementRequired(nil, claude) = false, want true for a project with no acknowledgements")
	}
	if !agent.FullPermissionAcknowledgementRequired([]string{"codex"}, "claude") {
		t.Error("FullPermissionAcknowledgementRequired([codex], claude) = false, want true for an unacknowledged provider")
	}
	if agent.FullPermissionAcknowledgementRequired([]string{"claude"}, "claude") {
		t.Error("FullPermissionAcknowledgementRequired([claude], claude) = true, want false once acknowledged")
	}
}

func TestFullPermissionPromptDescribesUnattendedAccess(t *testing.T) {
	t.Parallel()

	prompt := agent.FullPermissionPrompt("claude")
	for _, expected := range []string{"claude", "full-permission", "unattended"} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("FullPermissionPrompt() = %q, want it to contain %q", prompt, expected)
		}
	}
}
