package platform

import (
	"net/url"
	"strings"
)

type ID string

const (
	ShengSuanYun ID = "shengsuanyun"
	CogFoundry   ID = "cogfoundry"
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
		Operational: false,
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
	if id == "" || id == Unknown {
		return UnknownPlatform(), true
	}
	return Platform{}, false
}

func UnknownPlatform() Platform {
	return Platform{ID: Unknown, DisplayName: "Unknown"}
}

func InferFromServer(raw string) Platform {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return UnknownPlatform()
	}
	if !strings.Contains(raw, "://") {
		if strings.Contains(raw, ":") {
			raw = "http://" + raw
		} else {
			raw = "https://" + raw
		}
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return UnknownPlatform()
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return UnknownPlatform()
	}
	for _, item := range registry {
		suffix := strings.ToLower(item.HostSuffix)
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return item
		}
	}
	return UnknownPlatform()
}
