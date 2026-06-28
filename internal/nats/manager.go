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

type Manager struct {
	dial         singleflight.Group
	store        *store.Store
	cache        map[string]*cachedClient
	credCache    map[string]cachedCredentials
	status       map[string]*connectionState
	sweepStop    chan struct{}
	cfg          config.Config
	mu           sync.Mutex
	sweepRunning atomic.Bool
}

type cachedClient struct {
	client    *Client
	createdAt time.Time
}

type cachedCredentials struct {
	fetchedAt time.Time
	cluster   store.Cluster
}

func NewManager(st *store.Store, cfg config.Config) *Manager {
	m := &Manager{
		store:     st,
		cfg:       cfg,
		cache:     make(map[string]*cachedClient),
		credCache: make(map[string]cachedCredentials),
		status:    make(map[string]*connectionState),
	}
	m.startSweeper()
	return m
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

	if !client.nc.IsConnected() {
		return "", false, errors.New("not connected")
	}
	serverName = client.nc.ConnectedServerName()

	_, err = client.js.AccountInfo()
	if err != nil {
		return serverName, false, nil
	}
	return serverName, true, nil
}

func (m *Manager) Evict(clusterID string) {
	m.evict(clusterID)
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
	m.mu.Lock()
	if entry, ok := m.credCache[clusterID]; ok && time.Since(entry.fetchedAt) < m.clientCacheTTL() {
		cluster := entry.cluster
		m.mu.Unlock()
		return cluster, nil
	}
	m.mu.Unlock()

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
	m.mu.Lock()
	if entry, ok := m.cache[cluster.ID]; ok && time.Since(entry.createdAt) < m.clientCacheTTL() {
		client := entry.client
		if client.IsAlive() {
			m.mu.Unlock()
			return client, nil
		}
		entry.client.Close()
		delete(m.cache, cluster.ID)
	}
	m.mu.Unlock()

	result, err, _ := m.dial.Do(cluster.ID, func() (any, error) {
		m.mu.Lock()
		if entry, ok := m.cache[cluster.ID]; ok && time.Since(entry.createdAt) < m.clientCacheTTL() {
			client := entry.client
			if client.IsAlive() {
				m.mu.Unlock()
				return client, nil
			}
			entry.client.Close()
			delete(m.cache, cluster.ID)
		}
		m.mu.Unlock()

		client, err := ConnectCluster(cluster, m.cfg.RequestTimeout, m.connectionHooks(cluster.ID))
		if err != nil {
			metrics.IncNATSDialError(cluster.ID)
			return nil, err
		}

		m.mu.Lock()
		if old, ok := m.cache[cluster.ID]; ok {
			old.client.Close()
		}
		m.cache[cluster.ID] = &cachedClient{client: client, createdAt: time.Now()}
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
}

func ConnectCluster(cluster store.Cluster, timeout time.Duration, hooks ConnectionHooks) (*Client, error) {
	cfg := config.Config{
		NATSURL:        cluster.NATSURL,
		NATSCredsFile:  cluster.CredsFilePath,
		NATSToken:      cluster.Token,
		MonitoringURL:  cluster.MonitoringURL,
		RequestTimeout: timeout,
	}
	return Connect(cfg, hooks)
}
