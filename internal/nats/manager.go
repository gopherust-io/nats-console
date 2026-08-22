package natsclient

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/internal/repo"
)

const defaultClientCacheTTL = 5 * time.Minute
const statusSubscriberBuffer = 4

// goalign:ignore
type Manager struct {
	dial            singleflight.Group
	db              *repo.DB
	cache           map[string]*cachedClient
	credCache       map[string]cachedCredentials
	status          map[string]*connectionState
	statusListeners map[string]map[chan domain.NATSConnectionStatus]struct{}
	sessionGens     map[string]uint64
	health          map[string]*healthBackoff
	views           *ViewCache
	sweepStop       chan struct{}
	cfg             config.Config
	mu              sync.RWMutex
	sweepRunning    atomic.Bool
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
	cluster   repo.Cluster
}

func NewManager(db *repo.DB, cfg config.Config) *Manager {
	return &Manager{
		db:              db,
		cfg:             cfg,
		cache:           make(map[string]*cachedClient),
		credCache:       make(map[string]cachedCredentials),
		status:          make(map[string]*connectionState),
		statusListeners: make(map[string]map[chan domain.NATSConnectionStatus]struct{}),
		sessionGens:     make(map[string]uint64),
		health:          make(map[string]*healthBackoff),
		views:           NewViewCache(cfg.JetStreamViewCacheTTL),
	}
}

// RequestTimeout returns the configured per-request timeout used by scoped
// executor helpers (WithExecutor). Zero means no wrapper timeout.
func (m *Manager) RequestTimeout() time.Duration {
	if m == nil {
		return 0
	}
	return m.cfg.RequestTimeout
}

func (m *Manager) ViewCache() *ViewCache {
	return m.views
}

func (m *Manager) InvalidateViews(clusterID string) {
	if m == nil {
		return
	}
	m.views.InvalidateCluster(clusterID)
}

func (m *Manager) clientCacheTTL() time.Duration {
	if m.cfg.NATS.ClientCacheTTL > 0 {
		return m.cfg.NATS.ClientCacheTTL
	}
	return defaultClientCacheTTL
}

func (m *Manager) BootstrapDefaultCluster(ctx context.Context) error {
	count, err := m.db.CountClusters(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	_, err = m.db.CreateCluster(ctx, repo.ClusterCreate{
		Name:          m.cfg.DefaultClusterName,
		NATSURL:       m.cfg.NATS.URL,
		MonitoringURL: m.cfg.NATS.MonitoringURL,
		CredsFilePath: m.cfg.NATS.CredsFile,
		Token:         m.cfg.NATS.Token,
		IsDefault:     true,
	})
	return err
}

func (m *Manager) Get(ctx context.Context, clusterID string) (*Client, error) {
	cluster, err := m.clusterCredentials(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	return m.connect(ctx, cluster)
}

func (m *Manager) Test(ctx context.Context, clusterID string) (serverName string, jetStream bool, err error) {
	return m.ping(ctx, clusterID)
}

func (m *Manager) ping(ctx context.Context, clusterID string) (serverName string, jetStream bool, err error) {
	client, err := m.Get(ctx, clusterID)
	if err != nil {
		return "", false, err
	}

	if !client.IsAlive() {
		// Drop a stale cached client and dial once more before reporting failure
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

func (m *Manager) Stop() {
	m.mu.Lock()
	if m.sweepStop != nil {
		close(m.sweepStop)
		m.sweepStop = nil
	}
	toClose := make([]*Client, 0, len(m.cache))
	for id, entry := range m.cache {
		toClose = append(toClose, entry.client)
		delete(m.cache, id)
		m.bumpSessionGenerationLocked(id)
	}
	m.credCache = make(map[string]cachedCredentials)
	m.health = make(map[string]*healthBackoff)
	for _, listeners := range m.statusListeners {
		for ch := range listeners {
			close(ch)
		}
	}
	m.statusListeners = make(map[string]map[chan domain.NATSConnectionStatus]struct{})
	m.mu.Unlock()
	for _, client := range toClose {
		_ = client.Close()
	}
	metrics.SetNATSConnectionsActive(0)
}

func (m *Manager) clusterCredentials(ctx context.Context, clusterID string) (repo.Cluster, error) {
	m.mu.RLock()
	if entry, ok := m.credCache[clusterID]; ok && time.Since(entry.fetchedAt) < m.clientCacheTTL() {
		cluster := entry.cluster
		m.mu.RUnlock()
		return cluster, nil
	}
	m.mu.RUnlock()

	cluster, err := m.db.GetClusterCredentials(ctx, clusterID)
	if err != nil {
		return repo.Cluster{}, err
	}

	m.mu.Lock()
	m.credCache[clusterID] = cachedCredentials{cluster: cluster, fetchedAt: time.Now()}
	m.mu.Unlock()
	return cluster, nil
}

func (m *Manager) connect(ctx context.Context, cluster repo.Cluster) (*Client, error) {
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

		dialStart := time.Now()
		client, err := ConnectCluster(ctx, m.cfg, cluster, m.connectionHooks(cluster.ID))
		metrics.ObserveNATSDialLatency(cluster.ID, time.Since(dialStart))
		if err != nil {
			metrics.IncNATSDialError(cluster.ID)
			return nil, err
		}

		var oldClient *Client
		m.mu.Lock()
		if old, ok := m.cache[cluster.ID]; ok {
			oldClient = old.client
			m.bumpSessionGenerationLocked(cluster.ID)
		}
		entry := &cachedClient{client: client}
		entry.touch()
		m.cache[cluster.ID] = entry
		m.mu.Unlock()
		if oldClient != nil {
			_ = oldClient.Close()
		}

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
	var toClose *Client
	if entry, ok := m.cache[clusterID]; ok {
		toClose = entry.client
		delete(m.cache, clusterID)
	}
	delete(m.credCache, clusterID)
	m.bumpSessionGenerationLocked(clusterID)
	if st, ok := m.status[clusterID]; ok {
		st.connected = false
		st.jetStreamOK = false
		st.lastCheckedAt = time.Now()
		snap := m.snapshotLocked(clusterID)
		m.publishStatusLocked(clusterID, snap)
		delete(m.status, clusterID)
	}
	m.mu.Unlock()
	if toClose != nil {
		_ = toClose.Close()
	}
	metrics.SetNATSConnectionsActive(m.activeConnectionCount())
}
