package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTTLCacheEvictsExpiredOnGet(t *testing.T) {
	cache := newTTLCache[string, string](time.Millisecond)
	cache.Set("a", "v")
	time.Sleep(2 * time.Millisecond)
	_, ok := cache.Get("a")
	assert.False(t, ok)
	assert.Equal(t, 0, cache.len(), "expired entry should be deleted on Get")
}

func TestTTLCachePurgeExpiredOnSet(t *testing.T) {
	cache := newTTLCache[string, string](time.Millisecond)
	for i := range 10 {
		cache.Set(string(rune('a'+i)), "v")
	}
	time.Sleep(2 * time.Millisecond)
	// Force purge path via ops counter / size check on next Set.
	cache.ops = ttlCachePurgeEvery - 1
	cache.Set("fresh", "ok")
	assert.Equal(t, 1, cache.len(), "purge should drop expired keys")
	got, ok := cache.Get("fresh")
	assert.True(t, ok)
	assert.Equal(t, "ok", got)
}
