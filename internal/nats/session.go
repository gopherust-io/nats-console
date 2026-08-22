package natsclient

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

var (
	ErrSessionClosed      = errors.New("nats session closed")
	ErrSessionInvalidated = errors.New("nats session invalidated")
	ErrSessionUnhealthy   = errors.New("nats session unhealthy")
)

// Session is a live handle to a cached cluster connection.
// It does not own the underlying Client exclusively — Evict and the sweeper
// invalidate outstanding sessions so callers must re-acquire via Manager.Session.
type Session struct {
	manager   *Manager
	client    *Client
	clusterID string
	createdAt time.Time
	lastUsed  atomic.Int64
	gen       uint64
	closed    atomic.Bool
}

func (m *Manager) newSession(clusterID string, client *Client) *Session {
	now := time.Now()
	s := &Session{
		manager:   m,
		client:    client,
		clusterID: clusterID,
		createdAt: now,
		gen:       m.sessionGeneration(clusterID),
	}
	s.lastUsed.Store(now.UnixNano())
	return s
}

// Session returns a live cluster connection session, creating or reusing the
// cached client. On acquire it runs a health probe (with failure backoff).
func (m *Manager) Session(ctx context.Context, clusterID string) (*Session, error) {
	if m == nil {
		return nil, errors.New("nats manager is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	client, err := m.Get(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	if err := m.probeOnAcquire(ctx, clusterID, client); err != nil {
		return nil, err
	}

	m.Touch(clusterID)
	return m.newSession(clusterID, client), nil
}

func (s *Session) ClusterID() string {
	if s == nil {
		return ""
	}
	return s.clusterID
}

func (s *Session) CreatedAt() time.Time {
	if s == nil {
		return time.Time{}
	}
	return s.createdAt
}

func (s *Session) LastUsed() time.Time {
	if s == nil {
		return time.Time{}
	}
	return time.Unix(0, s.lastUsed.Load())
}

func (s *Session) touch() {
	if s == nil {
		return
	}
	now := time.Now()
	s.lastUsed.Store(now.UnixNano())
	if s.manager != nil {
		s.manager.Touch(s.clusterID)
	}
}

// Client returns the underlying NATS client when the session is still valid.
func (s *Session) Client() (*Client, error) {
	if err := s.ensureValid(); err != nil {
		return nil, err
	}
	s.touch()
	return s.client, nil
}

// Healthy probes connectivity (IsConnected / AccountInfo) using the manager's
// health-check timeout. Failed probes apply backoff for subsequent acquires.
func (s *Session) Healthy(ctx context.Context) error {
	if err := s.ensureValid(); err != nil {
		return err
	}
	if s.manager == nil {
		if !s.client.IsAlive() {
			return ErrSessionUnhealthy
		}
		return nil
	}
	if err := s.manager.probeHealth(ctx, s.clusterID, s.client); err != nil {
		return err
	}
	s.touch()
	return nil
}

// Close marks this handle closed without tearing down the shared cached client.
// Use Manager.Evict to drop the connection (e.g. after credential rotation).
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	s.closed.Store(true)
	return nil
}

func (s *Session) ensureValid() error {
	if s == nil || s.client == nil {
		return ErrSessionClosed
	}
	if s.closed.Load() {
		return ErrSessionClosed
	}
	if s.manager != nil && s.manager.sessionGeneration(s.clusterID) != s.gen {
		return ErrSessionInvalidated
	}
	return nil
}

func (m *Manager) sessionGeneration(clusterID string) uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.sessionGens == nil {
		return 0
	}
	return m.sessionGens[clusterID]
}

func (m *Manager) bumpSessionGenerationLocked(clusterID string) {
	if m.sessionGens == nil {
		m.sessionGens = make(map[string]uint64)
	}
	m.sessionGens[clusterID]++
}

const (
	healthBackoffBase = 500 * time.Millisecond
	healthBackoffMax  = 30 * time.Second
	healthProbeMinGap = 2 * time.Second
)

type healthBackoff struct {
	lastFailAt  time.Time
	lastProbeAt time.Time
	failCount   int
}

func (m *Manager) healthCheckTimeout() time.Duration {
	if m.cfg.HealthCheckTimeout > 0 {
		return m.cfg.HealthCheckTimeout
	}
	return 2 * time.Second
}

func (m *Manager) probeOnAcquire(ctx context.Context, clusterID string, client *Client) error {
	m.mu.RLock()
	hb := m.health[clusterID]
	m.mu.RUnlock()

	if hb != nil && hb.failCount > 0 {
		backoff := healthBackoffDuration(hb.failCount)
		if time.Since(hb.lastFailAt) < backoff {
			if !client.IsAlive() {
				m.recordHealthFailure(clusterID)
				m.evict(clusterID)
				return ErrSessionUnhealthy
			}
			// Soft allow: connection looks up, but still within backoff — skip heavy probe.
			return nil
		}
	}

	if hb != nil && !hb.lastProbeAt.IsZero() && time.Since(hb.lastProbeAt) < healthProbeMinGap {
		if !client.IsAlive() {
			m.recordHealthFailure(clusterID)
			m.evict(clusterID)
			return ErrSessionUnhealthy
		}
		return nil
	}

	return m.probeHealth(ctx, clusterID, client)
}

func (m *Manager) probeHealth(ctx context.Context, clusterID string, client *Client) error {
	if client == nil || !client.IsAlive() {
		m.recordHealthFailure(clusterID)
		m.evict(clusterID)
		return ErrSessionUnhealthy
	}

	timeout := m.healthCheckTimeout()
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, jsErr := client.AccountInfo(probeCtx)
	alive := client.IsAlive()

	now := time.Now()
	m.mu.Lock()
	if m.health == nil {
		m.health = make(map[string]*healthBackoff)
	}
	hb := m.health[clusterID]
	if hb == nil {
		hb = &healthBackoff{}
		m.health[clusterID] = hb
	}
	hb.lastProbeAt = now

	st := m.ensureState(clusterID)
	st.cached = true
	st.connected = alive
	st.serverName = client.ServerName()
	st.lastCheckedAt = now
	if jsErr != nil {
		st.jetStreamOK = false
		st.lastError = jsErr.Error()
	} else {
		st.jetStreamOK = true
		st.lastError = ""
	}
	m.publishStatusLocked(clusterID, m.snapshotLocked(clusterID))
	m.mu.Unlock()

	// Hard failure: connection dropped, or probe hung while the caller ctx is still live.
	hung := jsErr != nil && probeCtx.Err() != nil && ctx.Err() == nil
	if !alive || hung {
		m.recordHealthFailure(clusterID)
		m.evict(clusterID)
		return ErrSessionUnhealthy
	}

	m.recordHealthSuccess(clusterID)
	return nil
}

func (m *Manager) recordHealthFailure(clusterID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.health == nil {
		m.health = make(map[string]*healthBackoff)
	}
	hb := m.health[clusterID]
	if hb == nil {
		hb = &healthBackoff{}
		m.health[clusterID] = hb
	}
	hb.failCount++
	hb.lastFailAt = time.Now()
	hb.lastProbeAt = hb.lastFailAt
}

func (m *Manager) recordHealthSuccess(clusterID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.health == nil {
		return
	}
	if hb, ok := m.health[clusterID]; ok {
		hb.failCount = 0
		hb.lastFailAt = time.Time{}
		hb.lastProbeAt = time.Now()
	}
}

func healthBackoffDuration(failCount int) time.Duration {
	if failCount <= 0 {
		return 0
	}
	d := healthBackoffBase
	for i := 1; i < failCount && d < healthBackoffMax; i++ {
		d *= 2
	}
	if d > healthBackoffMax {
		return healthBackoffMax
	}
	return d
}
