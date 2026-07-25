package natsclient

import (
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientIsAlive(t *testing.T) {
	t.Parallel()

	client := &Client{}
	assert.False(t, client.IsAlive())
}

func TestManagerTouchRefreshesLastUsed(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, config.Config{NATSClientCacheTTL: time.Minute})
	old := time.Now().Add(-30 * time.Second)
	m.cache["cluster-1"] = &cachedClient{
		client:   &Client{},
		lastUsed: old,
	}

	m.Touch("cluster-1")

	m.mu.Lock()
	defer m.mu.Unlock()
	require.True(t, m.cache["cluster-1"].lastUsed.After(old))
}

func TestManagerSweepExpiredRemovesStaleEntry(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, config.Config{NATSClientCacheTTL: time.Millisecond})
	m.cache["cluster-1"] = &cachedClient{
		client:   &Client{},
		lastUsed: time.Now().Add(-time.Second),
	}

	m.sweepExpired()

	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.cache["cluster-1"]
	require.False(t, ok)
}

func TestManagerSweepExpiredRemovesDeadConnection(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, config.Config{NATSClientCacheTTL: time.Minute})
	m.cache["cluster-1"] = &cachedClient{
		client:   &Client{},
		lastUsed: time.Now(),
	}

	m.sweepExpired()

	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.cache["cluster-1"]
	require.False(t, ok)
}

func TestConnectionHooksMarkState(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, config.Config{})
	hooks := m.connectionHooks("cluster-1")

	hooks.OnDisconnect(nil, assert.AnError)
	status := m.stateSnapshot("cluster-1")
	assert.False(t, status.Connected)
	assert.Equal(t, assert.AnError.Error(), status.LastError)
}
