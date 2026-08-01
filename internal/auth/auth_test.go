package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/store"
)

const testFingerprint = "fp-test-abcdef0123456789"

func testSessionConfig(t *testing.T, mutate func(*config.Config)) config.Config {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}))
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))
	cfg := config.Config{
		Auth: config.AuthConfig{
			SessionPrivateKey: privPEM,
			SessionPublicKey: pubPEM,
			SessionTTL: time.Hour,
			RefreshTokenTTL: 24 * time.Hour,
		},
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return cfg
}

func TestSessionRoundTrip(t *testing.T) {
	svc, err := auth.NewService(testSessionConfig(t, nil), nil)
	require.NoError(t, err)

	user := store.User{
		ID:       "user-1",
		Username: "alice",
		Roles:    []string{store.RoleAdmin},
	}
	token, err := svc.CreateSession(context.Background(), user, testFingerprint)
	require.NoError(t, err)

	parsed, err := svc.ParseSession(context.Background(), token, testFingerprint)
	require.NoError(t, err)
	assert.Equal(t, user.Username, parsed.Username)
	assert.Equal(t, user.ID, parsed.ID)
}

func TestParseSessionRejectsFingerprintMismatch(t *testing.T) {
	svc, err := auth.NewService(testSessionConfig(t, nil), nil)
	require.NoError(t, err)

	token, err := svc.CreateSession(context.Background(), store.User{ID: "u1", Username: "alice"}, testFingerprint)
	require.NoError(t, err)

	_, err = svc.ParseSession(context.Background(), token, "other-fingerprint")
	require.ErrorIs(t, err, auth.ErrUnauthorized)
}

func TestDeviceFingerprintStable(t *testing.T) {
	a := auth.DeviceFingerprint("Mozilla/5.0", "10.0.0.1")
	b := auth.DeviceFingerprint("Mozilla/5.0", "10.0.0.1")
	c := auth.DeviceFingerprint("Mozilla/5.0", "10.0.0.2")
	assert.Equal(t, a, b)
	assert.NotEqual(t, a, c)
	assert.Len(t, a, 64)
}

func TestParseSessionRejectsHS256(t *testing.T) {
	svc, err := auth.NewService(testSessionConfig(t, nil), nil)
	require.NoError(t, err)

	hsToken := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.Claims{
		UserID:      "user-1",
		Username:    "alice",
		Fingerprint: testFingerprint,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	signed, err := hsToken.SignedString([]byte("not-an-rsa-key-but-hmac-secret!!"))
	require.NoError(t, err)

	_, err = svc.ParseSession(context.Background(), signed, testFingerprint)
	require.ErrorIs(t, err, auth.ErrUnauthorized)
}

func TestParseSessionRejectsWrongPublicKey(t *testing.T) {
	svcA, err := auth.NewService(testSessionConfig(t, nil), nil)
	require.NoError(t, err)
	svcB, err := auth.NewService(testSessionConfig(t, nil), nil)
	require.NoError(t, err)

	token, err := svcA.CreateSession(context.Background(), store.User{ID: "u1", Username: "alice"}, testFingerprint)
	require.NoError(t, err)
	_, err = svcB.ParseSession(context.Background(), token, testFingerprint)
	require.ErrorIs(t, err, auth.ErrUnauthorized)
}

func TestCreateSessionRejectsEmptyUserID(t *testing.T) {
	svc, err := auth.NewService(testSessionConfig(t, nil), nil)
	require.NoError(t, err)

	_, err = svc.CreateSession(context.Background(), store.User{
		Username: "admin",
		Roles:    []string{store.RoleAdmin},
		IsRoot:   true,
	}, testFingerprint)
	require.ErrorIs(t, err, auth.ErrUnauthorized)
}

func TestSessionCookieSecure(t *testing.T) {
	svc, err := auth.NewService(testSessionConfig(t, nil), nil)
	require.NoError(t, err)

	cookie := svc.SessionCookie("token")
	assert.True(t, cookie.Secure, "expected Secure cookie")
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestNewServiceRequiresRSAKeys(t *testing.T) {
	_, err := auth.NewService(config.Config{Auth: config.AuthConfig{
		SessionTTL: time.Hour,
	}}, nil)
	require.Error(t, err)
}
