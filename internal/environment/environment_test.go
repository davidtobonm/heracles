package environment_test

import (
	"slices"
	"testing"

	"github.com/davidtobonm/heracles/internal/environment"
)

func TestFilterIncludesEssentialVariablesAndAllowlist(t *testing.T) {
	t.Parallel()

	source := []string{"PATH=/usr/bin", "HOME=/home/dev", "HERACLES_ALLOWED=yes", "HERACLES_SECRET=no"}
	got := environment.Filter([]string{"HERACLES_ALLOWED"}, source)

	if !slices.Contains(got, "PATH=/usr/bin") || !slices.Contains(got, "HOME=/home/dev") {
		t.Errorf("Filter() = %#v, want essential variables included", got)
	}
	if !slices.Contains(got, "HERACLES_ALLOWED=yes") {
		t.Errorf("Filter() = %#v, want allowlisted variable included", got)
	}
	for _, entry := range got {
		if entry == "HERACLES_SECRET=no" {
			t.Errorf("Filter() = %#v, should exclude non-allowlisted variable", got)
		}
	}
}

func TestFilterDeduplicatesNames(t *testing.T) {
	t.Parallel()

	source := []string{"PATH=/usr/bin"}
	got := environment.Filter([]string{"PATH"}, source)
	if len(got) != 1 || got[0] != "PATH=/usr/bin" {
		t.Errorf("Filter() = %#v, want single deduplicated entry", got)
	}
}

func TestMissingReportsAbsentOrEmptyRequiredVariables(t *testing.T) {
	t.Parallel()

	source := []string{"API_KEY=secret-value", "EMPTY_VAR="}
	missing := environment.Missing([]string{"API_KEY", "EMPTY_VAR", "DB_URL"}, source)
	if want := []string{"EMPTY_VAR", "DB_URL"}; !slices.Equal(missing, want) {
		t.Errorf("Missing() = %#v, want %#v", missing, want)
	}

	if missing := environment.Missing([]string{"API_KEY"}, source); len(missing) != 0 {
		t.Errorf("Missing() = %#v, want none missing", missing)
	}
}

func TestIsSecretNameMatchesCommonCredentialPatterns(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"API_KEY", "GITHUB_TOKEN", "DB_PASSWORD", "AWS_SECRET_ACCESS_KEY", "TLS_CERT"} {
		if !environment.IsSecretName(name) {
			t.Errorf("IsSecretName(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"PATH", "HOME", "DB_URL", "USER"} {
		if environment.IsSecretName(name) {
			t.Errorf("IsSecretName(%q) = true, want false", name)
		}
	}
}

func TestSecretValuesReturnsOnlyValuesOfSecretLikeNames(t *testing.T) {
	t.Parallel()

	env := []string{"PATH=/usr/bin", "API_KEY=super-secret-value", "DB_URL=postgres://localhost"}
	values := environment.SecretValues(env)
	if want := []string{"super-secret-value"}; !slices.Equal(values, want) {
		t.Errorf("SecretValues() = %#v, want %#v", values, want)
	}
}
