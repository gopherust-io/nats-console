package snapshot

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// RequestReplyConnzPath is the monitoring query used for request/reply discovery.
const RequestReplyConnzPath = "/connz?limit=1024&subs=1"

// TopologyJSZPath is the monitoring query used for topology / rich dashboard views.
const TopologyJSZPath = "/jsz?streams=1&consumers=1&config=1"

// SnapshotEvent notifies listeners that a cluster snapshot was refreshed.
type SnapshotEvent struct {
	CapturedAt time.Time
	ClusterID  string
}

// ClusterSnapshot holds the last scraped monitoring payloads for a cluster.
type ClusterSnapshot struct {
	CapturedAt   time.Time
	Varz         []byte
	Jsz          []byte
	JszTopology  []byte
	Connz        []byte
	ProbeResults []domain.RequestReplyProbeResult
}

// Hub is a process-local cache of the latest monitoring snapshots per cluster.
//
// goalign:ignore // seq+mu false-share notes are accepted for this hot cache hub
type Hub struct {
	seq       atomic.Uint64
	entries   map[string]*ClusterSnapshot
	listeners map[string]map[chan SnapshotEvent]struct{} // per-cluster
	mu        sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		entries:   make(map[string]*ClusterSnapshot),
		listeners: make(map[string]map[chan SnapshotEvent]struct{}),
	}
}

// Publish stores the latest snapshot for a cluster and notifies subscribers.
// Callers retain ownership of snap buffers; Publish clones defensively.
func (h *Hub) Publish(clusterID string, snap ClusterSnapshot) {
	h.publish(clusterID, snap, false)
}

// PublishTakesOwnership stores snap without cloning byte fields. Caller must
// not mutate or reuse Varz/Jsz/JszTopology/Connz after this returns.
func (h *Hub) PublishTakesOwnership(clusterID string, snap ClusterSnapshot) {
	h.publish(clusterID, snap, true)
}

func (h *Hub) publish(clusterID string, snap ClusterSnapshot, takeOwnership bool) {
	if h == nil || strings.IsEmpty(clusterID) {
		return
	}
	if snap.CapturedAt.IsZero() {
		snap.CapturedAt = time.Now().UTC()
	}
	entry := &ClusterSnapshot{
		CapturedAt:   snap.CapturedAt,
		ProbeResults: cloneProbeResults(snap.ProbeResults),
	}
	if takeOwnership {
		entry.Varz = snap.Varz
		entry.Jsz = snap.Jsz
		entry.JszTopology = snap.JszTopology
		entry.Connz = snap.Connz
	} else {
		entry.Varz = cloneBytes(snap.Varz)
		entry.Jsz = cloneBytes(snap.Jsz)
		entry.JszTopology = cloneBytes(snap.JszTopology)
		entry.Connz = cloneBytes(snap.Connz)
	}

	h.mu.Lock()
	h.entries[clusterID] = entry
	var listeners []chan SnapshotEvent
	if set := h.listeners[clusterID]; len(set) > 0 {
		listeners = make([]chan SnapshotEvent, 0, len(set))
		for ch := range set {
			listeners = append(listeners, ch)
		}
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
	if h == nil || strings.IsEmpty(clusterID) {
		return
	}
	h.mu.Lock()
	delete(h.entries, clusterID)
	h.mu.Unlock()
}

// Latest returns the last snapshot for a cluster, if any.
// Prefer MonitoringPayload / ProbeResultsOverlay for hot paths.
func (h *Hub) Latest(clusterID string) (ClusterSnapshot, bool) {
	if h == nil {
		return ClusterSnapshot{}, false
	}
	h.mu.RLock()
	entry, ok := h.entries[clusterID]
	if !ok || entry == nil {
		h.mu.RUnlock()
		return ClusterSnapshot{}, false
	}
	// Copy pointers under lock; clone after unlock to avoid holding RLock during memcpy.
	rawVarz := entry.Varz
	rawJsz := entry.Jsz
	rawTopo := entry.JszTopology
	rawConnz := entry.Connz
	probes := entry.ProbeResults
	captured := entry.CapturedAt
	h.mu.RUnlock()

	return ClusterSnapshot{
		CapturedAt:   captured,
		Varz:         cloneBytes(rawVarz),
		Jsz:          cloneBytes(rawJsz),
		JszTopology:  cloneBytes(rawTopo),
		Connz:        cloneBytes(rawConnz),
		ProbeResults: cloneProbeResults(probes),
	}, true
}

// MonitoringPayload returns a cached monitoring body for a normalized path, if present.
// Supported: /varz, /jsz, and topology-style /jsz?streams=1&consumers=1&config=1.
// Only the requested field is cloned (not the whole snapshot).
func (h *Hub) MonitoringPayload(clusterID, path string) ([]byte, time.Time, bool) {
	if h == nil {
		return nil, time.Time{}, false
	}
	h.mu.RLock()
	entry, ok := h.entries[clusterID]
	if !ok || entry == nil {
		h.mu.RUnlock()
		metrics.IncSnapshotHubMiss(path)
		return nil, time.Time{}, false
	}
	var raw []byte
	var hit string
	switch normalizeMonitoringPath(path) {
	case "/varz":
		raw, hit = entry.Varz, "varz"
	case "/jsz":
		raw, hit = entry.Jsz, "jsz"
	case TopologyJSZPath:
		raw, hit = entry.JszTopology, "jsz_topology"
	case RequestReplyConnzPath:
		raw, hit = entry.Connz, "connz"
	default:
		h.mu.RUnlock()
		metrics.IncSnapshotHubMiss(path)
		return nil, time.Time{}, false
	}
	if len(raw) == 0 {
		h.mu.RUnlock()
		metrics.IncSnapshotHubMiss(path)
		return nil, time.Time{}, false
	}
	captured := entry.CapturedAt
	h.mu.RUnlock()
	out := cloneBytes(raw)
	metrics.IncSnapshotHubHit(hit)
	return out, captured, true
}

// ProbeResultsOverlay returns cached request/reply probe results without cloning fat payloads.
func (h *Hub) ProbeResultsOverlay(clusterID string) ([]domain.RequestReplyProbeResult, time.Time, bool) {
	if h == nil {
		return nil, time.Time{}, false
	}
	h.mu.RLock()
	entry, ok := h.entries[clusterID]
	if !ok || entry == nil {
		h.mu.RUnlock()
		return nil, time.Time{}, false
	}
	probes := entry.ProbeResults
	captured := entry.CapturedAt
	h.mu.RUnlock()
	if len(probes) == 0 {
		return nil, captured, false
	}
	return cloneProbeResults(probes), captured, true
}

// Subscribe is deprecated for global fanout; use SubscribeCluster.
// Kept as an alias that requires callers to migrate — prefer SubscribeCluster.
func (h *Hub) Subscribe(buffer int) (<-chan SnapshotEvent, func()) {
	return h.SubscribeCluster("", buffer)
}

// SubscribeCluster receives snapshot refresh events for one cluster only.
func (h *Hub) SubscribeCluster(clusterID string, buffer int) (<-chan SnapshotEvent, func()) {
	if h == nil {
		ch := make(chan SnapshotEvent)
		close(ch)
		return ch, func() {}
	}
	if buffer < 1 {
		buffer = 8
	}
	if strings.IsEmpty(clusterID) {
		ch := make(chan SnapshotEvent)
		close(ch)
		return ch, func() {}
	}
	ch := make(chan SnapshotEvent, buffer)
	h.mu.Lock()
	set := h.listeners[clusterID]
	if set == nil {
		set = make(map[chan SnapshotEvent]struct{})
		h.listeners[clusterID] = set
	}
	set[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		if set := h.listeners[clusterID]; set != nil {
			delete(set, ch)
			if len(set) == 0 {
				delete(h.listeners, clusterID)
			}
		}
		h.mu.Unlock()
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

func cloneProbeResults(in []domain.RequestReplyProbeResult) []domain.RequestReplyProbeResult {
	if len(in) == 0 {
		return nil
	}
	out := make([]domain.RequestReplyProbeResult, len(in))
	copy(out, in)
	return out
}

func normalizeMonitoringPath(path string) string {
	if strings.IsEmpty(path) {
		return ""
	}
	// Strip leading spaces; keep query order as provided by callers.
	if path[0] != '/' {
		path = "/" + path
	}
	switch path {
	case "/varz", "/jsz", TopologyJSZPath, RequestReplyConnzPath:
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
