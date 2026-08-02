//go:build integration

package security_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gopherust-io/nats-consol/internal/config"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginSetsSecureSessionCookies(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.PublicBaseURL = "https://nats-consol.example.com"
	})

	resp, err := srv.Client.Post(
		"http://nats-consol.local/api/v1/auth/login",
		"application/json",
		strings.NewReader(`{"username":"admin","password":"admin"}`),
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "status body = %s", commonstrings.BytesToString(resp.Body))

	var sessionCookie, refreshCookie, csrfCookie *testutil.Cookie
	for _, c := range resp.Cookies() {
		switch c.Name {
		case "nats_consol_session":
			sessionCookie = c
		case "nats_consol_refresh":
			refreshCookie = c
		case "nats_consol_csrf":
			csrfCookie = c
		}
	}
	require.NotNil(t, sessionCookie, "expected session cookie, got %#v", resp.Cookies())
	require.NotNil(t, refreshCookie, "expected refresh cookie, got %#v", resp.Cookies())
	require.NotNil(t, csrfCookie, "expected csrf cookie, got %#v", resp.Cookies())
	assert.True(t, sessionCookie.HttpOnly, "session cookie must be HttpOnly")
	assert.True(t, refreshCookie.HttpOnly, "refresh cookie must be HttpOnly")
	assert.True(t, sessionCookie.Secure, "cookies must be Secure")
	assert.True(t, refreshCookie.Secure, "refresh cookie must be Secure")
	assert.True(t, csrfCookie.Secure, "cookies must be Secure")
	assert.Equal(t, http.SameSiteLaxMode, sessionCookie.SameSite, "session SameSite")
}
