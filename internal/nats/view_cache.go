package natsclient

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"golang.org/x/sync/singleflight"

	"github.com/gopherust-io/nats-consol/internal/metrics"
)

const defaultViewCacheTTL = 3 * time.Second

// ViewCache coalesces and short-TTLs JetStream/monitoring read results per cluster.
type ViewCache struct {
	sf    singleflight.Group
	items map[string]viewCacheEntry
	ttl   time.Duration
	mu    sync.RWMutex
}

type viewCacheEntry struct {
	expiresAt time.Time
	payload   any
	etag      string
}

// NewViewCache creates a view cache with the given TTL (defaults to 3s).
func NewViewCache(ttl time.Duration) *ViewCache {
	if ttl <= 0 {
		ttl = defaultViewCacheTTL
	}
	return &ViewCache{
		items: make(map[string]viewCacheEntry),
		ttl:   ttl,
	}
}

// GetOrLoad returns a cached value or runs load once under singleflight.
func (c *ViewCache) GetOrLoad(key string, load func() (any, error)) (any, string, error) {
	if c == nil {
		v, err := load()
		return v, etagOf(v), err
	}
	if v, etag, ok := c.get(key); ok {
		metrics.IncViewCacheHit()
		return v, etag, nil
	}
	metrics.IncViewCacheMiss()

	v, err, _ := c.sf.Do(key, func() (any, error) {
		if v, etag, ok := c.get(key); ok {
			return cachedResult{v: v, etag: etag}, nil
		}
		raw, err := load()
		if err != nil {
			return nil, err
		}
		etag := etagOf(raw)
		c.set(key, raw, etag)
		return cachedResult{v: raw, etag: etag}, nil
	})
	if err != nil {
		return nil, "", err
	}
	if cr, ok := v.(cachedResult); ok {
		return cr.v, cr.etag, nil
	}
	return v, etagOf(v), nil
}

type cachedResult struct {
	v    any
	etag string
}

// InvalidateCluster drops all cached entries for a cluster ID prefix.
func (c *ViewCache) InvalidateCluster(clusterID string) {
	if c == nil || clusterID == "" {
		return
	}
	prefix := clusterID + "|"
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.items, k)
		}
	}
}

// InvalidatePrefix drops entries whose key starts with prefix.
func (c *ViewCache) InvalidatePrefix(prefix string) {
	if c == nil || prefix == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.items, k)
		}
	}
}

func (c *ViewCache) get(key string) (any, string, bool) {
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, "", false
	}
	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, "", false
	}
	return entry.payload, entry.etag, true
}

func (c *ViewCache) set(key string, payload any, etag string) {
	c.mu.Lock()
	c.items[key] = viewCacheEntry{
		payload:   payload,
		etag:      etag,
		expiresAt: time.Now().Add(c.ttl),
	}
	// Bound map growth under churn.
	if len(c.items) > 2048 {
		now := time.Now()
		for k, e := range c.items {
			if now.After(e.expiresAt) {
				delete(c.items, k)
			}
		}
	}
	c.mu.Unlock()
}

func etagOf(v any) string {
	switch t := v.(type) {
	case []byte:
		sum := sha256.Sum256(t)
		return `"` + hex.EncodeToString(sum[:8]) + `"`
	case string:
		sum := sha256.Sum256([]byte(t))
		return `"` + hex.EncodeToString(sum[:8]) + `"`
	default:
		raw, err := sonic.Marshal(v)
		if err != nil {
			sum := sha256.Sum256(fmt.Appendf(nil, "%T", v))
			return `"` + hex.EncodeToString(sum[:8]) + `"`
		}
		sum := sha256.Sum256(raw)
		return `"` + hex.EncodeToString(sum[:8]) + `"`
	}
}

// ViewCacheKey builds a cache key for a cluster-scoped read.
func ViewCacheKey(clusterID, op string, parts ...string) string {
	n := len(clusterID) + 1 + len(op)
	for _, p := range parts {
		n += 1 + len(p)
	}
	b := make([]byte, 0, n)
	b = append(b, clusterID...)
	b = append(b, '|')
	b = append(b, op...)
	for _, p := range parts {
		b = append(b, '|')
		b = append(b, p...)
	}
	return string(b)
}
