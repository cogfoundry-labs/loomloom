package workflowpreview

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateViewURL rejects unsafe destinations without assuming a deployment
// domain or opening a URL. The trusted Server remains the address authority.
func ValidateViewURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" ||
		strings.ContainsAny(raw, "\r\n\t") {
		return fmt.Errorf("invalid workbench viewUrl returned by server")
	}
	if u.Scheme == "https" || u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1") {
		return nil
	}
	return fmt.Errorf("workbench viewUrl requires HTTPS (HTTP is allowed only on loopback)")
}
