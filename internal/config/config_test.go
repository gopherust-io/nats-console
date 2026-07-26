package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRequiresDatabaseURL(t *testing.T) {
	cfg := Config{Env: "development", AuthEnabled: false}
	require.Error(t, cfg.Validate())
	assert.Contains(t, cfg.Validate().Error(), "DATABASE_URL")
}

func TestValidateRequiresAdminPasswordWhenAuthEnabled(t *testing.T) {
	cfg := Config{
		Env:         "development",
		DatabaseURL: "postgres://u:p@localhost:5432/db?sslmode=disable",
		AuthEnabled: true,
	}
	require.Error(t, cfg.Validate())
	assert.Contains(t, cfg.Validate().Error(), "ADMIN_PASSWORD")
}

func TestValidateProductionConfig(t *testing.T) {
	cfg := Config{
		Env:              "production",
		DatabaseURL:      "postgres://u:p@db.example:5432/natsconsol?sslmode=verify-full",
		NATSURL:          "tls://nats.example:4222",
		NATSToken:        "secret-token",
		MonitoringURL:    "https://nats.example:8222",
		EncryptionKey:    "long-enough-secret-key",
		SessionSecret:    "another-long-secret",
		AuthEnabled:      true,
		AdminPassword:    "not-admin",
		PprofAuthEnabled: true,
	}
	require.NoError(t, cfg.Validate(), "valid production config rejected")

	cfg.AdminPassword = "admin"
	require.Error(t, cfg.Validate(), "default admin password should fail in production")
}

func TestValidateProductionRejectsInsecureTransports(t *testing.T) {
	base := Config{
		Env:              "production",
		DatabaseURL:      "postgres://u:p@db.example:5432/natsconsol?sslmode=disable",
		NATSURL:          "nats://nats.example:4222",
		MonitoringURL:    "http://nats.example:8222",
		EncryptionKey:    "long-enough-secret-key",
		SessionSecret:    "another-long-secret",
		AuthEnabled:      true,
		AdminPassword:    "not-admin",
		PprofAuthEnabled: true,
	}
	err := base.Validate()
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "sslmode")
	assert.Contains(t, msg, "tls://")
	assert.Contains(t, msg, "https://")
	assert.Contains(t, msg, "NATS_CREDS_FILE or NATS_TOKEN")
}

func TestValidateProductionRejectsSkipVerify(t *testing.T) {
	cfg := Config{
		Env:                       "production",
		DatabaseURL:               "postgres://u:p@db.example:5432/natsconsol?sslmode=require",
		NATSURL:                   "tls://nats.example:4222",
		NATSToken:                 "token",
		MonitoringURL:             "https://nats.example:8222",
		EncryptionKey:             "long-enough-secret-key",
		SessionSecret:             "another-long-secret",
		AuthEnabled:               true,
		AdminPassword:             "not-admin",
		NATSTlsInsecureSkipVerify: true,
	}
	require.Error(t, cfg.Validate())
	assert.Contains(t, cfg.Validate().Error(), "NATS_TLS_INSECURE_SKIP_VERIFY")
}

func TestTLSEnabled(t *testing.T) {
	httpsCfg := Config{PublicBaseURL: "https://example.com"}
	assert.True(t, httpsCfg.TLSEnabled(), "https public base url should enable TLS helpers")

	httpCfg := Config{PublicBaseURL: "http://localhost:8080"}
	assert.False(t, httpCfg.TLSEnabled(), "http public base url should not enable TLS helpers")
}

func TestValidateSMTPWhenEnabled(t *testing.T) {
	cfg := Config{
		Env:         "development",
		DatabaseURL: "postgres://u:p@localhost:5432/db?sslmode=disable",
		AuthEnabled: false,
		SMTPEnabled: true,
	}
	require.Error(t, cfg.Validate())
	assert.Contains(t, cfg.Validate().Error(), "SMTP_HOST")
	assert.Contains(t, cfg.Validate().Error(), "SMTP_FROM")

	cfg.SMTPHost = "smtp.example.com"
	cfg.SMTPFrom = "alerts@example.com"
	cfg.SMTPPort = 587
	require.NoError(t, cfg.Validate())
}
