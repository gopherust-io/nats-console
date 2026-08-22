package natsclient

import (
	"errors"
	"net/url"
	"strings"

	"github.com/gopherust-io/nats-consol/internal/config"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const (
	schemeHttps = "https"
	schemeTLS   = "tls"
	schemeWSS   = "wss"
)

// ValidateSecureClientURL requires tls:// or wss:// for the env/default NATS client URL
func ValidateSecureClientURL(natsURL string) error {
	u, err := url.Parse(natsURL)
	if err != nil {
		return errors.New("NATS_URL is not a valid URL")
	}
	switch strings.ToLower(u.Scheme) {
	case schemeTLS, schemeWSS:
		return nil
	default:
		return errors.New("NATS_URL must use tls:// or wss://")
	}
}

// ValidateSecureMonitoringURL requires https:// for the env/default monitoring URL
func ValidateSecureMonitoringURL(monitoringURL string) error {
	u, err := url.Parse(monitoringURL)
	if err != nil {
		return errors.New("NATS_MONITORING_URL is not a valid URL")
	}
	switch strings.ToLower(u.Scheme) {
	case schemeHttps:
		return nil
	default:
		return errors.New("NATS_MONITORING_URL must use https://")
	}
}

// ValidateEnvConfig enforces production transport rules for env-backed NATS settings
// User/test clusters connected via ConnectCluster are not subject to these checks
func ValidateEnvConfig(cfg config.Config) error {
	if cfg.NATS.TlsInsecureSkipVerify {
		return errors.New("NATS_TLS_INSECURE_SKIP_VERIFY must be false")
	}
	if !commonstrings.IsEmpty(cfg.NATS.URL) {
		if err := ValidateSecureClientURL(cfg.NATS.URL); err != nil {
			return err
		}
		if commonstrings.IsEmpty(cfg.NATS.CredsFile) && commonstrings.IsEmpty(cfg.NATS.Token) {
			return errors.New("NATS_CREDS_FILE or NATS_TOKEN is required when NATS_URL is set")
		}
	}
	if !commonstrings.IsEmpty(cfg.NATS.MonitoringURL) {
		if err := ValidateSecureMonitoringURL(cfg.NATS.MonitoringURL); err != nil {
			return err
		}
	}
	return nil
}
