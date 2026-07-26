package auth

import (
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestSessionCacheGetSetInvalidate(t *testing.T) {
	cache := newSessionCache(time.Minute)
	user := store.User{ID: "u1", Username: "alice", Roles: []string{"admin"}}

	cache.Set("tok", user)
	got, ok := cache.Get("tok")
	assert.True(t, ok)
	assert.Equal(t, "alice", got.Username)

	cache.Invalidate("tok")
	_, ok = cache.Get("tok")
	assert.False(t, ok)
}

func TestSessionCacheExpires(t *testing.T) {
	cache := newSessionCache(time.Millisecond)
	user := store.User{ID: "u1", Username: "alice"}
	cache.Set("tok", user)
	time.Sleep(2 * time.Millisecond)
	_, ok := cache.Get("tok")
	assert.False(t, ok)
	assert.Equal(t, 0, cache.cache.len(), "expired session should be deleted on Get")
}
