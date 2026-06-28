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

func TestManagerEvictRemovesCache(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, config.Config{NATSClientCacheTTL: time.Minute})
	m.cache["cluster-1"] = &cachedClient{
		client:    &Client{},
		createdAt: time.Now(),
	}
	m.credCache["cluster-1"] = cachedCredentials{fetchedAt: time.Now()}

	m.Evict("cluster-1")

	m.mu.Lock()
	defer m.mu.Unlock()
	_, cached := m.cache["cluster-1"]
	_, creds := m.credCache["cluster-1"]
	require.False(t, cached)
	require.False(t, creds)
}

func TestManagerSweepExpiredRemovesStaleEntry(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, config.Config{NATSClientCacheTTL: time.Millisecond})
	m.cache["cluster-1"] = &cachedClient{
		client:    &Client{},
		createdAt: time.Now().Add(-time.Second),
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
		client:    &Client{},
		createdAt: time.Now(),
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
