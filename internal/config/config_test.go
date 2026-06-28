package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateProductionConfig(t *testing.T) {
	cfg := Config{
		Env:           "production",
		EncryptionKey: "long-enough-secret-key",
		SessionSecret: "another-long-secret",
		AuthEnabled:   true,
		AdminPassword: "not-admin",
	}
	require.NoError(t, cfg.Validate(), "valid production config rejected")

	cfg.AdminPassword = "admin"
	require.Error(t, cfg.Validate(), "default admin password should fail in production")
}

func TestTLSEnabled(t *testing.T) {
	httpsCfg := Config{PublicBaseURL: "https://example.com"}
	assert.True(t, httpsCfg.TLSEnabled(), "https public base url should enable TLS helpers")

	httpCfg := Config{PublicBaseURL: "http://localhost:8080"}
	assert.False(t, httpCfg.TLSEnabled(), "http public base url should not enable TLS helpers")
}
