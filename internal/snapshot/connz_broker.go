package snapshot

import (
	"context"
	"crypto/sha256"
	"sync"
	"time"

	"github.com/gopherust-io/nats-consol/pkg/common/safe"
)

const (
	// ConnzAuthPath is the monitoring query used by the Connections UI.
	ConnzAuthPath = "/connz?limit=1024&auth=1"

	// DefaultConnzInterval is how often the broker scrapes while subscribers exist.
	DefaultConnzInterval = 5 * time.Second

	// DefaultReplicasInterval is how often the replicas SSE broker scrapes while
	// the Replicas page is open.
	DefaultReplicasInterval = 5 * time.Second

	// DefaultReplicasScrapeTimeout is the per-tick budget for multi-peer
	// varz/routez/jsz failover (must exceed DefaultReplicasInterval).
	DefaultReplicasScrapeTimeout = 12 * time.Second

	connzSubscriberBuffer = 4
)

// ConnzFetcher loads a connz monitoring payload for a cluster.
type ConnzFetcher func(ctx context.Context, clusterID string) ([]byte, error)

// ConnzBroker scrapes connz only while at least one subscriber is attached per cluster.
type ConnzBroker struct {
	fetch         ConnzFetcher
	clusters      map[string]*connzCluster
	interval      time.Duration
	scrapeTimeout time.Duration
	mu            sync.Mutex
}

type connzCluster struct {
	listeners map[chan []byte]struct{}
	stop      chan struct{}
	latest    []byte
	latestSHA [32]byte
	hasSHA    bool
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

// Subscribe attaches a listener for clusterID. The first subscriber starts the scrape loop;
// the returned unsubscribe stops it when the last listener leaves.
// latest is a clone of the most recent payload (may be nil on first subscribe).
func (b *ConnzBroker) Subscribe(clusterID string) (updates <-chan []byte, latest []byte, unsubscribe func()) {
	if b == nil || b.fetch == nil || clusterID == "" {
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
	latest = cloneBytes(entry.latest)
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

	safe.Run("connz_broker", func() { b.scrape(clusterID, entry) })

	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-entry.stop:
			return
		case <-ticker.C:
			safe.Run("connz_broker", func() { b.scrape(clusterID, entry) })
		}
	}
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
		return
	}
	sum := sha256.Sum256(data)

	b.mu.Lock()
	if b.clusters[clusterID] != entry {
		b.mu.Unlock()
		return
	}
	if entry.hasSHA && entry.latestSHA == sum {
		b.mu.Unlock()
		return
	}
	// Take ownership of fetch buffer when possible; clone so callers cannot mutate shared latest.
	payload := cloneBytes(data)
	entry.latest = payload
	entry.latestSHA = sum
	entry.hasSHA = true
	listeners := make([]chan []byte, 0, len(entry.listeners))
	for ch := range entry.listeners {
		listeners = append(listeners, ch)
	}
	b.mu.Unlock()

	// Fan out the immutable shared latest; subscribers must not mutate.
	for _, ch := range listeners {
		select {
		case ch <- payload:
		default:
			// Slow consumer: drop; next scrape will retry.
		}
	}
}
