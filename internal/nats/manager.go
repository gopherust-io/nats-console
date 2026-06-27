package natsclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/store"
)

const clientCacheTTL = 5 * time.Minute

type Manager struct {
	store *store.Store
	cfg   config.Config

	mu    sync.Mutex
	cache map[string]*cachedClient
}

type cachedClient struct {
	client    *Client
	createdAt time.Time
}

func NewManager(st *store.Store, cfg config.Config) *Manager {
	return &Manager{
		store: st,
		cfg:   cfg,
		cache: make(map[string]*cachedClient),
	}
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
	cluster, err := m.store.GetCluster(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	return m.connect(cluster)
}

func (m *Manager) Test(ctx context.Context, clusterID string) (serverName string, jetstream bool, err error) {
	client, err := m.Get(ctx, clusterID)
	if err != nil {
		return "", false, err
	}
	defer m.evict(clusterID)

	if !client.nc.IsConnected() {
		return "", false, fmt.Errorf("not connected")
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
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, entry := range m.cache {
		entry.client.Close()
		delete(m.cache, id)
	}
}

func (m *Manager) connect(cluster store.Cluster) (*Client, error) {
	m.mu.Lock()
	if entry, ok := m.cache[cluster.ID]; ok && time.Since(entry.createdAt) < clientCacheTTL {
		client := entry.client
		m.mu.Unlock()
		return client, nil
	}
	m.mu.Unlock()

	client, err := ConnectCluster(cluster, m.cfg.RequestTimeout)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	if old, ok := m.cache[cluster.ID]; ok {
		old.client.Close()
	}
	m.cache[cluster.ID] = &cachedClient{client: client, createdAt: time.Now()}
	m.mu.Unlock()
	return client, nil
}

func (m *Manager) evict(clusterID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.cache[clusterID]; ok {
		entry.client.Close()
		delete(m.cache, clusterID)
	}
}

func ConnectCluster(cluster store.Cluster, timeout time.Duration) (*Client, error) {
	cfg := config.Config{
		NATSURL:        cluster.NATSURL,
		NATSCredsFile:  cluster.CredsFilePath,
		NATSToken:      cluster.Token,
		MonitoringURL:  cluster.MonitoringURL,
		RequestTimeout: timeout,
	}
	return Connect(cfg)
}
