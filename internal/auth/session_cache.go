package auth

import (
	"time"

	"github.com/gopherust-io/nats-consol/internal/store"
)

const defaultSessionCacheTTL = 45 * time.Second

type sessionCache struct {
	cache *ttlCache[string, store.User]
}

func newSessionCache(ttl time.Duration) *sessionCache {
	if ttl <= 0 {
		ttl = defaultSessionCacheTTL
	}
	return &sessionCache{cache: newTTLCache[string, store.User](ttl)}
}

func (c *sessionCache) Get(token string) (store.User, bool) {
	return c.cache.Get(token)
}

func (c *sessionCache) Set(token string, user store.User) {
	if token == "" {
		return
	}
	c.cache.Set(token, user)
}

func (c *sessionCache) Invalidate(token string) {
	if token == "" {
		return
	}
	c.cache.Invalidate(token)
}
