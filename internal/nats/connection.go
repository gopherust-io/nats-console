package natsclient

import (
	"context"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/log"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/nats-io/nats.go"
)

type connectionState struct {
	lastConnectedAt time.Time
	lastCheckedAt   time.Time
	serverName      string
	lastError       string
	reconnects      uint64
	connected       bool
	jetStreamOK     bool
	cached          bool
}

func (m *Manager) connectionHooks(clusterID string) ConnectionHooks {
	return ConnectionHooks{
		OnDisconnect: func(_ *nats.Conn, err error) {
			m.markDisconnected(clusterID, err)
			log.Warn().
				Str("component", "nats").
				Str("cluster_id", clusterID).
				Err(err).
				Msg("nats disconnected")
		},
		OnReconnect: func(nc *nats.Conn) {
			m.markReconnected(clusterID, nc)
			metrics.IncNATSReconnect(clusterID)
			log.Info().
				Str("component", "nats").
				Str("cluster_id", clusterID).
				Str("server", nc.ConnectedServerName()).
				Msg("nats reconnected")
		},
		OnClosed: func(_ *nats.Conn) {
			m.evict(clusterID)
			metrics.SetNATSConnectionsActive(m.activeConnectionCount())
			log.Info().
				Str("component", "nats").
				Str("cluster_id", clusterID).
				Msg("nats connection closed")
		},
	}
}

func (m *Manager) markDisconnected(clusterID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.ensureState(clusterID)
	st.connected = false
	st.jetStreamOK = false
	st.lastCheckedAt = time.Now()
	if err != nil {
		st.lastError = err.Error()
	}
}

func (m *Manager) markReconnected(clusterID string, nc *nats.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.ensureState(clusterID)
	now := time.Now()
	st.connected = nc.IsConnected()
	st.serverName = nc.ConnectedServerName()
	st.lastConnectedAt = now
	st.lastCheckedAt = now
	st.lastError = ""
	st.reconnects++
}

func (m *Manager) markConnected(clusterID string, client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.ensureState(clusterID)
	now := time.Now()
	st.cached = true
	st.connected = client.IsAlive()
	st.serverName = client.ServerName()
	st.lastConnectedAt = now
	st.lastCheckedAt = now
	st.lastError = ""
	if _, err := client.AccountInfo(context.Background()); err == nil {
		st.jetStreamOK = true
	} else {
		st.jetStreamOK = false
		st.lastError = err.Error()
	}
}

func (m *Manager) ensureState(clusterID string) *connectionState {
	if m.status == nil {
		m.status = make(map[string]*connectionState)
	}
	st, ok := m.status[clusterID]
	if !ok {
		st = &connectionState{}
		m.status[clusterID] = st
	}
	return st
}

func (m *Manager) stateSnapshot(clusterID string) domain.NATSConnectionStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.ensureState(clusterID)
	_, cached := m.cache[clusterID]
	out := domain.NATSConnectionStatus{
		ClusterID:     clusterID,
		Connected:     st.connected,
		Cached:        cached,
		JetStreamOK:   st.jetStreamOK,
		ServerName:    st.serverName,
		LastCheckedAt: st.lastCheckedAt,
		LastError:     st.lastError,
		Reconnects:    st.reconnects,
	}
	if !st.lastConnectedAt.IsZero() {
		t := st.lastConnectedAt
		out.LastConnectedAt = &t
	}
	return out
}

// Status returns the current connection status for a cluster (live probe).
func (m *Manager) Status(ctx context.Context, clusterID string) (domain.NATSConnectionStatus, error) {
	if _, err := m.clusterCredentials(ctx, clusterID); err != nil {
		return domain.NATSConnectionStatus{}, err
	}

	client, err := m.Get(ctx, clusterID)
	if err != nil {
		m.mu.Lock()
		st := m.ensureState(clusterID)
		st.connected = false
		st.jetStreamOK = false
		st.lastCheckedAt = time.Now()
		st.lastError = err.Error()
		m.mu.Unlock()
		metrics.IncNATSDialError(clusterID)
		out := m.stateSnapshot(clusterID)
		return out, nil
	}

	m.markConnected(clusterID, client)
	metrics.SetNATSConnectionsActive(m.activeConnectionCount())
	return m.stateSnapshot(clusterID), nil
}

// ListStatuses returns connection status for all currently cached clusters.
func (m *Manager) ListStatuses() []domain.NATSConnectionStatus {
	m.mu.Lock()
	ids := make([]string, 0, len(m.cache))
	for id := range m.cache {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	out := make([]domain.NATSConnectionStatus, 0, len(ids))
	for _, id := range ids {
		out = append(out, m.stateSnapshot(id))
	}
	return out
}

func (m *Manager) activeConnectionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, entry := range m.cache {
		if entry.client.IsAlive() {
			count++
		}
	}
	return count
}

func (m *Manager) startSweeper() {
	if m.sweepRunning.Swap(true) {
		return
	}
	m.sweepStop = make(chan struct{})
	go m.runSweeper()
}

func (m *Manager) runSweeper() {
	interval := m.clientCacheTTL() / 2
	interval = max(interval, 30*time.Second)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.sweepExpired()
		case <-m.sweepStop:
			return
		}
	}
}

func (m *Manager) sweepExpired() {
	now := time.Now()
	ttl := m.clientCacheTTL()
	var expired []string

	m.mu.Lock()
	for id, entry := range m.cache {
		if now.Sub(entry.createdAt) >= ttl {
			expired = append(expired, id)
			entry.client.Close()
			delete(m.cache, id)
		} else if !entry.client.IsAlive() {
			expired = append(expired, id)
			entry.client.Close()
			delete(m.cache, id)
		}
	}
	m.mu.Unlock()

	if len(expired) > 0 {
		metrics.SetNATSConnectionsActive(m.activeConnectionCount())
		log.Debug().
			Str("component", "nats").
			Strs("cluster_ids", expired).
			Msg("swept stale nats connections")
	}
}
