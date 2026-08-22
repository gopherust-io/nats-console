package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/env"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func mustSessionPEMs(t *testing.T) (privPEM, pubPEM string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privPEM = commonstrings.BytesToString(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}))
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	pubPEM = commonstrings.BytesToString(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))
	return privPEM, pubPEM
}

func requiredEnv(t *testing.T, overrides map[string]string) map[string]string {
	t.Helper()
	priv, pub := mustSessionPEMs(t)
	m := map[string]string{
		"PROJECT_NAME":        "nats-console",
		"DATABASE_URL":        "postgres://u:p@db.example:5432/natsconsol?sslmode=verify-full",
		"ADMIN_PASSWORD":      "not-admin",
		"ENCRYPTION_KEY":      "long-enough-secret-key",
		"SESSION_PRIVATE_KEY": priv,
		"SESSION_PUBLIC_KEY":  pub,
		"NATS_URL":            "tls://nats.example:4222",
		"NATS_TOKEN":          "secret-token",
		"NATS_MONITORING_URL": "https://nats.example:8222",
		"SMTP_HOST":           "smtp.example.com",
		"SMTP_PORT":           "587",
	}
	maps.Copy(m, overrides)
	return m
}

func validConfig(t *testing.T) Config {
	t.Helper()
	priv, pub := mustSessionPEMs(t)
	return Config{
		DB: DBConfig{
			URL: "postgres://u:p@db.example:5432/natsconsol?sslmode=verify-full",
		},
		NATS: NATSConfig{
			URL:           "tls://nats.example:4222",
			Token:         "secret-token",
			MonitoringURL: "https://nats.example:8222",
		},
		EncryptionKey: "long-enough-secret-key",
		Auth: AuthConfig{
			SessionPrivateKey: priv,
			SessionPublicKey:  pub,
		},
		AdminPassword: "not-admin",
		Pprof: PprofConfig{
			AuthEnabled: true,
		},
	}
}

func TestLoadConfigRequiresDatabaseURL(t *testing.T) {
	_, err := LoadConfigFrom(env.FromMap(requiredEnv(t, map[string]string{"DATABASE_URL": ""})))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL")
}

func TestLoadConfigRequiresAdminPassword(t *testing.T) {
	_, err := LoadConfigFrom(env.FromMap(requiredEnv(t, map[string]string{"ADMIN_PASSWORD": ""})))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ADMIN_PASSWORD")
}

func TestLoadConfigRequiresEncryptionKey(t *testing.T) {
	_, err := LoadConfigFrom(env.FromMap(requiredEnv(t, map[string]string{"ENCRYPTION_KEY": ""})))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ENCRYPTION_KEY")

	cfg, err := LoadConfigFrom(env.FromMap(requiredEnv(t, nil)))
	require.NoError(t, err)
	require.NoError(t, cfg.Validate())
}

func TestValidateRequiresSessionKeys(t *testing.T) {
	cfg := validConfig(t)
	cfg.Auth.SessionPrivateKey = ""
	cfg.Auth.SessionPublicKey = ""
	require.Error(t, cfg.Validate())
	assert.Contains(t, cfg.Validate().Error(), "SESSION_PRIVATE_KEY")
}

func TestLoadConfigRequiresSessionKeys(t *testing.T) {
	_, err := LoadConfigFrom(env.FromMap(requiredEnv(t, map[string]string{"SESSION_PRIVATE_KEY": ""})))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SESSION_PRIVATE_KEY")

	_, err = LoadConfigFrom(env.FromMap(requiredEnv(t, map[string]string{"SESSION_PUBLIC_KEY": ""})))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SESSION_PUBLIC_KEY")
}

func TestTrustedProxyList(t *testing.T) {
	assert.Nil(t, Config{}.TrustedProxyList())
	assert.Nil(t, Config{TrustedProxies: "   "}.TrustedProxyList())
	assert.Equal(t, []string{"10.0.0.0/8", "192.168.1.1"}, Config{
		TrustedProxies: " 10.0.0.0/8 , 192.168.1.1 ,, ",
	}.TrustedProxyList())
}

func TestValidateAcceptsSecureConfig(t *testing.T) {
	cfg := validConfig(t)
	require.NoError(t, cfg.Validate(), "valid config rejected")

	cfg.PublicBaseURL = "https://example.com"
	cfg.AdminPassword = "admin"
	require.Error(t, cfg.Validate(), "default admin password should fail")
}

func TestValidateRejectsDefaultAdminInProduction(t *testing.T) {
	t.Setenv("ENV", "production")
	cfg := validConfig(t)
	cfg.PublicBaseURL = "http://localhost:8080"
	cfg.AdminPassword = "admin"
	require.Error(t, cfg.Validate(), "default admin password should fail in production even without HTTPS")
}

func TestTLSEnabled(t *testing.T) {
	httpsCfg := Config{PublicBaseURL: "https://example.com"}
	assert.True(t, httpsCfg.TLSEnabled(), "https public base url should enable TLS helpers")

	httpCfg := Config{PublicBaseURL: "http://localhost:8080"}
	assert.False(t, httpCfg.TLSEnabled(), "http public base url should not enable TLS helpers")
}

func TestValidateSMTPWhenEnabled(t *testing.T) {
	cfg := validConfig(t)
	cfg.SMTP.Enabled = true
	require.Error(t, cfg.Validate())
	assert.Contains(t, cfg.Validate().Error(), "SMTP_HOST")
	assert.Contains(t, cfg.Validate().Error(), "SMTP_FROM")

	cfg.SMTP.Host = "smtp.example.com"
	cfg.SMTP.From = "alerts@example.com"
	cfg.SMTP.Port = 587
	require.NoError(t, cfg.Validate())
}
