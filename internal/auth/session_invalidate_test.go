package auth

import (
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvalidateSessionRemovesCacheEntry(t *testing.T) {
	svc, err := NewService(config.Config{
		AuthEnabled:   true,
		SessionSecret: "test-session-secret-key",
		SessionTTL:    time.Hour,
	}, nil)
	require.NoError(t, err)

	user := store.User{ID: "user-1", Username: "alice", Roles: []string{store.RoleAdmin}}
	token, err := svc.CreateSession(user)
	require.NoError(t, err)

	_, err = svc.ParseSession(token)
	require.NoError(t, err)
	assert.Equal(t, 1, svc.sessions.cache.len())

	svc.InvalidateSession(token)
	assert.Equal(t, 0, svc.sessions.cache.len())

	_, err = svc.ParseSession(token)
	assert.ErrorIs(t, err, ErrUnauthorized, "revoked JWT must not re-authenticate")
}
