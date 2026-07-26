package natsclient

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/internal/store"
	"golang.org/x/sync/singleflight"
)

const defaultClientCacheTTL = 5 * time.Minute

// goalign:ignore
type Manager struct {
	dial         singleflight.Group
	store        *store.Store
	cache        map[string]*cachedClient
	credCache    map[string]cachedCredentials
	status       map[string]*connectionState
	views        *ViewCache
	sweepStop    chan struct{}
	cfg          config.Config
	mu           sync.RWMutex
	sweepRunning atomic.Bool
}

type cachedClient struct {
	client       *Client
	lastUsedNano atomic.Int64
}

func (c *cachedClient) touch() {
	c.lastUsedNano.Store(time.Now().UnixNano())
}

func (c *cachedClient) lastUsed() time.Time {
	return time.Unix(0, c.lastUsedNano.Load())
}

func (c *cachedClient) setLastUsed(t time.Time) {
	c.lastUsedNano.Store(t.UnixNano())
}

type cachedCredentials struct {
	fetchedAt time.Time
	cluster   store.Cluster
}

func NewManager(st *store.Store, cfg config.Config) *Manager {
	viewTTL := cfg.JetStreamViewCacheTTL
	if viewTTL <= 0 {
		viewTTL = defaultViewCacheTTL
	}
	m := &Manager{
		store:     st,
		cfg:       cfg,
		cache:     make(map[string]*cachedClient),
		credCache: make(map[string]cachedCredentials),
		status:    make(map[string]*connectionState),
		views:     NewViewCache(viewTTL),
	}
	m.startSweeper()
	return m
}

// ViewCache returns the short-TTL JetStream/monitoring view cache.
func (m *Manager) ViewCache() *ViewCache {
	return m.views
}

// InvalidateViews drops cached JetStream/monitoring views for a cluster.
func (m *Manager) InvalidateViews(clusterID string) {
	if m == nil {
		return
	}
	m.views.InvalidateCluster(clusterID)
}

func (m *Manager) clientCacheTTL() time.Duration {
	if m.cfg.NATSClientCacheTTL > 0 {
		return m.cfg.NATSClientCacheTTL
	}
	return defaultClientCacheTTL
}

func (m *Manager) BootstrapDefaultCluster(ctx context.Context) error {
	count, err := m.store.CountClusters(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	_, err = m.store.CreateCluster(ctx, store.ClusterCreate{
		Name:          m.cfg.DefaultClusterName,
		NATSURL:       m.cfg.NATSURL,
		MonitoringURL: m.cfg.MonitoringURL,
		CredsFilePath: m.cfg.NATSCredsFile,
		Token:         m.cfg.NATSToken,
		IsDefault:     true,
	})
	return err
}

func (m *Manager) Get(ctx context.Context, clusterID string) (*Client, error) {
	cluster, err := m.clusterCredentials(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	return m.connect(cluster)
}

func (m *Manager) Test(ctx context.Context, clusterID string) (serverName string, jetstream bool, err error) {
	return m.ping(ctx, clusterID)
}

func (m *Manager) ping(ctx context.Context, clusterID string) (serverName string, jetstream bool, err error) {
	client, err := m.Get(ctx, clusterID)
	if err != nil {
		return "", false, err
	}

	if !client.IsAlive() {
		// Drop a stale cached client and dial once more before reporting failure.
		m.evict(clusterID)
		client, err = m.Get(ctx, clusterID)
		if err != nil {
			return "", false, err
		}
		if !client.IsAlive() {
			return "", false, errors.New("not connected")
		}
	}
	serverName = client.ServerName()

	_, err = client.AccountInfo(ctx)
	if err != nil {
		return serverName, false, nil
	}
	return serverName, true, nil
}

func (m *Manager) Evict(clusterID string) {
	m.evict(clusterID)
}

// Touch refreshes lastUsed for a cached client so long-lived WS sessions
// are not swept while still active. Uses a read lock + atomic timestamp so
// the hot path does not contend with dials/sweeps.
func (m *Manager) Touch(clusterID string) {
	m.mu.RLock()
	entry, ok := m.cache[clusterID]
	m.mu.RUnlock()
	if ok {
		entry.touch()
	}
}

func (m *Manager) Close() {
	if m.sweepStop != nil {
		close(m.sweepStop)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, entry := range m.cache {
		entry.client.Close()
		delete(m.cache, id)
	}
	m.credCache = make(map[string]cachedCredentials)
	metrics.SetNATSConnectionsActive(0)
}

func (m *Manager) clusterCredentials(ctx context.Context, clusterID string) (store.Cluster, error) {
	m.mu.RLock()
	if entry, ok := m.credCache[clusterID]; ok && time.Since(entry.fetchedAt) < m.clientCacheTTL() {
		cluster := entry.cluster
		m.mu.RUnlock()
		return cluster, nil
	}
	m.mu.RUnlock()

	cluster, err := m.store.GetClusterCredentials(ctx, clusterID)
	if err != nil {
		return store.Cluster{}, err
	}

	m.mu.Lock()
	m.credCache[clusterID] = cachedCredentials{cluster: cluster, fetchedAt: time.Now()}
	m.mu.Unlock()
	return cluster, nil
}

func (m *Manager) connect(cluster store.Cluster) (*Client, error) {
	// Fast path: read lock + atomic touch (same pattern as Touch).
	m.mu.RLock()
	if entry, ok := m.cache[cluster.ID]; ok && time.Since(entry.lastUsed()) < m.clientCacheTTL() {
		client := entry.client
		alive := client.IsAlive()
		m.mu.RUnlock()
		if alive {
			entry.touch()
			return client, nil
		}
		m.evict(cluster.ID)
	} else {
		m.mu.RUnlock()
	}

	result, err, _ := m.dial.Do(cluster.ID, func() (any, error) {
		m.mu.RLock()
		if entry, ok := m.cache[cluster.ID]; ok && time.Since(entry.lastUsed()) < m.clientCacheTTL() {
			client := entry.client
			alive := client.IsAlive()
			m.mu.RUnlock()
			if alive {
				entry.touch()
				return client, nil
			}
			m.evict(cluster.ID)
		} else {
			m.mu.RUnlock()
		}

		client, err := ConnectCluster(context.Background(), m.cfg, cluster, m.connectionHooks(cluster.ID))
		if err != nil {
			metrics.IncNATSDialError(cluster.ID)
			return nil, err
		}

		m.mu.Lock()
		if old, ok := m.cache[cluster.ID]; ok {
			old.client.Close()
		}
		entry := &cachedClient{client: client}
		entry.touch()
		m.cache[cluster.ID] = entry
		m.mu.Unlock()

		m.markConnected(cluster.ID, client)
		metrics.SetNATSConnectionsActive(m.activeConnectionCount())
		return client, nil
	})
	if err != nil {
		return nil, err
	}
	return result.(*Client), nil
}

func (m *Manager) evict(clusterID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.cache[clusterID]; ok {
		entry.client.Close()
		delete(m.cache, clusterID)
	}
	delete(m.credCache, clusterID)
	delete(m.status, clusterID)
}
