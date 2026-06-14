package agent

import (
	"fmt"
	"slices"
)

// FullPermissionPrompt returns the disclosure Heracles shows before the first
// interactive full-permission Labor that launches provider, per ADR 0023.
func FullPermissionPrompt(provider string) string {
	return fmt.Sprintf("Heracles will launch %s with its verified full-permission bypass, granting unattended access to your shell and Target Repositories for this Labor. Continue?", provider)
}

// FullPermissionAcknowledgementRequired reports whether the first interactive
// full-permission Labor that launches provider must show FullPermissionPrompt
// and obtain explicit acknowledgement before launch. acknowledgedProviders are
// the providers a project has already acknowledged.
func FullPermissionAcknowledgementRequired(acknowledgedProviders []string, provider string) bool {
	return !slices.Contains(acknowledgedProviders, provider)
}
