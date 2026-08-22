package apikit

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

var (
	resourceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._\-/]{0,255}$`)
	// Case-insensitive; prefer strings.ToLower for storage/RBAC comparisons.
	uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

func ValidateResourceName(name string) error {
	if commonstrings.IsEmpty(name) || len(name) > 256 {
		return errors.New("invalid name")
	}
	if !resourceNamePattern.MatchString(name) {
		return errors.New("invalid name: use letters, numbers, dot, dash, underscore, or slash")
	}
	return nil
}

func ValidateClusterName(name string) error {
	if err := ValidateResourceName(name); err != nil {
		return err
	}
	if strings.Contains(name, "/") {
		return errors.New("invalid cluster name")
	}
	return nil
}

func ValidateNATSURL(raw string) error {
	if commonstrings.IsEmpty(raw) {
		return errors.New("invalid nats url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("invalid nats url")
	}
	switch strings.ToLower(u.Scheme) {
	case "nats", "tls", "ws", "wss":
	default:
		return errors.New("invalid nats url scheme")
	}
	if host := u.Hostname(); !commonstrings.IsEmpty(host) && isBlockedSSRFHost(host) {
		return errors.New("nats url host not allowed")
	}
	return nil
}

// ValidateMonitoringURL guards the cluster monitoringUrl field (H6): it must
// be http/https, and must not point at a link-local address, since the
// server fetches this URL on the caller's behalf (SSRF risk against cloud
// metadata endpoints such as 169.254.169.254).
func ValidateMonitoringURL(raw string) error {
	if commonstrings.IsEmpty(raw) {
		return errors.New("invalid monitoring url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("invalid monitoring url")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return errors.New("invalid monitoring url scheme: must be http or https")
	}
	host := u.Hostname()
	if commonstrings.IsEmpty(host) || isBlockedSSRFHost(host) {
		return errors.New("monitoring url host not allowed")
	}
	return nil
}

// isBlockedSSRFHost reports whether host is a known SSRF target: cloud
// metadata hostnames/IPs and link-local addresses (169.254.0.0/16 -
// including the 169.254.169.254 metadata endpoint used by AWS/GCP/Azure -
// and fe80::/10). Ordinary loopback/private addresses are intentionally
// still allowed since clusters are commonly reached over localhost or a
// private network in development and self-hosted deployments.
// Fetch paths additionally re-check redirect targets and resolved IPs
// (see monitoring_failover CheckRedirect and gopherust-io/nats Monitoring).
func isBlockedSSRFHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if commonstrings.IsEmpty(h) {
		return true
	}
	switch h {
	case "metadata.google.internal", "metadata.goog":
		return true
	}
	ip := net.ParseIP(h)
	if ip == nil {
		return false
	}
	return ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

func ValidateUUID(id string) error {
	if !uuidPattern.MatchString(id) {
		return errors.New("invalid id")
	}
	return nil
}

const minPasswordLen = 8

func ValidatePassword(password string) error {
	password = strings.TrimSpace(password)
	if commonstrings.IsEmpty(password) {
		return errors.New("password required")
	}
	if len(password) < minPasswordLen {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

func ValidateRoles(roles []string) error {
	if len(roles) == 0 {
		return errors.New("roles required")
	}
	for _, role := range roles {
		switch role {
		case domain.RoleAdmin, domain.RoleOperator, domain.RoleViewer:
		default:
			return errors.New("invalid role: " + role)
		}
	}
	return nil
}
