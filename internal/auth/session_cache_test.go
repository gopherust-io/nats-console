package auth

import (
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestSessionCacheGetSetInvalidate(t *testing.T) {
	cache := newSessionCache(time.Minute)
	user := domain.User{ID: "u1", Username: "alice", Roles: []string{"admin"}}

	cache.Set("tok", "fp1", user)
	got, ok := cache.Get("tok", "fp1")
	assert.True(t, ok)
	assert.Equal(t, "alice", got.Username)

	_, ok = cache.Get("tok", "fp-other")
	assert.False(t, ok, "fingerprint mismatch must miss cache")

	cache.Invalidate("tok")
	_, ok = cache.Get("tok", "fp1")
	assert.False(t, ok)
}

func TestSessionCacheExpires(t *testing.T) {
	cache := newSessionCache(time.Millisecond)
	user := domain.User{ID: "u1", Username: "alice"}
	cache.Set("tok", "fp1", user)
	time.Sleep(2 * time.Millisecond)
	_, ok := cache.Get("tok", "fp1")
	assert.False(t, ok)
	assert.Equal(t, 0, cache.cache.len(), "expired session should be deleted on Get")
}
