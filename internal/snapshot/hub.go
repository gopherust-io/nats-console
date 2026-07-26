package snapshot

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gopherust-io/nats-consol/internal/metrics"
)

// TopologyJSZPath is the monitoring query used for topology / rich dashboard views.
const TopologyJSZPath = "/jsz?streams=1&consumers=1&config=1"

// SnapshotEvent notifies listeners that a cluster snapshot was refreshed.
type SnapshotEvent struct {
	CapturedAt time.Time
	ClusterID  string
}

// ClusterSnapshot holds the last scraped monitoring payloads for a cluster.
type ClusterSnapshot struct {
	CapturedAt  time.Time
	Varz        []byte
	Jsz         []byte
	JszTopology []byte
}

// Hub is a process-local cache of the latest monitoring snapshots per cluster.
type Hub struct {
	entries   map[string]*ClusterSnapshot
	listeners map[chan SnapshotEvent]struct{}
	seq       atomic.Uint64
	mu        sync.RWMutex
}

// NewHub creates an empty snapshot hub.
func NewHub() *Hub {
	return &Hub{
		entries:   make(map[string]*ClusterSnapshot),
		listeners: make(map[chan SnapshotEvent]struct{}),
	}
}

// Publish stores the latest snapshot for a cluster and notifies subscribers.
func (h *Hub) Publish(clusterID string, snap ClusterSnapshot) {
	if h == nil || clusterID == "" {
		return
	}
	if snap.CapturedAt.IsZero() {
		snap.CapturedAt = time.Now().UTC()
	}
	// Defensive copies so callers can reuse buffers.
	entry := &ClusterSnapshot{
		CapturedAt:  snap.CapturedAt,
		Varz:        cloneBytes(snap.Varz),
		Jsz:         cloneBytes(snap.Jsz),
		JszTopology: cloneBytes(snap.JszTopology),
	}

	h.mu.Lock()
	h.entries[clusterID] = entry
	listeners := make([]chan SnapshotEvent, 0, len(h.listeners))
	for ch := range h.listeners {
		listeners = append(listeners, ch)
	}
	h.mu.Unlock()

	h.seq.Add(1)
	ev := SnapshotEvent{ClusterID: clusterID, CapturedAt: entry.CapturedAt}
	for _, ch := range listeners {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Invalidate drops the cached snapshot for a cluster so the next scrape or
// fresh monitoring request cannot serve stale JetStream topology.
func (h *Hub) Invalidate(clusterID string) {
	if h == nil || clusterID == "" {
		return
	}
	h.mu.Lock()
	delete(h.entries, clusterID)
	h.mu.Unlock()
}

// Latest returns the last snapshot for a cluster, if any.
func (h *Hub) Latest(clusterID string) (ClusterSnapshot, bool) {
	if h == nil {
		return ClusterSnapshot{}, false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	entry, ok := h.entries[clusterID]
	if !ok || entry == nil {
		return ClusterSnapshot{}, false
	}
	return ClusterSnapshot{
		CapturedAt:  entry.CapturedAt,
		Varz:        cloneBytes(entry.Varz),
		Jsz:         cloneBytes(entry.Jsz),
		JszTopology: cloneBytes(entry.JszTopology),
	}, true
}

// MonitoringPayload returns a cached monitoring body for a normalized path, if present.
// Supported: /varz, /jsz, and topology-style /jsz?streams=1&consumers=1&config=1.
func (h *Hub) MonitoringPayload(clusterID, path string) ([]byte, time.Time, bool) {
	snap, ok := h.Latest(clusterID)
	if !ok {
		return nil, time.Time{}, false
	}
	switch normalizeMonitoringPath(path) {
	case "/varz":
		if len(snap.Varz) == 0 {
			return nil, time.Time{}, false
		}
		metrics.IncSnapshotHubHit("varz")
		return snap.Varz, snap.CapturedAt, true
	case "/jsz":
		if len(snap.Jsz) == 0 {
			return nil, time.Time{}, false
		}
		metrics.IncSnapshotHubHit("jsz")
		return snap.Jsz, snap.CapturedAt, true
	case TopologyJSZPath:
		if len(snap.JszTopology) == 0 {
			return nil, time.Time{}, false
		}
		metrics.IncSnapshotHubHit("jsz_topology")
		return snap.JszTopology, snap.CapturedAt, true
	default:
		metrics.IncSnapshotHubMiss(path)
		return nil, time.Time{}, false
	}
}

// Subscribe receives snapshot refresh events. Buffer is small; slow consumers drop.
func (h *Hub) Subscribe(buffer int) (<-chan SnapshotEvent, func()) {
	if h == nil {
		ch := make(chan SnapshotEvent)
		close(ch)
		return ch, func() {}
	}
	if buffer < 1 {
		buffer = 8
	}
	ch := make(chan SnapshotEvent, buffer)
	h.mu.Lock()
	h.listeners[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.listeners, ch)
		h.mu.Unlock()
		close(ch)
	}
}

// Seq returns a monotonically increasing publish counter (for ETag / diagnostics).
func (h *Hub) Seq() uint64 {
	if h == nil {
		return 0
	}
	return h.seq.Load()
}

func cloneBytes(in []byte) []byte {
	if len(in) == 0 {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}

func normalizeMonitoringPath(path string) string {
	if path == "" {
		return ""
	}
	// Strip leading spaces; keep query order as provided by callers.
	if path[0] != '/' {
		path = "/" + path
	}
	switch path {
	case "/varz", "/jsz", TopologyJSZPath:
		return path
	default:
		// Accept equivalent topology query orderings.
		if isTopologyJSZ(path) {
			return TopologyJSZPath
		}
		if path == "/jsz?" || len(path) >= 5 && path[:5] == "/jsz?" {
			// Bare /jsz with unknown query — not hub-cached as summary.
			return path
		}
		return path
	}
}

func isTopologyJSZ(path string) bool {
	if len(path) < 5 || path[:5] != "/jsz?" {
		return false
	}
	q := path[5:]
	return containsQueryFlag(q, "streams=1") &&
		containsQueryFlag(q, "consumers=1") &&
		containsQueryFlag(q, "config=1")
}

func containsQueryFlag(query, flag string) bool {
	if query == flag {
		return true
	}
	if len(query) < len(flag) {
		return false
	}
	// Check &flag& / start / end.
	for i := 0; i+len(flag) <= len(query); i++ {
		if query[i:i+len(flag)] != flag {
			continue
		}
		beforeOK := i == 0 || query[i-1] == '&'
		afterOK := i+len(flag) == len(query) || query[i+len(flag)] == '&'
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}
