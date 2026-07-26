package config

import (
	"net/url"
	"strings"
)

func validatePostgresSSLMode(databaseURL string, production bool) string {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return "DATABASE_URL is not a valid URL"
	}
	mode := strings.ToLower(strings.TrimSpace(u.Query().Get("sslmode")))
	secure := mode == "require" || mode == "verify-ca" || mode == "verify-full"
	if production && !secure {
		return "DATABASE_URL must set sslmode=require, verify-ca, or verify-full when ENV=production"
	}
	return ""
}

func validateSecureNATSURL(natsURL string, production bool) string {
	u, err := url.Parse(natsURL)
	if err != nil {
		return "NATS_URL is not a valid URL"
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "tls", "wss":
		return ""
	case "nats", "ws":
		if production {
			return "NATS_URL must use tls:// or wss:// when ENV=production"
		}
		return ""
	default:
		return "NATS_URL must use nats://, tls://, ws://, or wss://"
	}
}

func validateSecureMonitoringURL(monitoringURL string, production bool) string {
	u, err := url.Parse(monitoringURL)
	if err != nil {
		return "NATS_MONITORING_URL is not a valid URL"
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "https":
		return ""
	case "http":
		if production {
			return "NATS_MONITORING_URL must use https:// when ENV=production"
		}
		return ""
	default:
		return "NATS_MONITORING_URL must use http:// or https://"
	}
}
