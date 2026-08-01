package auth

import (
	"time"

	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const defaultSessionCacheTTL = 45 * time.Second

type cachedSession struct {
	fingerprint string
	user        store.User
}

type sessionCache struct {
	cache *ttlCache[string, cachedSession]
}

func newSessionCache(ttl time.Duration) *sessionCache {
	if ttl <= 0 {
		ttl = defaultSessionCacheTTL
	}
	return &sessionCache{cache: newTTLCache[string, cachedSession](ttl)}
}

func (c *sessionCache) Get(token, fingerprint string) (store.User, bool) {
	entry, ok := c.cache.Get(token)
	if !ok || entry.fingerprint != fingerprint {
		return store.User{}, false
	}
	return entry.user, true
}

func (c *sessionCache) Set(token, fingerprint string, user store.User) {
	if strings.IsEmpty(token) {
		return
	}
	c.cache.Set(token, cachedSession{user: user, fingerprint: fingerprint})
}

func (c *sessionCache) Invalidate(token string) {
	if strings.IsEmpty(token) {
		return
	}
	c.cache.Invalidate(token)
}

func (c *sessionCache) len() int {
	return c.cache.len()
}
