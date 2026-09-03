package version

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    int
	}{
		{name: "older", current: "v0.1.4", latest: "v0.1.5", want: -1},
		{name: "equal", current: "v0.1.5", latest: "v0.1.5", want: 0},
		{name: "newer", current: "v0.1.6", latest: "v0.1.5", want: 1},
		{name: "prerelease behind stable", current: "v1.1.0-beta.1", latest: "v1.1.0", want: -1},
		{name: "stable ahead of prerelease", current: "v1.1.0", latest: "v1.1.0-rc.1", want: 1},
		{name: "newer prerelease", current: "v1.1.0-beta.2", latest: "v1.1.0-beta.1", want: 1},
		{name: "prerelease ahead of older stable", current: "v1.1.0-beta.1", latest: "v1.0.0", want: 1},
		{name: "dev fallback", current: "dev", latest: "v0.1.5", want: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareVersions(tt.current, tt.latest); got != tt.want {
				t.Fatalf("compareVersions(%q, %q)=%d want %d", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestReleaseChannel(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{version: "v1.1.0", want: "stable"},
		{version: "v1.1.0-beta.1", want: "beta"},
		{version: "v1.1.0-rc.1", want: "rc"},
		{version: "v1.1.0-internal.1", want: "internal"},
		{version: "v1.1.0-preview.1", want: "prerelease"},
		{version: "dev", want: "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			if got := ReleaseChannel(tt.version); got != tt.want {
				t.Fatalf("ReleaseChannel(%q)=%q want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestCheckLatest(t *testing.T) {
	origVersion := Version
	t.Cleanup(func() {
		Version = origVersion
		_ = os.Unsetenv("LOOMLOOM_CLI_RELEASE_API")
		_ = os.Unsetenv("BATCHJOB_CLI_RELEASE_API")
	})
	Version = "v0.1.4"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.1.5"}`))
	}))
	defer server.Close()
	_ = os.Setenv("LOOMLOOM_CLI_RELEASE_API", server.URL)

	status, err := CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest() error = %v", err)
	}
	if status.CurrentVersion != "v0.1.4" {
		t.Fatalf("CurrentVersion=%q", status.CurrentVersion)
	}
	if status.LatestVersion != "v0.1.5" {
		t.Fatalf("LatestVersion=%q", status.LatestVersion)
	}
	if status.CurrentChannel != "stable" {
		t.Fatalf("CurrentChannel=%q", status.CurrentChannel)
	}
	if status.LatestChannel != "stable" {
		t.Fatalf("LatestChannel=%q", status.LatestChannel)
	}
	if !status.UpdateAvailable {
		t.Fatalf("UpdateAvailable=false, want true")
	}
	if status.UpgradeHint == "" {
		t.Fatalf("UpgradeHint should not be empty")
	}
}

func TestStableUpdateCacheRefreshesStableReleaseAndSkipsPrereleases(t *testing.T) {
	origVersion := Version
	origUserCacheDir := userCacheDir
	origLatestNPMURL := latestNPMURL
	t.Cleanup(func() {
		Version = origVersion
		userCacheDir = origUserCacheDir
		latestNPMURL = origLatestNPMURL
		_ = os.Unsetenv("LOOMLOOM_NO_UPDATE_CHECK")
	})
	cacheDir := t.TempDir()
	userCacheDir = func() (string, error) { return cacheDir, nil }

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"version":"1.2.4"}`))
	}))
	defer server.Close()
	latestNPMURL = server.URL

	Version = "v1.2.3"
	if notice := CachedStableUpdateNotice(); notice != "" {
		t.Fatalf("cold cache notice=%q want empty", notice)
	}
	RefreshStableUpdateCache(context.Background())
	if requests != 1 {
		t.Fatalf("requests=%d want 1", requests)
	}

	notice := CachedStableUpdateNotice()
	if !strings.Contains(notice, "1.2.4") {
		t.Fatalf("cached notice=%q want latest release", notice)
	}
	RefreshStableUpdateCache(context.Background())
	if requests != 1 {
		t.Fatalf("cached check requests=%d want 1", requests)
	}

	Version = "v1.2.3-beta.1"
	notice = CachedStableUpdateNotice()
	if notice != "" {
		t.Fatalf("prerelease notice=%q", notice)
	}
	RefreshStableUpdateCache(context.Background())
	if requests != 1 {
		t.Fatalf("prerelease check requests=%d want 1", requests)
	}
}

func TestUpdateCheckCacheFreshness(t *testing.T) {
	if !cacheFresh(updateCheckCache{CheckedAt: time.Now(), LatestVersion: "1.2.3", Source: "npm"}) {
		t.Fatal("fresh cache was not accepted")
	}
	if cacheFresh(updateCheckCache{CheckedAt: time.Now().Add(-updateCheckTTL - time.Second), LatestVersion: "1.2.3", Source: "npm"}) {
		t.Fatal("expired cache was accepted")
	}
	if cacheFresh(updateCheckCache{CheckedAt: time.Now(), LatestVersion: "v1.2.3", Source: "github"}) {
		t.Fatal("cache from another source was accepted")
	}
}
