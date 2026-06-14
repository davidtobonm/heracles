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
	output  map[string]string
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

func (system *fakeSystem) Output(_ context.Context, command string, args ...string) (string, error) {
	system.calls = append(system.calls, command+" "+strings.Join(args, " "))
	return system.output[command], system.runErr[command]
}

func TestCheckReportsClaudeAuthenticationFailure(t *testing.T) {
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
				Profiles:       map[string]project.ProfileConfig{"default": {Provider: "claude", Model: "sonnet", Effort: "medium"}},
			},
		},
	}
	system := &fakeSystem{
		output: map[string]string{"claude": `{"loggedIn":false}`},
	}

	report := doctor.Check(context.Background(), loaded, agent.DefaultRegistry(), system)

	if report.OK {
		t.Fatal("Check() report is OK, want claude authentication failure")
	}
	if !strings.Contains(report.String(), "claude is not authenticated") {
		t.Fatalf("report %q does not contain claude authentication guidance", report.String())
	}
}

func TestCheckReportsDirectProviderAuthenticationState(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		provider  string
		runErr    error
		wantOK    bool
		wantInLog string
	}{
		{name: "codex not authenticated", provider: "codex", runErr: errors.New("not logged in"), wantOK: false, wantInLog: "codex is not authenticated; run `codex login`"},
		{name: "codex authenticated", provider: "codex", wantOK: true, wantInLog: "authenticated"},
		{name: "kimi not authenticated", provider: "kimi", runErr: errors.New("not logged in"), wantOK: false, wantInLog: "kimi is not authenticated; run `kimi auth login`"},
		{name: "kimi authenticated", provider: "kimi", wantOK: true, wantInLog: "authenticated"},
		{name: "openclaw not authenticated", provider: "openclaw", runErr: errors.New("not logged in"), wantOK: false, wantInLog: "openclaw is not authenticated; run `openclaw auth login`"},
		{name: "openclaw authenticated", provider: "openclaw", wantOK: true, wantInLog: "authenticated"},
		{name: "hermes not authenticated", provider: "hermes", runErr: errors.New("not logged in"), wantOK: false, wantInLog: "hermes is not authenticated; run `hermes auth login`"},
		{name: "hermes authenticated", provider: "hermes", wantOK: true, wantInLog: "authenticated"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
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
						Profiles:       map[string]project.ProfileConfig{"default": {Provider: testCase.provider}},
					},
				},
			}
			system := &fakeSystem{}
			if testCase.runErr != nil {
				system.runErr = map[string]error{testCase.provider: testCase.runErr}
			}

			report := doctor.Check(context.Background(), loaded, agent.DefaultRegistry(), system)

			if report.OK != testCase.wantOK {
				t.Fatalf("Check() report.OK = %v, want %v; report: %s", report.OK, testCase.wantOK, report.String())
			}
			if !strings.Contains(report.String(), testCase.wantInLog) {
				t.Fatalf("report %q does not contain %q", report.String(), testCase.wantInLog)
			}
		})
	}
}

func TestCheckReportsUnavailableOpenCodeModel(t *testing.T) {
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
				Profiles:       map[string]project.ProfileConfig{"default": {Provider: "opencode", Model: "kimi-k2.6"}},
			},
		},
	}
	system := &fakeSystem{
		output: map[string]string{
			"opencode": "OpenAI oauth\n1 credentials\nopenai/gpt-5.4\nopenai/gpt-5.5\n",
		},
	}

	report := doctor.Check(context.Background(), loaded, agent.DefaultRegistry(), system)

	if report.OK {
		t.Fatal("Check() report is OK, want opencode model failure")
	}
	if !strings.Contains(report.String(), `opencode model "kimi-k2.6" is unavailable`) {
		t.Fatalf("report %q does not contain opencode model guidance", report.String())
	}
}
