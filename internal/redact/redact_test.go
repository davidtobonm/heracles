package redact_test

import (
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/redact"
)

func TestStringReplacesConfiguredSecretValues(t *testing.T) {
	t.Parallel()

	redactor := redact.New([]string{"super-secret-token"})
	got := redactor.String("authorization: bearer super-secret-token (request failed)")
	if strings.Contains(got, "super-secret-token") {
		t.Errorf("String() = %q, secret value leaked", got)
	}
	if !strings.Contains(got, redact.Placeholder) {
		t.Errorf("String() = %q, want placeholder", got)
	}
}

func TestStringIgnoresShortValues(t *testing.T) {
	t.Parallel()

	redactor := redact.New([]string{"abc"})
	got := redactor.String("the abc is fine")
	if got != "the abc is fine" {
		t.Errorf("String() = %q, want unchanged short value preserved", got)
	}
}

func TestStringRedactsLongerValuesBeforeShorterOverlappingOnes(t *testing.T) {
	t.Parallel()

	redactor := redact.New([]string{"secret", "secret-extended"})
	got := redactor.String("token=secret-extended")
	if strings.Contains(got, "secret") {
		t.Errorf("String() = %q, want no remaining secret fragment", got)
	}
}

func TestNilRedactorReturnsInputUnchanged(t *testing.T) {
	t.Parallel()

	var redactor *redact.Redactor
	if got := redactor.String("unchanged"); got != "unchanged" {
		t.Errorf("String() = %q, want %q", got, "unchanged")
	}
}
