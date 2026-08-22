package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func testRSAConfig(t *testing.T) config.Config {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privPEM := commonstrings.BytesToString(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}))
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	pubPEM := commonstrings.BytesToString(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))
	return config.Config{
		Auth: config.AuthConfig{
			SessionPrivateKey: privPEM,
			SessionPublicKey:  pubPEM,
			SessionTTL:        time.Hour,
		},
	}
}

func TestInvalidateSessionRemovesCacheEntry(t *testing.T) {
	ctx := context.Background()
	svc, err := NewService(testRSAConfig(t), nil)
	require.NoError(t, err)

	user := domain.User{ID: "user-1", Username: "alice", Roles: []string{domain.RoleAdmin}}
	const fph = "fp-invalidate-test"
	token, err := svc.CreateSession(ctx, user, fph)
	require.NoError(t, err)

	_, err = svc.ParseSession(ctx, token, fph)
	require.NoError(t, err)
	assert.Equal(t, 1, svc.sessions.len())

	svc.InvalidateSession(ctx, token)
	assert.Equal(t, 0, svc.sessions.len())

	_, err = svc.ParseSession(ctx, token, fph)
	assert.ErrorIs(t, err, ErrUnauthorized, "revoked JWT must not re-authenticate")
}
