package snapshot

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gopherust-io/nats-consol/pkg/common/fingerprint"
	"github.com/gopherust-io/nats-consol/pkg/common/safe"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const (
	// ConnzAuthPath is the monitoring query used by the Connections UI.
	ConnzAuthPath = "/connz?limit=1024&auth=1"

	// DefaultConnzInterval is how often the broker scrapes while subscribers exist.
	// Keep short so connect/disconnect shows up in the Connections UI quickly.
	DefaultConnzInterval = time.Second

	// DefaultReplicasInterval is how often the replicas SSE broker scrapes while
	// the Replicas page is open. Keep short so meta leader flips land quickly.
	DefaultReplicasInterval = 2 * time.Second

	// DefaultAccountOverviewInterval is how often the Account Overview SSE broker
	// scrapes JetStream account + request/reply while the page is open.
	DefaultAccountOverviewInterval = 2 * time.Second

	// DefaultReplicasScrapeTimeout is the minimum per-tick budget for multi-peer
	// varz/routez/jsz failover. Kept modest so a dead primary cannot stall SSE.
	DefaultReplicasScrapeTimeout = 6 * time.Second

	// maxReplicasScrapeTimeout caps one scrape so the broker loop stays responsive.
	maxReplicasScrapeTimeout = 8 * time.Second

	// replicasMonitorHopTimeout matches api.replicasMonitorHopTimeout (per-base HTTP).
	replicasMonitorHopTimeout = time.Second

	connzSubscriberBuffer = 4
)

// ReplicasScrapeTimeout returns a scrape budget for routez+jsz sequential failover
// across candidateCount monitoring bases, plus varz fan-out headroom.
func ReplicasScrapeTimeout(candidateCount int) time.Duration {
	if candidateCount < 1 {
		candidateCount = 1
	}
	// Worst case: routez failover + jsz failover (each up to N hops) + varz headroom.
	d := max(time.Duration(candidateCount*2)*replicasMonitorHopTimeout+3*time.Second, DefaultReplicasScrapeTimeout)
	if d > maxReplicasScrapeTimeout {
		return maxReplicasScrapeTimeout
	}
	return d
}

// ConnzFetcher loads a connz monitoring payload for a cluster.
type ConnzFetcher func(ctx context.Context, clusterID string) ([]byte, error)

// ConnzFetchErrorFallback builds a replacement payload when fetch fails.
// latest may be nil. Return nil to keep the previous snapshot unpublished.
type ConnzFetchErrorFallback func(clusterID string, latest []byte) []byte

// ConnzBroker scrapes connz only while at least one subscriber is attached per cluster.
type ConnzBroker struct {
	fetch               ConnzFetcher
	onFetchError        ConnzFetchErrorFallback
	clusters            map[string]*connzCluster
	interval            time.Duration
	scrapeTimeout       time.Duration
	fetchErrorThreshold int // consecutive failures before fallback; <=0 means 2
	mu                  sync.Mutex
}

type connzCluster struct {
	listeners map[chan []byte]struct{}
	stop      chan struct{}
	latest    []byte
	latestFP  uint64
	hasFP     bool
	failCount int
	scraping  atomic.Bool
	wg        sync.WaitGroup
}

// NewConnzBroker creates a demand-driven connz scraper. interval <= 0 uses DefaultConnzInterval.
// Scrape timeout defaults to the tick interval (use WithScrapeTimeout for longer budgets).
func NewConnzBroker(fetch ConnzFetcher, interval time.Duration) *ConnzBroker {
	if interval <= 0 {
		interval = DefaultConnzInterval
	}
	return &ConnzBroker{
		fetch:         fetch,
		interval:      interval,
		scrapeTimeout: interval,
		clusters:      make(map[string]*connzCluster),
	}
}

// WithScrapeTimeout sets the per-scrape context budget (must be > 0).
func (b *ConnzBroker) WithScrapeTimeout(d time.Duration) *ConnzBroker {
	if b != nil && d > 0 {
		b.scrapeTimeout = d
	}
	return b
}

// WithFetchErrorFallback sets a handler that can publish a degraded snapshot when
// fetch fails (e.g. mark all replicas offline when monitoring is unreachable).
func (b *ConnzBroker) WithFetchErrorFallback(fn ConnzFetchErrorFallback) *ConnzBroker {
	if b != nil {
		b.onFetchError = fn
	}
	return b
}

// WithFetchErrorThreshold sets how many consecutive fetch failures trigger the
// fallback (default 2). Use 1 for replicas so all-down surfaces on the first miss.
func (b *ConnzBroker) WithFetchErrorThreshold(n int) *ConnzBroker {
	if b != nil && n > 0 {
		b.fetchErrorThreshold = n
	}
	return b
}

// Subscribe attaches a listener for clusterID. The first subscriber starts the scrape loop;
// the returned unsubscribe stops it when the last listener leaves.
// latest is a shared immutable view of the most recent payload (may be nil on first subscribe).
func (b *ConnzBroker) Subscribe(clusterID string) (updates <-chan []byte, latest []byte, unsubscribe func()) {
	if b == nil || b.fetch == nil || commonstrings.IsEmpty(clusterID) {
		ch := make(chan []byte)
		close(ch)
		return ch, nil, func() {}
	}

	ch := make(chan []byte, connzSubscriberBuffer)

	b.mu.Lock()
	entry, ok := b.clusters[clusterID]
	if !ok {
		entry = &connzCluster{
			listeners: make(map[chan []byte]struct{}),
			stop:      make(chan struct{}),
		}
		b.clusters[clusterID] = entry
		entry.wg.Add(1)
		go b.loop(clusterID, entry)
	}
	entry.listeners[ch] = struct{}{}
	latest = entry.latest // immutable shared snapshot
	b.mu.Unlock()

	var once sync.Once
	unsubscribe = func() {
		once.Do(func() {
			b.unsubscribe(clusterID, ch)
		})
	}
	return ch, latest, unsubscribe
}

func (b *ConnzBroker) unsubscribe(clusterID string, ch chan []byte) {
	b.mu.Lock()
	entry, ok := b.clusters[clusterID]
	if !ok {
		b.mu.Unlock()
		return
	}
	delete(entry.listeners, ch)
	if len(entry.listeners) > 0 {
		b.mu.Unlock()
		return
	}
	delete(b.clusters, clusterID)
	stop := entry.stop
	b.mu.Unlock()

	close(stop)
	entry.wg.Wait()
}

// ActiveClusters reports how many clusters currently have scrape loops (for tests).
func (b *ConnzBroker) ActiveClusters() int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.clusters)
}

// SubscriberCount reports listeners for a cluster (for tests).
func (b *ConnzBroker) SubscriberCount(clusterID string) int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.clusters[clusterID]
	if !ok {
		return 0
	}
	return len(entry.listeners)
}

func (b *ConnzBroker) loop(clusterID string, entry *connzCluster) {
	defer entry.wg.Done()

	b.kickScrape(clusterID, entry)

	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-entry.stop:
			return
		case <-ticker.C:
			b.kickScrape(clusterID, entry)
		}
	}
}

// kickScrape starts a scrape unless one is already in flight (avoids stacking
// behind a slow failover while still allowing the loop to stay responsive).
func (b *ConnzBroker) kickScrape(clusterID string, entry *connzCluster) {
	if !entry.scraping.CompareAndSwap(false, true) {
		return
	}
	go safe.Run("connz_broker", func() {
		defer entry.scraping.Store(false)
		b.scrape(clusterID, entry)
	})
}

func (b *ConnzBroker) scrape(clusterID string, entry *connzCluster) {
	timeout := b.scrapeTimeout
	if timeout <= 0 {
		timeout = b.interval
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	data, err := b.fetch(ctx, clusterID)
	if err != nil || len(data) == 0 {
		b.publishFetchError(clusterID, entry)
		return
	}
	b.publishPayload(clusterID, entry, data, true)
}

func (b *ConnzBroker) publishFetchError(clusterID string, entry *connzCluster) {
	b.mu.Lock()
	if b.clusters[clusterID] != entry {
		b.mu.Unlock()
		return
	}
	entry.failCount++
	fails := entry.failCount
	latest := entry.latest // immutable shared; fallback must not mutate
	fallback := b.onFetchError
	b.mu.Unlock()

	// Require consecutive failures so a single blip does not flicker everyone offline
	// (default 2). Replicas uses threshold 1 for faster all-down.
	threshold := b.fetchErrorThreshold
	if threshold <= 0 {
		threshold = 2
	}
	if fails < threshold || fallback == nil {
		return
	}
	data := fallback(clusterID, latest)
	if len(data) == 0 {
		return
	}
	b.publishPayload(clusterID, entry, data, false)
}

func (b *ConnzBroker) publishPayload(clusterID string, entry *connzCluster, data []byte, resetFails bool) {
	fp := fingerprint.Sum64(data)

	b.mu.Lock()
	if b.clusters[clusterID] != entry {
		b.mu.Unlock()
		return
	}
	if resetFails {
		entry.failCount = 0
	}
	if entry.hasFP && entry.latestFP == fp {
		b.mu.Unlock()
		return
	}
	// Take ownership of data; callers must not mutate after publishPayload returns.
	entry.latest = data
	entry.latestFP = fp
	entry.hasFP = true
	listeners := make([]chan []byte, 0, len(entry.listeners))
	for ch := range entry.listeners {
		listeners = append(listeners, ch)
	}
	b.mu.Unlock()

	for _, ch := range listeners {
		select {
		case ch <- data:
		default:
			// Slow consumer: drop; next scrape will retry.
		}
	}
}
