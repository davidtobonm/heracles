package cli_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/buildinfo"
	"github.com/davidtobonm/heracles/internal/cli"
	"github.com/davidtobonm/heracles/internal/update"
)

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestInstallCopiesExecutableToDirectory(t *testing.T) {
	t.Parallel()

	sourcePath := filepath.Join(t.TempDir(), "heracles-build")
	if err := os.WriteFile(sourcePath, []byte("binary contents"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	targetDir := filepath.Join(t.TempDir(), "bin")
	var stdout, stderr bytes.Buffer
	exitCode := cli.RunWithOptions([]string{"install", "--dir", targetDir, "--json"}, &stdout, &stderr, cli.Options{Executable: sourcePath})
	if exitCode != 0 {
		t.Fatalf("RunWithOptions(install) exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}

	var result struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout = %q", err, stdout.String())
	}
	if filepath.Dir(result.Path) != targetDir {
		t.Errorf("path directory = %q, want %q", filepath.Dir(result.Path), targetDir)
	}
	installed, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(installed) != "binary contents" {
		t.Errorf("installed contents = %q, want %q", installed, "binary contents")
	}
}

func TestInstallHelpExitsSuccessfully(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	exitCode := cli.Run([]string{"install", "--help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("Run(install --help) exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "-system") {
		t.Errorf("help output = %q, want install flags", stderr.String())
	}
}

type fakeUpdateSource struct {
	release   update.Release
	downloads map[string][]byte
	calls     int
}

func (source *fakeUpdateSource) Latest(context.Context) (update.Release, error) {
	source.calls++
	return source.release, nil
}

func (source *fakeUpdateSource) Download(_ context.Context, asset update.Asset) ([]byte, error) {
	return source.downloads[asset.Name], nil
}

func TestUpdateCheckReportsAvailableUpdateAsJSON(t *testing.T) {
	t.Parallel()

	source := &fakeUpdateSource{release: update.Release{Version: "v9.9.9"}}
	cachePath := filepath.Join(t.TempDir(), "update.json")
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)

	var stdout, stderr bytes.Buffer
	exitCode := cli.RunWithOptions([]string{"update", "--check", "--json"}, &stdout, &stderr, cli.Options{
		UpdateSource:    source,
		UpdateCachePath: cachePath,
		Now:             func() time.Time { return now },
	})
	if exitCode != 0 {
		t.Fatalf("RunWithOptions(update --check) exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}

	var result struct {
		CurrentVersion  string `json:"current_version"`
		LatestVersion   string `json:"latest_version"`
		UpdateAvailable bool   `json:"update_available"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout = %q", err, stdout.String())
	}
	if result.CurrentVersion != buildinfo.Version() || result.LatestVersion != "v9.9.9" || !result.UpdateAvailable {
		t.Errorf("result = %#v", result)
	}
}

func TestUpdateDefaultIsSilentWhenUpToDate(t *testing.T) {
	t.Parallel()

	source := &fakeUpdateSource{release: update.Release{Version: buildinfo.Version()}}
	cachePath := filepath.Join(t.TempDir(), "update.json")
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)

	var stdout, stderr bytes.Buffer
	exitCode := cli.RunWithOptions([]string{"update"}, &stdout, &stderr, cli.Options{
		UpdateSource:    source,
		UpdateCachePath: cachePath,
		Now:             func() time.Time { return now },
	})
	if exitCode != 0 {
		t.Fatalf("RunWithOptions(update) exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if stdout.String() != "" || stderr.String() != "" {
		t.Errorf("stdout/stderr = %q / %q, want both empty", stdout.String(), stderr.String())
	}
}

func TestUpdateApplyDownloadsVerifiesAndReplacesExecutable(t *testing.T) {
	t.Parallel()

	newBinary := []byte("new heracles binary")
	assetName := update.AssetName(runtime.GOOS, runtime.GOARCH)
	checksums := []byte(sha256Hex(newBinary) + "  " + assetName + "\n")
	source := &fakeUpdateSource{
		release: update.Release{
			Version: "v9.9.9",
			Assets: []update.Asset{
				{Name: assetName, URL: "https://example.com/" + assetName},
				{Name: update.ChecksumsAsset, URL: "https://example.com/checksums.txt"},
			},
		},
		downloads: map[string][]byte{
			assetName:             newBinary,
			update.ChecksumsAsset: checksums,
		},
	}

	executablePath := filepath.Join(t.TempDir(), "heracles")
	if err := os.WriteFile(executablePath, []byte("old heracles binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cachePath := filepath.Join(t.TempDir(), "update.json")
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	var stdout, stderr bytes.Buffer
	exitCode := cli.RunWithOptions([]string{"update", "--apply", "--json"}, &stdout, &stderr, cli.Options{
		UpdateSource:    source,
		UpdateCachePath: cachePath,
		Executable:      executablePath,
		Now:             func() time.Time { return now },
	})
	if exitCode != 0 {
		t.Fatalf("RunWithOptions(update --apply) exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}

	var result struct {
		Applied bool `json:"applied"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout = %q", err, stdout.String())
	}
	if !result.Applied {
		t.Fatal("applied = false, want true")
	}
	installed, err := os.ReadFile(executablePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(installed) != string(newBinary) {
		t.Errorf("installed contents = %q, want %q", installed, newBinary)
	}
}

func TestUpdateHelpExitsSuccessfully(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	exitCode := cli.Run([]string{"update", "--help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("Run(update --help) exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "-apply") {
		t.Errorf("help output = %q, want update flags", stderr.String())
	}
}
