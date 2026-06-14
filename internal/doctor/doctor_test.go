package doctor_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
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
	// outputs returns canned output keyed by the full "command arg1 arg2..."
	// invocation, taking precedence over output when present.
	outputs map[string]string
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
	call := strings.TrimSpace(command + " " + strings.Join(args, " "))
	system.calls = append(system.calls, call)
	if value, ok := system.outputs[call]; ok {
		return value, system.runErr[command]
	}
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

func allLabelsJSON(t *testing.T, present ...string) string {
	t.Helper()
	type entry struct {
		Name string `json:"name"`
	}
	names := doctor.RequiredLabels
	if len(present) > 0 {
		names = present
	}
	entries := make([]entry, len(names))
	for index, name := range names {
		entries[index] = entry{Name: name}
	}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return string(data)
}

func baseLoadedConfig(root string) project.LoadedConfig {
	return project.LoadedConfig{
		Path: filepath.Join(root, "heracles.yaml"),
		Config: project.Config{
			Version:      1,
			IssueTracker: project.IssueTrackerConfig{GitHub: "acme/backlog"},
			Repositories: []project.RepositoryConfig{{Name: "app", Path: ".", GitHub: "acme/app", BaseBranch: "main"}},
			Agents: project.AgentConfig{
				DefaultProfile: "default",
				Profiles:       map[string]project.ProfileConfig{"default": {Provider: "codex"}},
			},
		},
	}
}

func TestCheckReportsMissingTrackerLabelsAsFixableBlocker(t *testing.T) {
	t.Parallel()

	loaded := baseLoadedConfig(t.TempDir())
	system := &fakeSystem{outputs: map[string]string{
		"gh label list --repo acme/backlog --json name --limit 100": allLabelsJSON(t, "heracles:ready"),
	}}

	report := doctor.Check(context.Background(), loaded, agent.DefaultRegistry(), system)

	if report.OK {
		t.Fatal("Check() report is OK, want missing tracker labels to block")
	}
	if !strings.Contains(report.String(), "[failed] Tracker Labels") || !strings.Contains(report.String(), "heracles:blocked") {
		t.Errorf("report = %q, want failed Tracker Labels listing missing labels", report.String())
	}
	if !strings.Contains(report.String(), "heracles doctor --fix") {
		t.Errorf("report = %q, want a pointer to `heracles doctor --fix`", report.String())
	}
}

func TestCheckReportsTrackerLabelsOKWhenAllPresent(t *testing.T) {
	t.Parallel()

	loaded := baseLoadedConfig(t.TempDir())
	system := &fakeSystem{outputs: map[string]string{
		"gh label list --repo acme/backlog --json name --limit 100": allLabelsJSON(t),
	}}

	report := doctor.Check(context.Background(), loaded, agent.DefaultRegistry(), system)

	if !strings.Contains(report.String(), "[ok] Tracker Labels: all required labels exist") {
		t.Errorf("report = %q, want OK Tracker Labels", report.String())
	}
}

func TestCheckReportsMissingBaseBranchAsBlocker(t *testing.T) {
	t.Parallel()

	loaded := baseLoadedConfig(t.TempDir())
	system := &fakeSystem{
		outputs: map[string]string{"gh label list --repo acme/backlog --json name --limit 100": allLabelsJSON(t)},
		runErr:  map[string]error{"git": errors.New("not a valid ref")},
	}

	report := doctor.Check(context.Background(), loaded, agent.DefaultRegistry(), system)

	if report.OK {
		t.Fatal("Check() report is OK, want missing base branch to block")
	}
	if !strings.Contains(report.String(), "[failed] Target Repository app base branch: not a valid ref") {
		t.Errorf("report = %q, want failed base branch diagnostic", report.String())
	}
}

func TestCheckReportsMissingVerificationCommandAsBlocker(t *testing.T) {
	t.Parallel()

	loaded := baseLoadedConfig(t.TempDir())
	loaded.Config.Repositories[0].Verify = []string{"make test"}
	system := &fakeSystem{
		outputs: map[string]string{"gh label list --repo acme/backlog --json name --limit 100": allLabelsJSON(t)},
		missing: map[string]error{"make": errors.New("not installed")},
	}

	report := doctor.Check(context.Background(), loaded, agent.DefaultRegistry(), system)

	if report.OK {
		t.Fatal("Check() report is OK, want missing verification command to block")
	}
	if !strings.Contains(report.String(), `[failed] Repository app verification command "make"`) {
		t.Errorf("report = %q, want failed verification command diagnostic", report.String())
	}
}

func TestCheckReportsMissingVerificationEnvironmentVariableAsBlocker(t *testing.T) {
	t.Parallel()

	loaded := baseLoadedConfig(t.TempDir())
	loaded.Config.Repositories[0].VerifyEnv = []string{"HERACLES_DOCTOR_TEST_MISSING_VAR"}
	system := &fakeSystem{outputs: map[string]string{
		"gh label list --repo acme/backlog --json name --limit 100": allLabelsJSON(t),
	}}

	report := doctor.Check(context.Background(), loaded, agent.DefaultRegistry(), system)

	if report.OK {
		t.Fatal("Check() report is OK, want missing verification environment variable to block")
	}
	if !strings.Contains(report.String(), "[failed] Repository app verification environment") || !strings.Contains(report.String(), "HERACLES_DOCTOR_TEST_MISSING_VAR") {
		t.Errorf("report = %q, want failed verification environment diagnostic", report.String())
	}
}

func TestCheckWarnsWhenAutoMergeNotAllowedWithoutBlocking(t *testing.T) {
	t.Parallel()

	loaded := baseLoadedConfig(t.TempDir())
	loaded.Config.Delivery.AutoMerge = true
	system := &fakeSystem{outputs: map[string]string{
		"gh label list --repo acme/backlog --json name --limit 100": allLabelsJSON(t),
		"gh repo view acme/app --json autoMergeAllowed":             `{"autoMergeAllowed":false}`,
		"gh api repos/acme/app/actions/workflows --jq .total_count": "1",
	}}

	report := doctor.Check(context.Background(), loaded, agent.DefaultRegistry(), system)

	if !report.OK {
		t.Fatalf("Check() report is not OK, want auto-merge to only warn:\n%s", report.String())
	}
	if !strings.Contains(report.String(), "[warn] Repository app auto-merge") {
		t.Errorf("report = %q, want auto-merge warning", report.String())
	}
}

func TestCheckWarnsWhenNoCIWorkflowsConfiguredWithoutBlocking(t *testing.T) {
	t.Parallel()

	loaded := baseLoadedConfig(t.TempDir())
	system := &fakeSystem{outputs: map[string]string{
		"gh label list --repo acme/backlog --json name --limit 100": allLabelsJSON(t),
		"gh api repos/acme/app/actions/workflows --jq .total_count": "0",
	}}

	report := doctor.Check(context.Background(), loaded, agent.DefaultRegistry(), system)

	if !report.OK {
		t.Fatalf("Check() report is not OK, want missing CI to only warn:\n%s", report.String())
	}
	if !strings.Contains(report.String(), "[warn] Repository app CI: no GitHub Actions workflows configured") {
		t.Errorf("report = %q, want CI warning", report.String())
	}
}

func TestCheckWarnsAboutMissingWorkspaceRoot(t *testing.T) {
	t.Parallel()

	loaded := baseLoadedConfig(t.TempDir())
	system := &fakeSystem{outputs: map[string]string{
		"gh label list --repo acme/backlog --json name --limit 100": allLabelsJSON(t),
		"gh api repos/acme/app/actions/workflows --jq .total_count": "1",
	}}

	report := doctor.Check(context.Background(), loaded, agent.DefaultRegistry(), system)

	if !report.OK {
		t.Fatalf("Check() report is not OK, want missing workspace root to only warn:\n%s", report.String())
	}
	if !strings.Contains(report.String(), "[warn] Workspaces") || !strings.Contains(report.String(), "heracles doctor --fix") {
		t.Errorf("report = %q, want workspace root warning pointing to --fix", report.String())
	}
}

func TestFixProjectCreatesMissingWorkspaceRootAndTrackerLabels(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	loaded := baseLoadedConfig(root)
	system := &fakeSystem{outputs: map[string]string{
		"gh label list --repo acme/backlog --json name --limit 100": allLabelsJSON(t, "heracles:ready"),
		"gh api repos/acme/app/actions/workflows --jq .total_count": "1",
	}}

	report := doctor.FixProject(context.Background(), loaded, agent.DefaultRegistry(), system)

	if _, err := os.Stat(filepath.Join(root, ".heracles", "workspaces")); err != nil {
		t.Errorf("FixProject() did not create the workspace root: %v", err)
	}
	var created []string
	for _, call := range system.calls {
		if strings.HasPrefix(call, "gh label create ") {
			created = append(created, call)
		}
	}
	if len(created) != len(doctor.RequiredLabels)-1 {
		t.Errorf("FixProject() created %d labels, want %d:\n%v", len(created), len(doctor.RequiredLabels)-1, created)
	}
	if !strings.Contains(report.String(), "[ok] Workspaces") {
		t.Errorf("report = %q, want Workspaces repaired", report.String())
	}
}

func TestReportStringDistinguishesWarningsFromFailures(t *testing.T) {
	t.Parallel()

	report := doctor.Report{Checks: []doctor.Diagnostic{
		{Name: "a", OK: true, Message: "fine"},
		{Name: "b", Warning: true, Message: "be careful"},
		{Name: "c", Message: "broken"},
	}}

	for _, expected := range []string{"[ok] a: fine", "[warn] b: be careful", "[failed] c: broken"} {
		if !strings.Contains(report.String(), expected) {
			t.Errorf("report = %q, want %q", report.String(), expected)
		}
	}
}
