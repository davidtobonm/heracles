// Package update checks for, verifies, and applies Heracles self-updates from GitHub Releases.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Asset describes one downloadable release artifact.
type Asset struct {
	Name string
	URL  string
	Size int64
}

// Release describes one GitHub release.
type Release struct {
	Version string
	Assets  []Asset
}

// Asset returns the named asset, if present.
func (release Release) Asset(name string) (Asset, bool) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return Asset{}, false
}

// ChecksumsAsset is the name of the published checksums file.
const ChecksumsAsset = "checksums.txt"

// AssetName returns the expected release binary name for goos/goarch, matching release automation.
func AssetName(goos, goarch string) string {
	if goos == "windows" {
		return fmt.Sprintf("heracles-%s-%s.exe", goos, goarch)
	}
	return fmt.Sprintf("heracles-%s-%s", goos, goarch)
}

// ParseChecksums parses sha256sum-format output into a name to lowercase hex digest map.
func ParseChecksums(data []byte) (map[string]string, error) {
	checksums := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid checksum line %q", line)
		}
		name := strings.TrimPrefix(fields[1], "*")
		checksums[name] = strings.ToLower(fields[0])
	}
	return checksums, nil
}

// VerifyChecksum returns an error if data's SHA-256 digest does not match expectedHex.
func VerifyChecksum(data []byte, expectedHex string) error {
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if !strings.EqualFold(actual, expectedHex) {
		return fmt.Errorf("checksum mismatch: got %s, want %s", actual, expectedHex)
	}
	return nil
}

// Apply atomically replaces the file at path with newBinary, preserving its permissions.
func Apply(path string, newBinary []byte) error {
	mode := os.FileMode(0o755)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode()
	}

	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "heracles-update-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if _, err := temp.Write(newBinary); err != nil {
		temp.Close()
		return fmt.Errorf("write update: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close update: %w", err)
	}
	if err := os.Chmod(tempPath, mode); err != nil {
		return fmt.Errorf("set permissions: %w", err)
	}

	backupPath := path + ".old"
	_ = os.Remove(backupPath)
	if err := os.Rename(path, backupPath); err != nil {
		return fmt.Errorf("back up current executable: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Rename(backupPath, path)
		return fmt.Errorf("install update: %w", err)
	}
	_ = os.Remove(backupPath)
	return nil
}

// Source retrieves release metadata and assets.
type Source interface {
	Latest(ctx context.Context) (Release, error)
	Download(ctx context.Context, asset Asset) ([]byte, error)
}

// GitHubSource retrieves releases from the GitHub REST API.
type GitHubSource struct {
	Owner  string
	Repo   string
	Client *http.Client
}

func (source GitHubSource) client() *http.Client {
	if source.Client != nil {
		return source.Client
	}
	return http.DefaultClient
}

// Latest fetches the latest published release.
func (source GitHubSource) Latest(ctx context.Context) (Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", source.Owner, source.Repo)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")

	response, err := source.client().Do(request)
	if err != nil {
		return Release{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("github releases request failed: %s", response.Status)
	}

	var payload struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return Release{}, fmt.Errorf("decode release: %w", err)
	}

	release := Release{Version: payload.TagName}
	for _, asset := range payload.Assets {
		release.Assets = append(release.Assets, Asset{Name: asset.Name, URL: asset.BrowserDownloadURL, Size: asset.Size})
	}
	return release, nil
}

// Download retrieves an asset's contents.
func (source GitHubSource) Download(ctx context.Context, asset Asset) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return nil, err
	}
	response, err := source.client().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s failed: %s", asset.Name, response.Status)
	}
	return io.ReadAll(response.Body)
}

// DownloadVerified downloads the release binary for goos/goarch and verifies it against checksums.
func DownloadVerified(ctx context.Context, source Source, release Release, goos, goarch string) ([]byte, error) {
	assetName := AssetName(goos, goarch)
	asset, ok := release.Asset(assetName)
	if !ok {
		return nil, fmt.Errorf("release %s has no asset %s", release.Version, assetName)
	}
	checksumsAsset, ok := release.Asset(ChecksumsAsset)
	if !ok {
		return nil, fmt.Errorf("release %s has no %s", release.Version, ChecksumsAsset)
	}

	binary, err := source.Download(ctx, asset)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", assetName, err)
	}
	checksumData, err := source.Download(ctx, checksumsAsset)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", ChecksumsAsset, err)
	}
	checksums, err := ParseChecksums(checksumData)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", ChecksumsAsset, err)
	}
	expected, ok := checksums[assetName]
	if !ok {
		return nil, fmt.Errorf("%s has no checksum for %s", ChecksumsAsset, assetName)
	}
	if err := VerifyChecksum(binary, expected); err != nil {
		return nil, fmt.Errorf("%s: %w", assetName, err)
	}
	return binary, nil
}

// CacheEntry is a persisted record of the last update check.
type CacheEntry struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

// Stale reports whether the entry is older than interval relative to now, or empty.
func (entry CacheEntry) Stale(now time.Time, interval time.Duration) bool {
	return entry.CheckedAt.IsZero() || now.Sub(entry.CheckedAt) >= interval
}

// LoadCache reads a cache entry from path. A missing file returns a zero entry without error.
func LoadCache(path string) (CacheEntry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return CacheEntry{}, nil
	}
	if err != nil {
		return CacheEntry{}, err
	}
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return CacheEntry{}, err
	}
	return entry, nil
}

// SaveCache writes a cache entry to path, creating parent directories as needed.
func SaveCache(path string, entry CacheEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// DefaultInterval is how often a cached update check is refreshed.
const DefaultInterval = 24 * time.Hour

// CheckResult reports update availability for the running binary.
type CheckResult struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	Checked         bool   `json:"checked"`
}

// Check returns cached or freshly fetched update status.
func Check(ctx context.Context, source Source, cachePath string, currentVersion string, now time.Time, interval time.Duration, force bool) (CheckResult, error) {
	entry, err := LoadCache(cachePath)
	if err != nil {
		return CheckResult{}, err
	}

	if !force && !entry.Stale(now, interval) {
		return CheckResult{
			CurrentVersion:  currentVersion,
			LatestVersion:   entry.LatestVersion,
			UpdateAvailable: entry.LatestVersion != "" && entry.LatestVersion != currentVersion,
		}, nil
	}

	release, err := source.Latest(ctx)
	if err != nil {
		return CheckResult{}, err
	}
	if err := SaveCache(cachePath, CacheEntry{CheckedAt: now, LatestVersion: release.Version}); err != nil {
		return CheckResult{}, err
	}
	return CheckResult{
		CurrentVersion:  currentVersion,
		LatestVersion:   release.Version,
		UpdateAvailable: release.Version != "" && release.Version != currentVersion,
		Checked:         true,
	}, nil
}
