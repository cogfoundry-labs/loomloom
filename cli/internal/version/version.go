package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// Version is injected at release build time. Local builds keep the default.
	Version = "dev"
)

const (
	repoOwnerRepo      = "Cogfoundry-ai/loomloom"
	defaultReleaseAPI  = "https://api.github.com/repos/" + repoOwnerRepo + "/releases/latest"
	defaultHTTPTimeout = 5 * time.Second
)

type Status struct {
	CurrentVersion  string
	LatestVersion   string
	CurrentChannel  string
	LatestChannel   string
	UpdateAvailable bool
	UpgradeHint     string
}

type latestReleaseResponse struct {
	TagName string `json:"tag_name"`
}

type semverVersion struct {
	core       [3]int
	prerelease string
}

func CheckLatest(ctx context.Context) (*Status, error) {
	status := &Status{CurrentVersion: strings.TrimSpace(Version)}
	if status.CurrentVersion == "" {
		status.CurrentVersion = "dev"
	}
	status.CurrentChannel = ReleaseChannel(status.CurrentVersion)

	latest, err := resolveLatestVersion(ctx)
	if err != nil {
		return status, err
	}
	status.LatestVersion = latest
	status.LatestChannel = ReleaseChannel(latest)

	switch compareVersions(status.CurrentVersion, latest) {
	case -1:
		status.UpdateAvailable = true
		status.UpgradeHint = fmt.Sprintf(
			"new CLI release available: %s (current: %s). Run the installer again to update CLI and skill together.",
			latest, status.CurrentVersion,
		)
	default:
		status.UpdateAvailable = false
	}

	return status, nil
}

func ReleaseChannel(raw string) string {
	parsed, ok := parseSemver(raw)
	if !ok {
		return "dev"
	}
	if parsed.prerelease == "" {
		return "stable"
	}
	channel := strings.Split(parsed.prerelease, ".")[0]
	switch channel {
	case "beta", "rc", "internal":
		return channel
	default:
		return "prerelease"
	}
}

// resolveLatestVersion finds the latest stable release tag. It prefers the git
// protocol (git ls-remote), which is not subject to the GitHub REST API rate
// limit, and falls back to the REST API when git is unavailable. An explicit
// release-API override env var forces the REST API path.
func resolveLatestVersion(ctx context.Context) (string, error) {
	if releaseAPIOverride() != "" {
		return fetchLatestVersion(ctx, releaseAPIURL())
	}
	if tag, err := fetchLatestVersionViaGit(ctx); err == nil && tag != "" {
		return tag, nil
	}
	return fetchLatestVersion(ctx, releaseAPIURL())
}

// fetchLatestVersionViaGit lists tags via `git ls-remote` and returns the
// highest stable version tag (vX.Y.Z, no prerelease suffix).
func fetchLatestVersionViaGit(ctx context.Context) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git not available: %w", err)
	}
	// Cap the git lookup to the same budget as the HTTP path so a slow network
	// never makes this non-critical version check hang.
	gitCtx, cancel := context.WithTimeout(ctx, defaultHTTPTimeout)
	defer cancel()
	cmd := exec.CommandContext(gitCtx, "git", "ls-remote", "--tags", "--refs",
		"https://github.com/"+repoOwnerRepo+".git")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git ls-remote: %w", err)
	}

	best := ""
	for _, line := range strings.Split(string(out), "\n") {
		idx := strings.Index(line, "refs/tags/")
		if idx < 0 {
			continue
		}
		tag := strings.TrimSpace(line[idx+len("refs/tags/"):])
		parsed, ok := parseSemver(tag)
		if !ok || parsed.prerelease != "" {
			continue // stable releases only
		}
		if best == "" || compareVersions(tag, best) == 1 {
			best = tag
		}
	}
	if best == "" {
		return "", fmt.Errorf("no stable release tag found via git")
	}
	return best, nil
}

func releaseAPIOverride() string {
	if v := strings.TrimSpace(os.Getenv("LOOMLOOM_CLI_RELEASE_API")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("BATCHJOB_CLI_RELEASE_API"))
}

func releaseAPIURL() string {
	if v := releaseAPIOverride(); v != "" {
		return v
	}
	return defaultReleaseAPI
}

func fetchLatestVersion(ctx context.Context, apiURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "loomloom/"+strings.TrimSpace(Version))

	httpClient := &http.Client{Timeout: defaultHTTPTimeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("check latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("check latest release: unexpected status %d", resp.StatusCode)
	}

	var payload latestReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode latest release response: %w", err)
	}
	tag := strings.TrimSpace(payload.TagName)
	if tag == "" {
		return "", fmt.Errorf("latest release response missing tag_name")
	}
	return tag, nil
}

func compareVersions(current, latest string) int {
	cur, okCur := parseSemver(current)
	lat, okLat := parseSemver(latest)
	if !okCur || !okLat {
		return strings.Compare(current, latest)
	}
	for i := 0; i < len(cur.core); i++ {
		if cur.core[i] < lat.core[i] {
			return -1
		}
		if cur.core[i] > lat.core[i] {
			return 1
		}
	}

	switch {
	case cur.prerelease == "" && lat.prerelease != "":
		return 1
	case cur.prerelease != "" && lat.prerelease == "":
		return -1
	case cur.prerelease != "" && lat.prerelease != "":
		return comparePrerelease(cur.prerelease, lat.prerelease)
	default:
		return 0
	}
}

var semverPattern = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)\.([0-9]+)(?:-([0-9A-Za-z.-]+))?$`)

func parseSemver(raw string) (semverVersion, bool) {
	raw = strings.TrimSpace(raw)
	matches := semverPattern.FindStringSubmatch(raw)
	if matches == nil {
		return semverVersion{}, false
	}
	var out semverVersion
	for i := 0; i < 3; i++ {
		num, err := strconv.Atoi(matches[i+1])
		if err != nil {
			return semverVersion{}, false
		}
		out.core[i] = num
	}
	out.prerelease = matches[4]
	return out, true
}

func comparePrerelease(current, latest string) int {
	curParts := strings.Split(current, ".")
	latParts := strings.Split(latest, ".")
	for i := 0; i < len(curParts) && i < len(latParts); i++ {
		cmp := comparePrereleaseIdentifier(curParts[i], latParts[i])
		if cmp != 0 {
			return cmp
		}
	}
	if len(curParts) < len(latParts) {
		return -1
	}
	if len(curParts) > len(latParts) {
		return 1
	}
	return 0
}

func comparePrereleaseIdentifier(current, latest string) int {
	curNum, curIsNum := parseNumericIdentifier(current)
	latNum, latIsNum := parseNumericIdentifier(latest)
	switch {
	case curIsNum && latIsNum:
		if curNum < latNum {
			return -1
		}
		if curNum > latNum {
			return 1
		}
		return 0
	case curIsNum:
		return -1
	case latIsNum:
		return 1
	default:
		return strings.Compare(current, latest)
	}
}

func parseNumericIdentifier(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	num, err := strconv.Atoi(raw)
	return num, err == nil
}
