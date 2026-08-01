//go:build integration

package security_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/tests/testutil"
)

func TestLoginRateLimit(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.Auth.RateLimit = 3
		cfg.Auth.RateLimitWindow = time.Minute
	})

	var lastStatus int
	var lastBody []byte
	var lastRetryAfter string
	for i := 0; i < 5; i++ {
		resp, err := srv.Client.Post("http://nats-consol.local/api/v1/auth/login", "application/json", strings.NewReader(`{"username":"admin","password":"wrong"}`))
		require.NoError(t, err)
		lastStatus = resp.StatusCode
		lastRetryAfter = resp.Header("Retry-After")
		lastBody = resp.Body
	}
	require.Equal(t, http.StatusTooManyRequests, lastStatus, "last status")
	assert.Equal(t, "60", lastRetryAfter)

	var envelope struct {
		Error httpstatus.ErrorBody `json:"error"`
	}
	require.NoError(t, sonic.Unmarshal(lastBody, &envelope))
	assert.Equal(t, httpstatus.CodeRateLimit, envelope.Error.Code)
	assert.True(t, envelope.Error.Retryable)
	assert.Equal(t, 60, envelope.Error.RetryAfterSeconds)
}
