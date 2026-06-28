package auth_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/store"
)

func TestBasicAuthEnabledWithoutOIDC(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		AuthEnabled:   true,
		OIDCEnabled:   false,
		AdminPassword: "test-admin-password",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if svc.OIDCEnabled() {
		t.Fatal("expected OIDC disabled")
	}
	if !svc.BasicAuthEnabled() {
		t.Fatal("expected basic auth enabled when OIDC is off")
	}
}

func TestBasicAuthDisabledWhenAuthOff(t *testing.T) {
	svc, err := auth.NewService(config.Config{AuthEnabled: false}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if svc.BasicAuthEnabled() {
		t.Fatal("expected basic auth disabled when auth is off")
	}
}

func TestSessionRoundTrip(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		AuthEnabled:   true,
		SessionSecret: "test-session-secret-key",
		SessionTTL:    time.Hour,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	user := store.User{
		ID:       "user-1",
		Username: "alice",
		Roles:    []string{store.RoleAdmin},
	}
	token, err := svc.CreateSession(user)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := svc.ParseSession(token)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Username != user.Username || parsed.ID != user.ID {
		t.Fatalf("parsed user = %+v, want %+v", parsed, user)
	}
}

func TestSessionCookieSecureInProduction(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		Env:           "production",
		SessionSecret: "test-session-secret-key",
		SessionTTL:    time.Hour,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	cookie := svc.SessionCookie("token")
	if !cookie.Secure {
		t.Fatal("expected Secure cookie in production")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite = %v", cookie.SameSite)
	}
}
