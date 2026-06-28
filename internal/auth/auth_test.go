package auth_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicAuthEnabledWithoutOIDC(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		AuthEnabled:   true,
		OIDCEnabled:   false,
		AdminPassword: "test-admin-password",
	}, nil)
	require.NoError(t, err)
	assert.False(t, svc.OIDCEnabled(), "expected OIDC disabled")
	assert.True(t, svc.BasicAuthEnabled(), "expected basic auth enabled when OIDC is off")
}

func TestBasicAuthDisabledWhenAuthOff(t *testing.T) {
	svc, err := auth.NewService(config.Config{AuthEnabled: false}, nil)
	require.NoError(t, err)
	assert.False(t, svc.BasicAuthEnabled(), "expected basic auth disabled when auth is off")
}

func TestSessionRoundTrip(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		AuthEnabled:   true,
		SessionSecret: "test-session-secret-key",
		SessionTTL:    time.Hour,
	}, nil)
	require.NoError(t, err)

	user := store.User{
		ID:       "user-1",
		Username: "alice",
		Roles:    []string{store.RoleAdmin},
	}
	token, err := svc.CreateSession(user)
	require.NoError(t, err)

	parsed, err := svc.ParseSession(token)
	require.NoError(t, err)
	assert.Equal(t, user.Username, parsed.Username)
	assert.Equal(t, user.ID, parsed.ID)
}

func TestSessionCookieSecureInProduction(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		Env:           "production",
		SessionSecret: "test-session-secret-key",
		SessionTTL:    time.Hour,
	}, nil)
	require.NoError(t, err)

	cookie := svc.SessionCookie("token")
	assert.True(t, cookie.Secure, "expected Secure cookie in production")
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}
