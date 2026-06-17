package setup_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/project"
	"github.com/davidtobonm/heracles/internal/setup"
)

func runGit(t testing.TB, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func newGitRepo(t *testing.T, withGoMod bool) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, "", "init", "--initial-branch=main", dir)
	runGit(t, dir, "remote", "add", "origin", "git@github.com:acme/widget.git")
	if withGoMod {
		writeFile(t, dir, "go.mod", "module example.com/widget\n\ngo 1.24\n")
	}
	return dir
}

func TestRunNewProjectFastSetup(t *testing.T) {
	t.Parallel()

	dir := newGitRepo(t, true)
	io, _ := newSetupIO(strings.Repeat("\n", 6))
	system := fakeDoctorSystem{installed: map[string]bool{"git": true, "gh": true, "codex": true, "gofmt": true, "go": true}}

	result, err := setup.Run(context.Background(), setup.Options{
		WorkingDirectory: dir,
		HomeDirectory:    t.TempDir(),
		IO:               io,
		Registry:         agent.DefaultRegistry(),
		System:           system,
		Publisher:        &fakePublisher{},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Cancelled {
		t.Fatalf("Run() cancelled, want completed")
	}
	canonicalDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolve repo path: %v", err)
	}
	if result.Path != filepath.Join(canonicalDir, "heracles.yaml") {
		t.Errorf("Path = %q, want heracles.yaml in repo root", result.Path)
	}
	if len(result.Config.Repositories) != 1 {
		t.Fatalf("repositories = %d, want 1", len(result.Config.Repositories))
	}
	want := []string{"gofmt -l .", "go vet ./...", "go test ./..."}
	if !equalSlices(result.Config.Repositories[0].Verify, want) {
		t.Errorf("Verify = %v, want %v", result.Config.Repositories[0].Verify, want)
	}
	if !result.Doctor.OK {
		t.Errorf("Doctor.OK = false, want true:\n%s", result.Doctor.String())
	}

	preferences, err := project.LoadPreferences(project.ProjectPreferencesPath(result.Path))
	if err != nil {
		t.Fatalf("LoadPreferences() error = %v", err)
	}
	for _, role := range []string{"planner", "issue_author", "implementer", "reviewer"} {
		if profile := preferences.Agents[role]; profile.Provider != "codex" {
			t.Errorf("preferences[%s].Provider = %q, want codex", role, profile.Provider)
		}
	}
}

func TestRunNewProjectCompleteSetup(t *testing.T) {
	t.Parallel()

	dir := newGitRepo(t, true)
	inputs := []string{
		"2",        // mode: Complete Setup
		"", "", "", // implementer: provider (codex), model, effort
		"n",         // sameForAll: no
		"2", "", "", // planner: provider (claude), model, effort
		"3", "", "", // issue_author: provider (opencode), model, variant
		"", "", "", // reviewer: provider (codex), model, effort
		"10", "2", "y", "", "", "", // labor policy
		"", "API_KEY,DB_URL", // verification confirm + env vars
	}
	io, _ := newSetupIO(strings.Join(inputs, "\n") + "\n")
	system := fakeDoctorSystem{installed: map[string]bool{"git": true, "gh": true, "codex": true, "claude": true, "opencode": true, "gofmt": true, "go": true}}

	result, err := setup.Run(context.Background(), setup.Options{
		WorkingDirectory: dir,
		HomeDirectory:    t.TempDir(),
		IO:               io,
		Registry:         agent.DefaultRegistry(),
		System:           system,
		Publisher:        &fakePublisher{},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Config.Planning.QuestionBudget != 10 {
		t.Errorf("QuestionBudget = %d, want 10", result.Config.Planning.QuestionBudget)
	}
	if result.Config.Labor.IssueConcurrency != 2 {
		t.Errorf("IssueConcurrency = %d, want 2", result.Config.Labor.IssueConcurrency)
	}
	if !result.Config.Delivery.AutoMerge {
		t.Errorf("AutoMerge = false, want true")
	}

	if want := []string{"API_KEY", "DB_URL"}; !equalSlices(result.Config.Repositories[0].VerifyEnv, want) {
		t.Errorf("VerifyEnv = %v, want %v", result.Config.Repositories[0].VerifyEnv, want)
	}

	preferences, err := project.LoadPreferences(project.ProjectPreferencesPath(result.Path))
	if err != nil {
		t.Fatalf("LoadPreferences() error = %v", err)
	}
	if got := preferences.Agents["planner"].Provider; got != "claude" {
		t.Errorf("preferences[planner].Provider = %q, want claude", got)
	}
	if got := preferences.Agents["issue_author"].Provider; got != "opencode" {
		t.Errorf("preferences[issue_author].Provider = %q, want opencode", got)
	}
	if got := preferences.Agents["implementer"].Provider; got != "codex" {
		t.Errorf("preferences[implementer].Provider = %q, want codex", got)
	}
	if got := preferences.Agents["reviewer"].Provider; got != "codex" {
		t.Errorf("preferences[reviewer].Provider = %q, want codex", got)
	}
}

func TestRunExistingProjectFastReconfigure(t *testing.T) {
	t.Parallel()

	dir := newGitRepo(t, true)
	if _, err := project.Initialize(context.Background(), project.InitOptions{WorkingDirectory: dir}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	io, _ := newSetupIO(strings.Repeat("\n", 6))
	system := fakeDoctorSystem{installed: map[string]bool{"git": true, "gh": true, "codex": true, "gofmt": true, "go": true}}

	result, err := setup.Run(context.Background(), setup.Options{
		WorkingDirectory: dir,
		HomeDirectory:    t.TempDir(),
		IO:               io,
		Registry:         agent.DefaultRegistry(),
		System:           system,
		Publisher:        &fakePublisher{},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Cancelled {
		t.Fatalf("Run() cancelled, want completed")
	}
	want := []string{"gofmt -l .", "go vet ./...", "go test ./..."}
	if !equalSlices(result.Config.Repositories[0].Verify, want) {
		t.Errorf("Verify = %v, want %v", result.Config.Repositories[0].Verify, want)
	}
}

func TestRunExistingProjectRepairPublishesBootstrap(t *testing.T) {
	t.Parallel()

	dir := newGitRepo(t, false)
	if _, err := project.Initialize(context.Background(), project.InitOptions{WorkingDirectory: dir}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	io, out := newSetupIO("3\nn\n")
	system := fakeDoctorSystem{installed: map[string]bool{"git": true, "gh": true, "codex": true, "gofmt": true, "go": true}}
	publisher := &fakePublisher{}
	var ranBootstrap bool

	result, err := setup.Run(context.Background(), setup.Options{
		WorkingDirectory: dir,
		HomeDirectory:    t.TempDir(),
		IO:               io,
		Registry:         agent.DefaultRegistry(),
		System:           system,
		Publisher:        publisher,
		RunBootstrapBacklog: func(context.Context, project.LoadedConfig) error {
			ranBootstrap = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Config.Repositories[0].Verify) != 0 {
		t.Errorf("Verify = %v, want empty for repair without detected commands", result.Config.Repositories[0].Verify)
	}
	if len(publisher.created) != 2 {
		t.Fatalf("created %d issues, want 2 (PRD + 1 proposal)", len(publisher.created))
	}
	if publisher.created[0].Title != "Heracles Project Bootstrap" {
		t.Errorf("first issue title = %q, want Heracles Project Bootstrap", publisher.created[0].Title)
	}
	if ranBootstrap {
		t.Errorf("RunBootstrapBacklog called, want skipped after declining")
	}
	if !strings.Contains(out.String(), "Published Heracles Project Bootstrap") {
		t.Errorf("output = %q, want bootstrap publication notice", out.String())
	}
}

func TestRunExistingProjectRepairPublishesBootstrapWithDetectedStack(t *testing.T) {
	t.Parallel()

	// pubspec.yaml identifies a Dart/Flutter stack for the Bootstrap proposal
	// text, but isn't recognized by DetectVerification, so the repository
	// stays under-verified and reaches the bootstrap path (unlike go.mod,
	// which DetectVerification always resolves to confident commands for).
	dir := newGitRepo(t, false)
	writeFile(t, dir, "pubspec.yaml", "name: daily_water\n")
	if _, err := project.Initialize(context.Background(), project.InitOptions{WorkingDirectory: dir}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	io, _ := newSetupIO("3\nn\n")
	system := fakeDoctorSystem{installed: map[string]bool{"git": true, "gh": true, "codex": true, "gofmt": true, "go": true}}
	publisher := &fakePublisher{}

	if _, err := setup.Run(context.Background(), setup.Options{
		WorkingDirectory: dir,
		HomeDirectory:    t.TempDir(),
		IO:               io,
		Registry:         agent.DefaultRegistry(),
		System:           system,
		Publisher:        publisher,
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(publisher.created) != 2 {
		t.Fatalf("created %d issues, want 2 (PRD + 1 proposal)", len(publisher.created))
	}
	if !strings.Contains(publisher.created[1].Body, "Dart/Flutter") {
		t.Errorf("proposal body = %q, want it to mention the detected Dart/Flutter stack", publisher.created[1].Body)
	}
}

func TestRunExistingProjectRepairSkipsBootstrapForRepositoryWithoutFiles(t *testing.T) {
	t.Parallel()

	dir := newGitRepo(t, false)
	result, err := project.Initialize(context.Background(), project.InitOptions{WorkingDirectory: dir})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	loaded, err := project.Load(result.Path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	emptyRepoDir := filepath.Join(dir, "empty-service")
	if err := os.MkdirAll(emptyRepoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	// Mark the root repository already verified so it's excluded from the
	// repair's verification-detection and bootstrap loops, leaving only the
	// empty repository below under test.
	loaded.Config.Repositories[0].Verify = []string{"true"}
	loaded.Config.Repositories = append(loaded.Config.Repositories, project.RepositoryConfig{
		Name: "empty-service", Path: "empty-service", GitHub: "acme/empty-service", BaseBranch: "main",
	})
	if err := project.WriteConfig(loaded.Path, loaded.Config); err != nil {
		t.Fatalf("WriteConfig() error = %v", err)
	}

	io, out := newSetupIO("3\n")
	system := fakeDoctorSystem{installed: map[string]bool{"git": true, "gh": true, "codex": true, "gofmt": true, "go": true}}
	publisher := &fakePublisher{}

	if _, err := setup.Run(context.Background(), setup.Options{
		WorkingDirectory: dir,
		HomeDirectory:    t.TempDir(),
		IO:               io,
		Registry:         agent.DefaultRegistry(),
		System:           system,
		Publisher:        publisher,
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(publisher.created) != 0 {
		t.Errorf("created %d issues, want 0 since the only under-verified repository has no files", len(publisher.created))
	}
	if !strings.Contains(out.String(), "empty-service") || !strings.Contains(out.String(), "no files yet") {
		t.Errorf("output = %q, want a message explaining the bootstrap was skipped for empty-service", out.String())
	}
}

func TestRunExistingProjectRepairRunsBootstrapBacklogWhenAccepted(t *testing.T) {
	t.Parallel()

	dir := newGitRepo(t, false)
	if _, err := project.Initialize(context.Background(), project.InitOptions{WorkingDirectory: dir}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	io, _ := newSetupIO("3\ny\n")
	system := fakeDoctorSystem{installed: map[string]bool{"git": true, "gh": true, "codex": true, "gofmt": true, "go": true}}
	var ranBootstrap bool

	_, err := setup.Run(context.Background(), setup.Options{
		WorkingDirectory: dir,
		HomeDirectory:    t.TempDir(),
		IO:               io,
		Registry:         agent.DefaultRegistry(),
		System:           system,
		Publisher:        &fakePublisher{},
		RunBootstrapBacklog: func(context.Context, project.LoadedConfig) error {
			ranBootstrap = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !ranBootstrap {
		t.Errorf("RunBootstrapBacklog not called, want called after accepting")
	}
}

func TestRunExistingProjectCancelMakesNoChanges(t *testing.T) {
	t.Parallel()

	dir := newGitRepo(t, true)
	if _, err := project.Initialize(context.Background(), project.InitOptions{WorkingDirectory: dir}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	configPath := filepath.Join(dir, "heracles.yaml")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read heracles.yaml: %v", err)
	}

	io, _ := newSetupIO("4\n")
	system := fakeDoctorSystem{installed: map[string]bool{"git": true, "gh": true, "codex": true, "gofmt": true, "go": true}}

	result, err := setup.Run(context.Background(), setup.Options{
		WorkingDirectory: dir,
		HomeDirectory:    t.TempDir(),
		IO:               io,
		Registry:         agent.DefaultRegistry(),
		System:           system,
		Publisher:        &fakePublisher{},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Cancelled {
		t.Fatalf("Run() cancelled = false, want true")
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read heracles.yaml: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("heracles.yaml changed after Cancel")
	}
	if _, err := os.Stat(project.ProjectPreferencesPath(configPath)); !os.IsNotExist(err) {
		t.Errorf("preferences.yaml created after Cancel")
	}
}

func equalSlices(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
