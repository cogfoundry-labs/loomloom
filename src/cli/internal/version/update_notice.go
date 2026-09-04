package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const updateCheckTTL = 24 * time.Hour
const updateRegistryURL = "https://registry.npmjs.org/@cogfoundry%2floomloom/latest"

var userCacheDir = os.UserCacheDir
var latestNPMURL = updateRegistryURL

type updateCheckCache struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
	Source        string    `json:"source"`
}

type skillSyncReceipt struct {
	Version int `json:"version"`
	Agents map[string]struct {
		Package string `json:"package"`
		PackageVersion string `json:"package_version"`
	} `json:"agents"`
}

// CachedStableUpdateNotice reads only local state, so it is safe on every
// command path. The cache is a rate limiter, not a release source of truth.
func CachedStableUpdateNotice() string {
	if updateCheckDisabled() || ReleaseChannel(Version) != "stable" {
		return ""
	}

	cache, err := loadUpdateCheckCache()
	if err != nil {
		return ""
	}
	return updateNotice(Version, cache.LatestVersion)
}

// CachedSkillSyncNotice only reads the npm installer receipt. It never scans
// Agent directories or downloads Skill content during normal command use.
func CachedSkillSyncNotice() string {
	if updateCheckDisabled() || ReleaseChannel(Version) != "stable" {
		return ""
	}
	dir, err := os.UserConfigDir()
	if err != nil { return "" }
	data, err := os.ReadFile(filepath.Join(dir, "loomloom", "skill-sync.json"))
	if err != nil { return "" }
	var receipt skillSyncReceipt
	if json.Unmarshal(data, &receipt) != nil { return "" }
	for agent, entry := range receipt.Agents {
		if entry.Package != "@cogfoundry/loomloom" || compareVersions(entry.PackageVersion, Version) >= 0 { continue }
		return fmt.Sprintf("LoomLoom Skill for %s is older than CLI %s. Run: loomloom update --agent %s", agent, Version, agent)
	}
	return ""
}

// RefreshStableUpdateCache refreshes stale stable-release state. Call it in a
// goroutine: command execution must never wait for an update check.
func RefreshStableUpdateCache(ctx context.Context) {
	if updateCheckDisabled() || ReleaseChannel(Version) != "stable" {
		return
	}
	cache, err := loadUpdateCheckCache()
	if err == nil && cacheFresh(cache) {
		return
	}

	latest, err := fetchLatestNPMVersion(ctx)
	if err != nil {
		return
	}
	_ = saveUpdateCheckCache(updateCheckCache{
		CheckedAt:     time.Now().UTC(),
		LatestVersion: latest,
		Source:        "npm",
	})
}

func updateNotice(current, latest string) string {
	if compareVersions(current, latest) != -1 {
		return ""
	}
	return fmt.Sprintf(
		"new LoomLoom stable release available: %s (current: %s). Upgrade with your package manager; npm users can run: npm install -g @cogfoundry/loomloom@latest",
		latest,
		current,
	)
}

func updateCheckDisabled() bool {
	value := strings.TrimSpace(os.Getenv("LOOMLOOM_NO_UPDATE_CHECK"))
	return value == "1" || strings.EqualFold(value, "true")
}

func cacheFresh(cache updateCheckCache) bool {
	return cache.Source == "npm" && cache.LatestVersion != "" && !cache.CheckedAt.IsZero() && time.Since(cache.CheckedAt) < updateCheckTTL
}

func fetchLatestNPMVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestNPMURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "loomloom/"+strings.TrimSpace(Version))
	response, err := (&http.Client{Timeout: defaultHTTPTimeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("npm registry: unexpected status %d", response.StatusCode)
	}
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", err
	}
	if !parseVersionForUpdate(payload.Version) {
		return "", fmt.Errorf("npm registry: invalid stable version %q", payload.Version)
	}
	return payload.Version, nil
}

func parseVersionForUpdate(raw string) bool {
	parsed, ok := parseSemver(raw)
	return ok && parsed.prerelease == ""
}

func updateCheckCachePath() (string, error) {
	dir, err := userCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "loomloom", "update-check.json"), nil
}

func loadUpdateCheckCache() (updateCheckCache, error) {
	path, err := updateCheckCachePath()
	if err != nil {
		return updateCheckCache{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return updateCheckCache{}, err
	}
	var cache updateCheckCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return updateCheckCache{}, err
	}
	return cache, nil
}

func saveUpdateCheckCache(cache updateCheckCache) error {
	path, err := updateCheckCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
