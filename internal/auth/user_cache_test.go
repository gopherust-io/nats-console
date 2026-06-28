package auth

import (
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestUserCacheGetSetInvalidate(t *testing.T) {
	cache := newUserCache(time.Minute)
	user := store.User{ID: "u1", Username: "alice", Roles: []string{"admin"}}

	cache.Set(user)
	got, ok := cache.Get("u1")
	assert.True(t, ok)
	assert.Equal(t, "alice", got.Username)

	cache.Invalidate("u1")
	_, ok = cache.Get("u1")
	assert.False(t, ok)
}

func TestUserCacheExpires(t *testing.T) {
	cache := newUserCache(time.Millisecond)
	user := store.User{ID: "u1", Username: "alice"}
	cache.Set(user)
	time.Sleep(2 * time.Millisecond)
	_, ok := cache.Get("u1")
	assert.False(t, ok)
}
