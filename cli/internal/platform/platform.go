package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

type ID string

const (
	ShengSuanYun ID = "shengsuanyun"
	CogFoundry   ID = "cogfoundry"
	Custom       ID = "custom"
	Unknown      ID = "unknown"
)

type Platform struct {
	ID            ID
	DisplayName   string
	HostSuffix    string
	Operational   bool
	KeysURL       string
	RechargeURL   string
	AuthPageURL   string
	AccountAPIURL string
	DefaultServer string
}

var registry = []Platform{
	{
		ID:            ShengSuanYun,
		DisplayName:   "胜算云",
		HostSuffix:    "shengsuanyun.com",
		Operational:   true,
		KeysURL:       "https://console.shengsuanyun.com/user/keys",
		RechargeURL:   "https://console.shengsuanyun.com/user/recharge",
		AuthPageURL:   "https://www.shengsuanyun.com/auth",
		AccountAPIURL: "https://api.shengsuanyun.com",
		DefaultServer: "https://loomloom.shengsuanyun.com/loom/v1",
	},
	{
		ID:          CogFoundry,
		DisplayName: "CogFoundry",
		HostSuffix:  "cogfoundry.ai",
		Operational: true,
	},
}

func All() []Platform {
	out := make([]Platform, len(registry))
	copy(out, registry)
	return out
}

func ByID(id ID) (Platform, bool) {
	for _, item := range registry {
		if item.ID == id {
			return item, true
		}
	}
	switch id {
	case "", Unknown:
		return UnknownPlatform(), true
	case Custom:
		return CustomPlatform(), true
	default:
		return Platform{}, false
	}
}

func UnknownPlatform() Platform {
	return Platform{ID: Unknown, DisplayName: "Unknown"}
}

func CustomPlatform() Platform {
	return Platform{ID: Custom, DisplayName: "Custom", Operational: true}
}

func InferFromServer(raw string) Platform {
	normalized, err := NormalizeServer(raw)
	if err != nil {
		return UnknownPlatform()
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return UnknownPlatform()
	}
	host := strings.ToLower(parsed.Hostname())
	for _, item := range registry {
		suffix := strings.ToLower(item.HostSuffix)
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return item
		}
	}
	return CustomPlatform()
}

func NormalizeServer(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("server URL is required; set LOOMLOOM_SERVER or pass --server")
	}
	if !strings.Contains(raw, "://") {
		scheme := "https"
		hostPart := raw
		if slash := strings.IndexByte(hostPart, '/'); slash >= 0 {
			hostPart = hostPart[:slash]
		}
		hostForCheck := hostPart
		if host, _, err := net.SplitHostPort(hostPart); err == nil {
			hostForCheck = host
		}
		hostForCheck = strings.Trim(hostForCheck, "[]")
		if isLoopbackHost(hostForCheck) {
			scheme = "http"
		}
		raw = scheme + "://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse server URL: %w", err)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported server URL scheme %q; use https", parsed.Scheme)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("server URL must not contain user information")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery {
		return "", fmt.Errorf("server URL must not contain a query")
	}
	if parsed.Fragment != "" {
		return "", fmt.Errorf("server URL must not contain a fragment")
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return "", fmt.Errorf("invalid server URL: missing host")
	}
	if parsed.Scheme == "http" && !isLoopbackHost(host) {
		return "", fmt.Errorf("remote server URL must use https")
	}

	port := parsed.Port()
	if (parsed.Scheme == "https" && port == "443") || (parsed.Scheme == "http" && port == "80") {
		port = ""
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	parsed.Host = host
	if port != "" {
		parsed.Host = net.JoinHostPort(strings.Trim(host, "[]"), port)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

var (
	profileNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	nonProfileChars    = regexp.MustCompile(`[^a-z0-9]+`)
)

func GenerateProfileName(server string, platformID ID, existing []Profile, requested string) (string, error) {
	normalized, err := NormalizeServer(server)
	if err != nil {
		return "", err
	}
	for _, profile := range existing {
		if profile.Server == normalized && strings.TrimSpace(requested) == "" {
			return profile.Name, nil
		}
	}

	if requested = strings.TrimSpace(strings.ToLower(requested)); requested != "" {
		if len(requested) > 64 || !profileNamePattern.MatchString(requested) {
			return "", fmt.Errorf("invalid server name %q; use lowercase letters, numbers, and single hyphens (max 64 characters)", requested)
		}
		for _, profile := range existing {
			if profile.Name == requested && profile.Server != normalized {
				return "", fmt.Errorf("server name %q is already used by %s", requested, profile.Server)
			}
		}
		return requested, nil
	}

	base := automaticProfileBase(normalized, platformID)
	base = fitProfileName(base, normalized, 8)
	if profileNameAvailable(base, normalized, existing) {
		return base, nil
	}
	hash := serverHash(normalized)
	for _, size := range []int{8, 12, 16, 24, 32, 48, 64} {
		if size > len(hash) {
			size = len(hash)
		}
		candidate := fitProfileName(base+"-"+hash[:size], normalized, size)
		if profileNameAvailable(candidate, normalized, existing) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not generate a unique server name")
}

func TokenEnvName(profileName string) string {
	return "LOOMLOOM_TOKEN_" + strings.ToUpper(strings.ReplaceAll(profileName, "-", "_"))
}

func automaticProfileBase(server string, platformID ID) string {
	parsed, _ := url.Parse(server)
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	var base string

	switch platformID {
	case ShengSuanYun, CogFoundry:
		p, _ := ByID(platformID)
		prefix := strings.TrimSuffix(host, p.HostSuffix)
		prefix = strings.TrimSuffix(prefix, ".")
		prefix = sanitizeProfilePart(prefix)
		prefix = strings.TrimPrefix(prefix, "loomloom-")
		if prefix == "loomloom" {
			prefix = ""
		}
		base = string(platformID)
		if prefix != "" {
			base += "-" + prefix
		}
	default:
		if strings.Contains(host, ":") {
			base = "custom-server-" + serverHash(server)[:8]
		} else {
			base = "custom-" + sanitizeProfilePart(host)
			if base == "custom-" {
				base = "custom-server-" + serverHash(server)[:8]
			}
		}
	}
	if port != "" {
		base += "-p" + sanitizeProfilePart(port)
	}
	return strings.Trim(base, "-")
}

func sanitizeProfilePart(value string) string {
	value = strings.ToLower(value)
	value = nonProfileChars.ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}

func fitProfileName(base, server string, hashSize int) string {
	base = sanitizeProfilePart(base)
	if base == "" {
		base = "custom-server"
	}
	if len(base) <= 64 {
		return base
	}
	hash := serverHash(server)
	if hashSize < 8 {
		hashSize = 8
	}
	if hashSize > len(hash) {
		hashSize = len(hash)
	}
	prefixLen := 64 - hashSize - 1
	prefix := strings.TrimRight(base[:prefixLen], "-")
	return prefix + "-" + hash[:hashSize]
}

func profileNameAvailable(name, server string, existing []Profile) bool {
	for _, profile := range existing {
		if profile.Name == name {
			return profile.Server == server
		}
	}
	return true
}

func serverHash(server string) string {
	sum := sha256.Sum256([]byte(server))
	return hex.EncodeToString(sum[:])
}
