package natsclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/config"
)

func TestSessionCloseInvalidatesHandle(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, config.Config{HealthCheckTimeout: time.Second})
	client := &Client{}
	entry := &cachedClient{client: client}
	entry.touch()
	m.cache["c1"] = entry

	sess := m.newSession("c1", client)
	require.NoError(t, sess.Close())

	_, err := sess.Client()
	assert.ErrorIs(t, err, ErrSessionClosed)
}

func TestSessionInvalidatedOnEvict(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, config.Config{HealthCheckTimeout: time.Second})
	client := &Client{}
	entry := &cachedClient{client: client}
	entry.touch()
	m.cache["c1"] = entry
	m.credCache["c1"] = cachedCredentials{fetchedAt: time.Now()}

	sess := m.newSession("c1", client)
	m.Evict("c1")

	_, err := sess.Client()
	assert.ErrorIs(t, err, ErrSessionInvalidated)

	m.mu.RLock()
	_, hasCred := m.credCache["c1"]
	m.mu.RUnlock()
	assert.False(t, hasCred, "Evict must clear credential cache")
}

func TestHealthBackoffDuration(t *testing.T) {
	t.Parallel()

	assert.Equal(t, time.Duration(0), healthBackoffDuration(0))
	assert.Equal(t, healthBackoffBase, healthBackoffDuration(1))
	assert.Equal(t, 2*healthBackoffBase, healthBackoffDuration(2))
	assert.Equal(t, healthBackoffMax, healthBackoffDuration(100))
}

func TestProbeHealthFailsWhenNotAlive(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, config.Config{HealthCheckTimeout: time.Millisecond})
	client := &Client{}
	entry := &cachedClient{client: client}
	entry.touch()
	m.cache["c1"] = entry

	err := m.probeHealth(context.Background(), "c1", client)
	require.ErrorIs(t, err, ErrSessionUnhealthy)

	m.mu.RLock()
	_, ok := m.cache["c1"]
	failCount := 0
	if hb := m.health["c1"]; hb != nil {
		failCount = hb.failCount
	}
	m.mu.RUnlock()
	assert.False(t, ok)
	assert.GreaterOrEqual(t, failCount, 1)
}

func TestScopedContextAddsTimeout(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, config.Config{RequestTimeout: 50 * time.Millisecond})
	assert.Equal(t, 50*time.Millisecond, m.RequestTimeout())
}
