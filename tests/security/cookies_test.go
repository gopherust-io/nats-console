//go:build integration

package security_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginSetsSecureSessionCookies(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
		cfg.Env = "production"
		cfg.PublicBaseURL = "https://nats-consol.example.com"
	})

	resp, err := srv.Client.Post(
		"http://nats-consol.local/api/v1/auth/login",
		"application/json",
		strings.NewReader(`{"username":"admin","password":"admin"}`),
	)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "status body = %s", string(body))

	var sessionCookie, csrfCookie *http.Cookie
	for _, c := range resp.Cookies() {
		switch c.Name {
		case "nats_consol_session":
			sessionCookie = c
		case "nats_consol_csrf":
			csrfCookie = c
		}
	}
	require.NotNil(t, sessionCookie, "expected session cookie, got %#v", resp.Cookies())
	require.NotNil(t, csrfCookie, "expected csrf cookie, got %#v", resp.Cookies())
	assert.True(t, sessionCookie.HttpOnly, "session cookie must be HttpOnly")
	assert.True(t, sessionCookie.Secure, "cookies must be Secure in production")
	assert.True(t, csrfCookie.Secure, "cookies must be Secure in production")
	assert.Equal(t, http.SameSiteLaxMode, sessionCookie.SameSite, "session SameSite")
}
