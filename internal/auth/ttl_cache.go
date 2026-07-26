package auth

import (
	"sync"
	"time"
)

const (
	defaultTTLCacheTTL   = 45 * time.Second
	ttlCachePurgeEvery   = 256
	ttlCacheMaxStaleKeys = 512
)

type ttlCache[K comparable, V any] struct {
	entries map[K]ttlCacheEntry[V]
	ttl     time.Duration
	mu      sync.RWMutex
	ops     uint64
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
	if !ok {
		return zero, false
	}
	if !time.Now().After(entry.expiresAt) {
		return entry.value, true
	}
	c.mu.Lock()
	if e, still := c.entries[key]; still && time.Now().After(e.expiresAt) {
		delete(c.entries, key)
	}
	c.mu.Unlock()
	return zero, false
}

func (c *ttlCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.entries[key] = ttlCacheEntry[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.ops++
	if c.ops%ttlCachePurgeEvery == 0 || len(c.entries) > ttlCacheMaxStaleKeys {
		c.purgeExpiredLocked(time.Now())
	}
	c.mu.Unlock()
}

func (c *ttlCache[K, V]) Invalidate(key K) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

func (c *ttlCache[K, V]) purgeExpiredLocked(now time.Time) {
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}

func (c *ttlCache[K, V]) len() int {
	c.mu.RLock()
	n := len(c.entries)
	c.mu.RUnlock()
	return n
}
