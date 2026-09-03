package version

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const updateCheckTTL = 24 * time.Hour

var userCacheDir = os.UserCacheDir

type updateCheckCache struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

// StableUpdateNotice returns a non-empty human-readable notice only when the
// running CLI is a stable release and a newer stable release is known. The
// cache is a rate limiter, not a release source of truth.
func StableUpdateNotice(ctx context.Context) (string, error) {
	if updateCheckDisabled() || ReleaseChannel(Version) != "stable" {
		return "", nil
	}

	cache, cacheErr := loadUpdateCheckCache()
	if cacheErr == nil && cacheFresh(cache) {
		return updateNotice(Version, cache.LatestVersion), nil
	}

	status, err := CheckLatest(ctx)
	if err != nil {
		return "", err
	}
	_ = saveUpdateCheckCache(updateCheckCache{
		CheckedAt:     time.Now().UTC(),
		LatestVersion: status.LatestVersion,
	})
	return updateNotice(status.CurrentVersion, status.LatestVersion), nil
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
	return cache.LatestVersion != "" && !cache.CheckedAt.IsZero() && time.Since(cache.CheckedAt) < updateCheckTTL
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
