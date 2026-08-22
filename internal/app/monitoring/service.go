package monitoring

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gopherust-io/nats-consol/internal/app/query"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"golang.org/x/sync/singleflight"
)

// DefaultCacheTTL is the short in-process TTL for raw/parsed JSZ per cluster.
const DefaultCacheTTL = 3 * time.Second

// ErrPayloadTooLarge re-exports the query package sentinel for handler convenience.
var ErrPayloadTooLarge = query.ErrPayloadTooLarge

// Service owns JSZ topology fetch (hub + live), short TTL cache, parse, and insight extractors.
type Service struct {
	reads *query.Service
	ttl   time.Duration

	sf    singleflight.Group
	mu    sync.RWMutex
	cache map[string]jszCacheEntry
}

type jszCacheEntry struct {
	raw       []byte
	parsed    *JSZTopologyPayload
	captured  time.Time
	expiresAt time.Time
}

// NewService builds a monitoring insight service.
// reads may be nil (FetchJSZ will fail until SetReads is called).
// ttl <= 0 uses DefaultCacheTTL.
func NewService(reads *query.Service, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	return &Service{
		reads: reads,
		ttl:   ttl,
		cache: make(map[string]jszCacheEntry),
	}
}

// NewServiceFromGateway is a convenience constructor used by app.NewServices.
func NewServiceFromGateway(gateway query.ExecutorGetter, hub *snapshot.Hub, maxBodyBytes int64, ttl time.Duration) *Service {
	return NewService(query.NewService(gateway, hub, maxBodyBytes), ttl)
}

// SetReads replaces the underlying query read service (bootstrap wiring).
func (s *Service) SetReads(reads *query.Service) {
	if s == nil {
		return
	}
	s.reads = reads
}

// SetHub forwards hub wiring to the underlying query service.
func (s *Service) SetHub(hub *snapshot.Hub) {
	if s == nil || s.reads == nil {
		return
	}
	s.reads.SetHub(hub)
}

// SetMaxBodyBytes forwards size-limit wiring to the underlying query service.
func (s *Service) SetMaxBodyBytes(n int64) {
	if s == nil || s.reads == nil {
		return
	}
	s.reads.SetMaxBodyBytes(n)
}

// SetCacheTTL updates the in-process JSZ cache TTL (0 or negative restores default).
func (s *Service) SetCacheTTL(ttl time.Duration) {
	if s == nil {
		return
	}
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	s.ttl = ttl
}

// Reads returns the underlying CQRS-lite query service (may be nil).
func (s *Service) Reads() *query.Service {
	if s == nil {
		return nil
	}
	return s.reads
}

// FetchJSZ returns topology-path /jsz bytes for a cluster.
// When fresh is false, prefers the short TTL cache, then snapshot hub, then live executor.
func (s *Service) FetchJSZ(ctx context.Context, clusterID string, fresh bool) ([]byte, time.Time, error) {
	if s == nil {
		return nil, time.Time{}, errors.New("monitoring service unavailable")
	}
	if s.reads == nil {
		return nil, time.Time{}, errors.New("monitoring reads unavailable")
	}
	if !fresh {
		if raw, at, ok := s.getCached(clusterID); ok {
			return raw, at, nil
		}
	}

	key := clusterID
	if fresh {
		key = clusterID + "|fresh"
	}
	v, err, _ := s.sf.Do(key, func() (any, error) {
		if !fresh {
			if raw, at, ok := s.getCached(clusterID); ok {
				return jszFetchResult{raw: raw, at: at}, nil
			}
		}
		raw, at, err := s.reads.FetchMonitoring(ctx, clusterID, snapshot.TopologyJSZPath, fresh)
		if err != nil {
			return nil, err
		}
		// Copy before caching so callers cannot mutate the hub's shared slice via cache.
		owned := append([]byte(nil), raw...)
		s.putCached(clusterID, owned, nil, at)
		return jszFetchResult{raw: owned, at: at}, nil
	})
	if err != nil {
		return nil, time.Time{}, err
	}
	res := v.(jszFetchResult)
	return res.raw, res.at, nil
}

// FetchAndParse returns raw bytes and a parsed payload, filling the parsed cache slot when possible.
func (s *Service) FetchAndParse(ctx context.Context, clusterID string, fresh bool) ([]byte, JSZTopologyPayload, time.Time, error) {
	raw, at, err := s.FetchJSZ(ctx, clusterID, fresh)
	if err != nil {
		return nil, JSZTopologyPayload{}, time.Time{}, err
	}
	if !fresh {
		if payload, ok := s.getCachedParsed(clusterID); ok {
			return raw, payload, at, nil
		}
	}
	payload, err := ParsePayload(raw)
	if err != nil {
		return raw, JSZTopologyPayload{}, at, err
	}
	s.putCached(clusterID, raw, &payload, at)
	return raw, payload, at, nil
}

type jszFetchResult struct {
	raw []byte
	at  time.Time
}

func (s *Service) getCached(clusterID string) ([]byte, time.Time, bool) {
	s.mu.RLock()
	entry, ok := s.cache[clusterID]
	s.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) || len(entry.raw) == 0 {
		if ok {
			s.mu.Lock()
			delete(s.cache, clusterID)
			s.mu.Unlock()
		}
		return nil, time.Time{}, false
	}
	return entry.raw, entry.captured, true
}

func (s *Service) getCachedParsed(clusterID string) (JSZTopologyPayload, bool) {
	s.mu.RLock()
	entry, ok := s.cache[clusterID]
	s.mu.RUnlock()
	if !ok || entry.parsed == nil || time.Now().After(entry.expiresAt) {
		return JSZTopologyPayload{}, false
	}
	return *entry.parsed, true
}

func (s *Service) putCached(clusterID string, raw []byte, parsed *JSZTopologyPayload, captured time.Time) {
	if s == nil || clusterID == "" || len(raw) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.cache[clusterID]
	entry.raw = raw
	entry.captured = captured
	entry.expiresAt = time.Now().Add(s.ttl)
	if parsed != nil {
		cp := *parsed
		entry.parsed = &cp
	}
	s.cache[clusterID] = entry
	if len(s.cache) > 1024 {
		now := time.Now()
		for k, e := range s.cache {
			if now.After(e.expiresAt) {
				delete(s.cache, k)
			}
		}
	}
}

// InvalidateCluster drops the cached JSZ entry for a cluster.
func (s *Service) InvalidateCluster(clusterID string) {
	if s == nil || clusterID == "" {
		return
	}
	s.mu.Lock()
	delete(s.cache, clusterID)
	s.mu.Unlock()
}
