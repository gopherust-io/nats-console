//go:build integration

package security_test

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityHeadersPresent(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.PublicBaseURL = "https://nats-consol.example.com"
	})

	resp, err := srv.Client.Get("http://nats-consol.local/api/health")
	require.NoError(t, err)
	_ = resp.Body.Close()

	checks := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Content-Security-Policy":   "",
		"Strict-Transport-Security": "",
	}
	for header, want := range checks {
		got := resp.Header.Get(header)
		require.NotEmpty(t, got, "missing header %s", header)
		if want != "" {
			assert.Equal(t, want, got, "%s", header)
		}
	}
}
