package natsclient

import (
	"context"
	"time"

	"github.com/gopherust-io/tel"
	"github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// goalign:ignore
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
		OnDisconnect: func(nc *nats.Conn, err error) {
			m.markDisconnected(clusterID, err)
			tel.Warn().
				Str("component", "NATS").
				Str("clusterID", clusterID).
				Errs("disconnected errors", []error{err, nc.LastError()}).
				Msg("NATS disconnected")
		},
		OnReconnect: func(nc *nats.Conn) {
			m.markReconnected(clusterID, nc)
			metrics.IncNATSReconnect(clusterID)
			tel.Info().
				Str("component", "nats").
				Str("clusterID", clusterID).
				Str("server", nc.ConnectedServerName()).
				Err(nc.LastError()).
				Msg("NATS reconnected")
		},
		OnClosed: func(nc *nats.Conn) {
			m.evict(clusterID)
			metrics.SetNATSConnectionsActive(m.activeConnectionCount())
			tel.Info().
				Str("component", "nats").
				Str("clusterID", clusterID).
				Err(nc.LastError()).
				Msg("NATS connection closed")
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
	m.publishStatusLocked(clusterID, m.snapshotLocked(clusterID))
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
	m.publishStatusLocked(clusterID, m.snapshotLocked(clusterID))
}

func (m *Manager) markConnected(clusterID string, client *Client) {
	now := time.Now()
	serverName := client.ServerName()
	alive := client.IsAlive()
	jsOK := false
	var jsErr string

	timeout := m.cfg.HealthCheckTimeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	probeCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if _, err := client.AccountInfo(probeCtx); err != nil {
		jsErr = err.Error()
	} else {
		jsOK = true
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	st := m.ensureState(clusterID)
	st.cached = true
	st.connected = alive
	st.serverName = serverName
	st.lastConnectedAt = now
	st.lastCheckedAt = now
	st.jetStreamOK = jsOK
	st.lastError = jsErr
	m.publishStatusLocked(clusterID, m.snapshotLocked(clusterID))
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

func (m *Manager) snapshotLocked(clusterID string) domain.NATSConnectionStatus {
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

func (m *Manager) stateSnapshot(clusterID string) domain.NATSConnectionStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotLocked(clusterID)
}

func (m *Manager) publishStatusLocked(clusterID string, status domain.NATSConnectionStatus) {
	listeners := m.statusListeners[clusterID]
	if len(listeners) == 0 {
		return
	}
	for ch := range listeners {
		select {
		case ch <- status:
		default:
		}
	}
}

// SubscribeStatus streams connection status updates for clusterID.
// latest is the current snapshot (might be zero-valued if never probed).
func (m *Manager) SubscribeStatus(clusterID string) (<-chan domain.NATSConnectionStatus, domain.NATSConnectionStatus, func()) {
	if m == nil || strings.IsEmpty(clusterID) {
		ch := make(chan domain.NATSConnectionStatus)
		close(ch)
		return ch, domain.NATSConnectionStatus{}, func() {}
	}

	ch := make(chan domain.NATSConnectionStatus, statusSubscriberBuffer)

	m.mu.Lock()
	if m.statusListeners == nil {
		m.statusListeners = make(map[string]map[chan domain.NATSConnectionStatus]struct{})
	}
	if m.statusListeners[clusterID] == nil {
		m.statusListeners[clusterID] = make(map[chan domain.NATSConnectionStatus]struct{})
	}
	m.statusListeners[clusterID][ch] = struct{}{}
	latest := m.snapshotLocked(clusterID)
	m.mu.Unlock()

	return ch, latest, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		listeners := m.statusListeners[clusterID]
		if listeners == nil {
			return
		}
		if _, ok := listeners[ch]; !ok {
			return
		}
		delete(listeners, ch)
		close(ch)
		if len(listeners) == 0 {
			delete(m.statusListeners, clusterID)
		}
	}
}

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
		out := m.snapshotLocked(clusterID)
		m.publishStatusLocked(clusterID, out)
		m.mu.Unlock()
		metrics.IncNATSDialError(clusterID)
		return out, nil
	}

	m.markConnected(clusterID, client)
	metrics.SetNATSConnectionsActive(m.activeConnectionCount())
	return m.stateSnapshot(clusterID), nil
}

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

func (m *Manager) StartSweeper(ctx context.Context) {
	if m.sweepRunning.Swap(true) {
		return
	}
	m.sweepStop = make(chan struct{})
	const defaultSweepInterval = 30 * time.Second

	interval := m.clientCacheTTL() / 2
	interval = max(interval, defaultSweepInterval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.sweepStop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.sweepExpired()
		}
	}
}

func (m *Manager) sweepExpired() {
	var (
		now     = time.Now()
		ttl     = m.clientCacheTTL()
		expired []string
		toClose []*Client
	)

	m.mu.Lock()
	for id, entry := range m.cache {
		if now.Sub(entry.lastUsed()) >= ttl || !entry.client.IsAlive() {
			expired = append(expired, id)
			toClose = append(toClose, entry.client)
			delete(m.cache, id)
			m.bumpSessionGenerationLocked(id)
			if st, ok := m.status[id]; ok {
				st.connected = false
				st.jetStreamOK = false
				st.lastCheckedAt = now
				m.publishStatusLocked(id, m.snapshotLocked(id))
			}
			delete(m.status, id)
		}
	}

	for id, entry := range m.credCache {
		if now.Sub(entry.fetchedAt) >= ttl {
			delete(m.credCache, id)
		}
	}
	m.mu.Unlock()

	for _, client := range toClose {
		if err := client.Close(); err != nil {
			tel.Error().
				Str("component", "NATS").
				Any("client", client).
				Err(err).
				Msg("failed to close NATS client")
		}
	}

	if len(expired) > 0 {
		metrics.SetNATSConnectionsActive(m.activeConnectionCount())
		tel.Debug().
			Str("component", "NATS").
			Strs("clusterIDs", expired).
			Msg("swept stale NATS connections")
	}
}
