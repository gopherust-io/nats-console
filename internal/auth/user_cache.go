package auth

import (
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// Keep in step with userVersionCacheTTL so revoked grants are not served from
// a long-lived user cache after the version stamp has already been refreshed.
const defaultUserCacheTTL = 5 * time.Second

type userCache struct {
	cache *ttlCache[string, domain.User]
}

func newUserCache(ttl time.Duration) *userCache {
	if ttl <= 0 {
		ttl = defaultUserCacheTTL
	}
	return &userCache{cache: newTTLCache[string, domain.User](ttl)}
}

func (c *userCache) Get(userID string) (domain.User, bool) {
	return c.cache.Get(userID)
}

func (c *userCache) Set(user domain.User) {
	if strings.IsEmpty(user.ID) {
		return
	}
	c.cache.Set(user.ID, user)
}

func (c *userCache) Invalidate(userID string) {
	if strings.IsEmpty(userID) {
		return
	}
	c.cache.Invalidate(userID)
}
