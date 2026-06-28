package auth

import (
	"sync"
	"time"

	"github.com/gopherust-io/nats-consol/internal/store"
)

const defaultUserCacheTTL = 45 * time.Second

type userCache struct {
	entries map[string]userCacheEntry
	ttl     time.Duration
	mu      sync.RWMutex
}

type userCacheEntry struct {
	expiresAt time.Time
	user      store.User
}

func newUserCache(ttl time.Duration) *userCache {
	if ttl <= 0 {
		ttl = defaultUserCacheTTL
	}
	return &userCache{
		entries: make(map[string]userCacheEntry),
		ttl:     ttl,
	}
}

func (c *userCache) Get(userID string) (store.User, bool) {
	c.mu.RLock()
	entry, ok := c.entries[userID]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return store.User{}, false
	}
	return entry.user, true
}

func (c *userCache) Set(user store.User) {
	if user.ID == "" {
		return
	}
	c.mu.Lock()
	c.entries[user.ID] = userCacheEntry{
		user:      user,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

func (c *userCache) Invalidate(userID string) {
	if userID == "" {
		return
	}
	c.mu.Lock()
	delete(c.entries, userID)
	c.mu.Unlock()
}
