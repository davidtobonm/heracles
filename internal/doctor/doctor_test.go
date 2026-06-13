package doctor_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/doctor"
	"github.com/davidtobonm/heracles/internal/project"
)

func TestCheckReportsRepositoryGitHubExecutableAndCapabilityFailures(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	loaded := project.LoadedConfig{
		Path: filepath.Join(root, "heracles.yaml"),
		Config: project.Config{
			Version:      1,
			IssueTracker: project.IssueTrackerConfig{GitHub: "acme/backlog"},
			Repositories: []project.RepositoryConfig{{Name: "app", Path: ".", GitHub: "acme/app", BaseBranch: "main"}},
			Agents: project.AgentConfig{
				DefaultProfile: "default",
				Profiles: map[string]project.ProfileConfig{
					"default": {Provider: "codex", Effort: "medium"},
					"review":  {Provider: "opencode", Effort: "high"},
				},
				Roles: project.RoleConfig{Reviewer: "review"},
			},
		},
	}
	system := &fakeSystem{
		missing: map[string]error{"codex": errors.New("not installed")},
		runErr:  map[string]error{"gh": errors.New("not authenticated")},
	}

	report := doctor.Check(context.Background(), loaded, agent.DefaultRegistry(), system)

	if report.OK {
		t.Fatal("Check() report is OK, want failures")
	}
	for _, expected := range []string{"not installed", "not authenticated", "does not support effort"} {
		if !strings.Contains(report.String(), expected) {
			t.Errorf("report %q does not contain %q", report.String(), expected)
		}
	}
	if len(system.calls) == 0 {
		t.Fatal("Check() did not probe repositories and GitHub authentication")
	}
}

type fakeSystem struct {
	missing map[string]error
	runErr  map[string]error
	calls   []string
}

func (system *fakeSystem) LookPath(executable string) (string, error) {
	if err := system.missing[executable]; err != nil {
		return "", err
	}
	return "/fake/" + executable, nil
}

func (system *fakeSystem) Run(_ context.Context, command string, args ...string) error {
	system.calls = append(system.calls, command+" "+strings.Join(args, " "))
	return system.runErr[command]
}
