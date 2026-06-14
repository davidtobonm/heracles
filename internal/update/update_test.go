package update_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/update"
)

func TestAssetName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		goos, goarch, want string
	}{
		{"linux", "amd64", "heracles-linux-amd64"},
		{"darwin", "arm64", "heracles-darwin-arm64"},
		{"windows", "amd64", "heracles-windows-amd64.exe"},
	}
	for _, testCase := range cases {
		if got := update.AssetName(testCase.goos, testCase.goarch); got != testCase.want {
			t.Errorf("AssetName(%s, %s) = %q, want %q", testCase.goos, testCase.goarch, got, testCase.want)
		}
	}
}

func TestParseChecksums(t *testing.T) {
	t.Parallel()

	data := []byte("aaaa  heracles-linux-amd64\nbbbb  heracles-darwin-arm64\ncccc *heracles-windows-amd64.exe\n")
	checksums, err := update.ParseChecksums(data)
	if err != nil {
		t.Fatalf("ParseChecksums() error = %v", err)
	}
	if checksums["heracles-linux-amd64"] != "aaaa" || checksums["heracles-windows-amd64.exe"] != "cccc" {
		t.Errorf("ParseChecksums() = %#v", checksums)
	}
}

func TestVerifyChecksum(t *testing.T) {
	t.Parallel()

	data := []byte("release contents")
	sum := sha256.Sum256(data)
	hexSum := hex.EncodeToString(sum[:])
	if err := update.VerifyChecksum(data, hexSum); err != nil {
		t.Errorf("VerifyChecksum() error = %v", err)
	}
	if err := update.VerifyChecksum(data, stringsRepeat("0", 64)); err == nil {
		t.Error("VerifyChecksum() error = nil, want mismatch error")
	}
}

func TestApplyAtomicallyReplacesExecutable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "heracles")
	if err := os.WriteFile(path, []byte("old binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := update.Apply(path, []byte("new binary")); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(contents) != "new binary" {
		t.Errorf("contents = %q, want %q", contents, "new binary")
	}
}

type fakeSource struct {
	release      update.Release
	latestErr    error
	downloads    map[string][]byte
	downloadErrs map[string]error
	latestCalls  int
}

func (source *fakeSource) Latest(context.Context) (update.Release, error) {
	source.latestCalls++
	if source.latestErr != nil {
		return update.Release{}, source.latestErr
	}
	return source.release, nil
}

func (source *fakeSource) Download(_ context.Context, asset update.Asset) ([]byte, error) {
	if err, ok := source.downloadErrs[asset.Name]; ok {
		return nil, err
	}
	if data, ok := source.downloads[asset.Name]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("no fake download for %s", asset.Name)
}

func TestDownloadVerifiedSucceeds(t *testing.T) {
	t.Parallel()

	binary := []byte("verified binary")
	sum := sha256.Sum256(binary)
	checksums := []byte(fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), update.AssetName("linux", "amd64")))
	source := &fakeSource{
		release: update.Release{
			Version: "v1.2.3",
			Assets: []update.Asset{
				{Name: update.AssetName("linux", "amd64"), URL: "https://example.com/heracles-linux-amd64"},
				{Name: update.ChecksumsAsset, URL: "https://example.com/checksums.txt"},
			},
		},
		downloads: map[string][]byte{
			update.AssetName("linux", "amd64"): binary,
			update.ChecksumsAsset:              checksums,
		},
	}
	got, err := update.DownloadVerified(context.Background(), source, source.release, "linux", "amd64")
	if err != nil {
		t.Fatalf("DownloadVerified() error = %v", err)
	}
	if string(got) != string(binary) {
		t.Errorf("DownloadVerified() = %q, want %q", got, binary)
	}
}

func TestDownloadVerifiedRejectsChecksumMismatch(t *testing.T) {
	t.Parallel()

	binary := []byte("tampered binary")
	checksums := []byte(fmt.Sprintf("%s  %s\n", stringsRepeat("0", 64), update.AssetName("linux", "amd64")))
	source := &fakeSource{
		release: update.Release{
			Version: "v1.2.3",
			Assets: []update.Asset{
				{Name: update.AssetName("linux", "amd64"), URL: "https://example.com/heracles-linux-amd64"},
				{Name: update.ChecksumsAsset, URL: "https://example.com/checksums.txt"},
			},
		},
		downloads: map[string][]byte{
			update.AssetName("linux", "amd64"): binary,
			update.ChecksumsAsset:              checksums,
		},
	}
	if _, err := update.DownloadVerified(context.Background(), source, source.release, "linux", "amd64"); err == nil {
		t.Fatal("DownloadVerified() error = nil, want checksum mismatch error")
	}
}

func TestCheckUsesFreshCacheWithoutFetching(t *testing.T) {
	t.Parallel()

	cachePath := filepath.Join(t.TempDir(), "update.json")
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	if err := update.SaveCache(cachePath, update.CacheEntry{CheckedAt: now.Add(-time.Hour), LatestVersion: "v1.2.3"}); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}
	source := &fakeSource{release: update.Release{Version: "v9.9.9"}}
	result, err := update.Check(context.Background(), source, cachePath, "v1.2.3", now, 24*time.Hour, false)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if source.latestCalls != 0 || result.UpdateAvailable || result.LatestVersion != "v1.2.3" {
		t.Errorf("Check() = %#v, latestCalls=%d", result, source.latestCalls)
	}
}

func TestCheckRefreshesStaleCacheAndDetectsUpdate(t *testing.T) {
	t.Parallel()

	cachePath := filepath.Join(t.TempDir(), "update.json")
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	if err := update.SaveCache(cachePath, update.CacheEntry{CheckedAt: now.Add(-48 * time.Hour), LatestVersion: "v1.2.3"}); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}
	source := &fakeSource{release: update.Release{Version: "v1.3.0"}}
	result, err := update.Check(context.Background(), source, cachePath, "v1.2.3", now, 24*time.Hour, false)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if source.latestCalls != 1 || !result.UpdateAvailable || result.LatestVersion != "v1.3.0" {
		t.Errorf("Check() = %#v, latestCalls=%d", result, source.latestCalls)
	}
}

func stringsRepeat(value string, count int) string {
	result := ""
	for index := 0; index < count; index++ {
		result += value
	}
	return result
}
