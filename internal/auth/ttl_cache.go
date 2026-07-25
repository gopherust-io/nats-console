package auth

import (
	"sync"
	"time"
)

const defaultTTLCacheTTL = 45 * time.Second

type ttlCache[K comparable, V any] struct {
	entries map[K]ttlCacheEntry[V]
	ttl     time.Duration
	mu      sync.RWMutex
}

type ttlCacheEntry[V any] struct {
	expiresAt time.Time
	value     V
}

func newTTLCache[K comparable, V any](ttl time.Duration) *ttlCache[K, V] {
	if ttl <= 0 {
		ttl = defaultTTLCacheTTL
	}
	return &ttlCache[K, V]{
		entries: make(map[K]ttlCacheEntry[V]),
		ttl:     ttl,
	}
}

func (c *ttlCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	var zero V
	if !ok || time.Now().After(entry.expiresAt) {
		return zero, false
	}
	return entry.value, true
}

func (c *ttlCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.entries[key] = ttlCacheEntry[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

func (c *ttlCache[K, V]) Invalidate(key K) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}
