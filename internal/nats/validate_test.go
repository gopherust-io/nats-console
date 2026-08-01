package natsclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/config"
)

func TestValidateSecureClientURL(t *testing.T) {
	t.Parallel()

	for _, u := range []string{"tls://nats.example:4222", "wss://nats.example/nats"} {
		require.NoError(t, ValidateSecureClientURL(u), "%q", u)
	}
	for _, u := range []string{"nats://nats.example:4222", "ws://nats.example", "http://bad"} {
		require.Error(t, ValidateSecureClientURL(u), "%q", u)
	}
	err := ValidateSecureClientURL("nats://localhost:4222")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tls://")
}

func TestValidateSecureMonitoringURL(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateSecureMonitoringURL("https://nats.example:8222"))
	for _, u := range []string{"http://nats.example:8222", "ftp://nats.example"} {
		require.Error(t, ValidateSecureMonitoringURL(u), "%q", u)
	}
	err := ValidateSecureMonitoringURL("http://localhost:8222")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "https://")
}

func TestValidateEnvConfig(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateEnvConfig(config.Config{}))

	ok := config.Config{
		NATS: config.NATSConfig{
			URL:           "tls://nats.example:4222",
			Token:         "secret",
			MonitoringURL: "https://nats.example:8222",
		},
	}
	require.NoError(t, ValidateEnvConfig(ok))

	skip := ok
	skip.NATS.TlsInsecureSkipVerify = true
	require.Error(t, ValidateEnvConfig(skip))
	assert.Contains(t, ValidateEnvConfig(skip).Error(), "NATS_TLS_INSECURE_SKIP_VERIFY")

	noCreds := ok
	noCreds.NATS.Token = ""
	noCreds.NATS.CredsFile = ""
	require.Error(t, ValidateEnvConfig(noCreds))
	assert.Contains(t, ValidateEnvConfig(noCreds).Error(), "NATS_CREDS_FILE or NATS_TOKEN")

	plain := ok
	plain.NATS.URL = "nats://nats.example:4222"
	require.Error(t, ValidateEnvConfig(plain))

	httpMon := ok
	httpMon.NATS.MonitoringURL = "http://nats.example:8222"
	require.Error(t, ValidateEnvConfig(httpMon))
}
