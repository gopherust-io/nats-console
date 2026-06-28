package auth

import (
	"sync"
	"time"

	"github.com/gopherust-io/nats-consol/internal/store"
)

const defaultSessionCacheTTL = 45 * time.Second

type sessionCache struct {
	entries map[string]sessionCacheEntry
	ttl     time.Duration
	mu      sync.RWMutex
}

type sessionCacheEntry struct {
	expiresAt time.Time
	user      store.User
}

func newSessionCache(ttl time.Duration) *sessionCache {
	if ttl <= 0 {
		ttl = defaultSessionCacheTTL
	}
	return &sessionCache{
		entries: make(map[string]sessionCacheEntry),
		ttl:     ttl,
	}
}

func (c *sessionCache) Get(token string) (store.User, bool) {
	c.mu.RLock()
	entry, ok := c.entries[token]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return store.User{}, false
	}
	return entry.user, true
}

func (c *sessionCache) Set(token string, user store.User) {
	if token == "" {
		return
	}
	c.mu.Lock()
	c.entries[token] = sessionCacheEntry{
		user:      user,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

func (c *sessionCache) Invalidate(token string) {
	if token == "" {
		return
	}
	c.mu.Lock()
	delete(c.entries, token)
	c.mu.Unlock()
}
