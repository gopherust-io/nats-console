package auth

import (
	"time"

	"github.com/gopherust-io/nats-consol/internal/store"
)

const defaultUserCacheTTL = 45 * time.Second

type userCache struct {
	cache *ttlCache[string, store.User]
}

func newUserCache(ttl time.Duration) *userCache {
	if ttl <= 0 {
		ttl = defaultUserCacheTTL
	}
	return &userCache{cache: newTTLCache[string, store.User](ttl)}
}

func (c *userCache) Get(userID string) (store.User, bool) {
	return c.cache.Get(userID)
}

func (c *userCache) Set(user store.User) {
	if user.ID == "" {
		return
	}
	c.cache.Set(user.ID, user)
}

func (c *userCache) Invalidate(userID string) {
	if userID == "" {
		return
	}
	c.cache.Invalidate(userID)
}
