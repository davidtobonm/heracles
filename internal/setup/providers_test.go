package setup_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/project"
	"github.com/davidtobonm/heracles/internal/setup"
)

type fakeDoctorSystem struct {
	installed map[string]bool
}

func (system fakeDoctorSystem) LookPath(executable string) (string, error) {
	if system.installed[executable] {
		return "/usr/bin/" + executable, nil
	}
	return "", errors.New(executable + " not installed")
}

func (system fakeDoctorSystem) Run(context.Context, string, ...string) error {
	return nil
}

func (system fakeDoctorSystem) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func TestDetectProvidersReportsExecutableAvailability(t *testing.T) {
	t.Parallel()

	system := fakeDoctorSystem{installed: map[string]bool{"codex": true}}
	availability := setup.DetectProviders(agent.DefaultRegistry(), system)

	want := map[string]bool{"codex": true, "claude": false, "opencode": false, "kimi": false}
	if len(availability) != len(want) {
		t.Fatalf("DetectProviders() returned %d entries, want %d", len(availability), len(want))
	}
	for _, entry := range availability {
		if entry.Available != want[entry.Provider] {
			t.Errorf("DetectProviders() %s available = %v, want %v", entry.Provider, entry.Available, want[entry.Provider])
		}
	}
}

func newSetupIO(input string) (setup.IO, *bytes.Buffer) {
	var out bytes.Buffer
	return setup.IO{In: bufio.NewReader(strings.NewReader(input)), Out: &out}, &out
}

func TestChooseProfileCustomCodexModelAndEffort(t *testing.T) {
	t.Parallel()

	io, _ := newSetupIO("1\n3\ngpt-5.6-custom\n3\n")
	profile, err := setup.ChooseProfile(io, agent.DefaultRegistry(), nil, "Implementer", project.ProfileConfig{})
	if err != nil {
		t.Fatalf("ChooseProfile() error = %v", err)
	}
	want := project.ProfileConfig{Provider: "codex", Model: "gpt-5.6-custom", Effort: "high"}
	if !reflect.DeepEqual(profile, want) {
		t.Errorf("ChooseProfile() = %#v, want %#v", profile, want)
	}
}

func TestChooseProfileKeepsCurrentSelections(t *testing.T) {
	t.Parallel()

	current := project.ProfileConfig{Provider: "claude", Model: "sonnet", Effort: "high"}
	io, _ := newSetupIO("\n\n\n")
	profile, err := setup.ChooseProfile(io, agent.DefaultRegistry(), nil, "Implementer", current)
	if err != nil {
		t.Fatalf("ChooseProfile() error = %v", err)
	}
	if !reflect.DeepEqual(profile, current) {
		t.Errorf("ChooseProfile() = %#v, want unchanged %#v", profile, current)
	}
}

func TestChooseProfileResetsModelAndEffortWhenProviderChanges(t *testing.T) {
	t.Parallel()

	current := project.ProfileConfig{Provider: "codex", Model: "gpt-5.4", Effort: "high"}
	io, _ := newSetupIO("2\n5\n5\n")
	profile, err := setup.ChooseProfile(io, agent.DefaultRegistry(), nil, "Implementer", current)
	if err != nil {
		t.Fatalf("ChooseProfile() error = %v", err)
	}
	want := project.ProfileConfig{Provider: "claude"}
	if !reflect.DeepEqual(profile, want) {
		t.Errorf("ChooseProfile() = %#v, want %#v", profile, want)
	}
}

func TestChooseProfileOpenCodeUsesVariantPrompt(t *testing.T) {
	t.Parallel()

	io, out := newSetupIO("3\n1\nfast\n")
	profile, err := setup.ChooseProfile(io, agent.DefaultRegistry(), nil, "Implementer", project.ProfileConfig{})
	if err != nil {
		t.Fatalf("ChooseProfile() error = %v", err)
	}
	want := project.ProfileConfig{Provider: "opencode", Model: "opencode-go/kimi-k2.6", Variant: "fast"}
	if !reflect.DeepEqual(profile, want) {
		t.Errorf("ChooseProfile() = %#v, want %#v", profile, want)
	}
	if !strings.Contains(out.String(), "variant") {
		t.Errorf("output = %q, want a variant prompt", out.String())
	}
}

func TestChooseProfileAnnotatesUnavailableProviders(t *testing.T) {
	t.Parallel()

	availability := setup.DetectProviders(agent.DefaultRegistry(), fakeDoctorSystem{installed: map[string]bool{"codex": true}})
	io, out := newSetupIO("1\n4\n5\n")
	if _, err := setup.ChooseProfile(io, agent.DefaultRegistry(), availability, "Implementer", project.ProfileConfig{}); err != nil {
		t.Fatalf("ChooseProfile() error = %v", err)
	}
	if !strings.Contains(out.String(), "claude (not installed)") {
		t.Errorf("output = %q, want unavailable providers annotated", out.String())
	}
	if strings.Contains(out.String(), "codex (not installed)") {
		t.Errorf("output = %q, want available provider not annotated", out.String())
	}
}
