//go:build integration

package security_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/require"
)

func TestLoginRateLimit(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
		cfg.AuthRateLimit = 3
		cfg.AuthRateLimitWindow = time.Minute
	})

	body := strings.NewReader(`{"username":"admin","password":"wrong"}`)
	var lastStatus int
	for i := 0; i < 5; i++ {
		resp, err := srv.Client.Post("http://nats-consol.local/api/v1/auth/login", "application/json", body)
		require.NoError(t, err)
		lastStatus = resp.StatusCode
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
	}
	require.Equal(t, http.StatusTooManyRequests, lastStatus, "last status")
}
