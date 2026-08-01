package snapshot

import (
	"context"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/gopherust-io/tel"

	"github.com/gopherust-io/nats-consol/internal/alerter"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/mail"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/store"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/pkg/common/safe"
)

type streamCounters struct {
	bytes   float64
	lastSeq float64
}

// Collector scrapes NATS monitoring endpoints, stores normalized samples, and
// publishes raw payloads to the in-process Hub for UI/API reuse.
//
//nolint:govet // fieldalignment: config.Config is intentionally embedded by value
type Collector struct {
	mailer     mail.Sender
	store      *store.Store
	manager    *natsclient.Manager
	hub        *Hub
	stop       chan struct{}
	wg         sync.WaitGroup
	cfg        config.Config
	routeMu    sync.Mutex
	lastRoutes map[string][]string
	streamMu   sync.Mutex
	lastStream map[string]map[string]streamCounters // clusterID -> stream -> counters
	fpMu       sync.Mutex
	fpCache    map[string]fpCacheEntry // cluster/stream/consumer -> processing ms
}

type fpCacheEntry struct {
	at time.Time
	ms float64
}

const fingerprintCacheTTL = 10 * time.Minute

func Start(st *store.Store, manager *natsclient.Manager, cfg config.Config, mailer mail.Sender, hub *Hub) (*Collector, func()) {
	if !cfg.MetricsSnapshot.Enabled {
		return nil, nil
	}
	if mailer == nil {
		mailer = mail.NopSender{}
	}
	if hub == nil {
		hub = NewHub()
	}
	c := &Collector{
		store:      st,
		manager:    manager,
		hub:        hub,
		cfg:        cfg,
		mailer:     mailer,
		stop:       make(chan struct{}),
		lastRoutes: make(map[string][]string),
		lastStream: make(map[string]map[string]streamCounters),
		fpCache:    make(map[string]fpCacheEntry),
	}
	c.wg.Add(2)
	go c.loop()
	go c.cleanupLoop()
	return c, c.Stop
}

func (c *Collector) Hub() *Hub {
	if c == nil {
		return nil
	}
	return c.hub
}

func (c *Collector) Stop() {
	if c == nil {
		return
	}
	close(c.stop)
	c.wg.Wait()
}

func (c *Collector) loop() {
	defer c.wg.Done()
	safe.Run("metrics_snapshot", c.sample)

	ticker := time.NewTicker(c.cfg.MetricsSnapshot.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			safe.Run("metrics_snapshot", c.sample)
		}
	}
}

func (c *Collector) cleanupLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.cfg.MetricsSnapshot.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			safe.Run("metrics_snapshot", c.cleanup)
		}
	}
}

func (c *Collector) sample() {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.RequestTimeout)
	defer cancel()

	clusters, err := c.store.ListClusters(ctx)
	if err != nil {
		tel.Error().Err(err).Str("component", "metrics_snapshot").Msg("list clusters failed")
		return
	}

	capturedAt := time.Now().UTC()
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(6)
	for _, cluster := range clusters {
		clusterID := cluster.ID
		group.Go(func() error {
			c.sampleCluster(groupCtx, clusterID, capturedAt)
			return nil
		})
	}
	_ = group.Wait()
}

func (c *Collector) sampleCluster(ctx context.Context, clusterID string, capturedAt time.Time) {
	client, err := c.manager.Get(ctx, clusterID)
	if err != nil {
		metrics.IncSnapshotErrors(clusterID)
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Str("cluster_id", clusterID).Msg("get client failed")
		return
	}

	result, err := natsclient.CollectClusterSnapshot(client, ctx, domain.SlowConsumerThresholds{
		PendingThreshold: c.cfg.SlowConsumer.PendingThreshold,
		LagThreshold:     c.cfg.SlowConsumer.LagThreshold,
		AckPendingRatio:  c.cfg.SlowConsumer.AckPendingRatio,
	})
	if err != nil {
		metrics.IncSnapshotErrors(clusterID)
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Str("cluster_id", clusterID).Msg("collect metrics failed")
		return
	}

	c.hub.PublishTakesOwnership(clusterID, ClusterSnapshot{
		CapturedAt:  capturedAt,
		Varz:        result.Varz,
		Jsz:         result.Jsz,
		JszTopology: result.JszTopology,
		Connz:       result.Connz,
	})

	if len(result.ConsumerSamples) > 0 {
		if err := c.store.InsertIncidentConsumerSamples(ctx, clusterID, capturedAt, result.ConsumerSamples); err != nil {
			tel.Warn().Err(err).Str("component", "metrics_snapshot").Str("cluster_id", clusterID).Msg("insert incident consumer samples failed")
		}
	}
	c.persistRouteTransitions(ctx, clusterID, capturedAt, result.RouteNodes)

	payloadByStream := c.appendAvgPayloadSamples(clusterID, &result.Samples)

	if len(result.Samples) == 0 {
		c.upsertBottleneckRollups(ctx, client, clusterID, capturedAt, result.ConsumerSamples, payloadByStream)
		c.upsertArchitectureScore(ctx, clusterID, capturedAt, result.ArchitectureInputs, result.ConsumerSamples)
		return
	}
	if err := c.store.InsertMetricSamples(ctx, clusterID, capturedAt, result.Samples); err != nil {
		metrics.IncSnapshotErrors(clusterID)
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Str("cluster_id", clusterID).Msg("insert samples failed")
		return
	}

	metrics.IncSnapshotSuccess(clusterID)
	domainSamples := make([]domain.MetricSample, len(result.Samples))
	for i, sample := range result.Samples {
		domainSamples[i] = domain.MetricSample{Metric: sample.Metric, Value: sample.Value}
	}
	alerter.Evaluate(ctx, c.store, clusterID, domainSamples, alerter.Options{
		Mailer:        c.mailer,
		PublicBaseURL: c.cfg.PublicBaseURL,
	})

	c.upsertBottleneckRollups(ctx, client, clusterID, capturedAt, result.ConsumerSamples, payloadByStream)
	c.upsertArchitectureScore(ctx, clusterID, capturedAt, result.ArchitectureInputs, result.ConsumerSamples)
}

func (c *Collector) upsertArchitectureScore(
	ctx context.Context,
	clusterID string,
	capturedAt time.Time,
	inputs []domain.EventArchitectureInput,
	consumers []domain.IncidentConsumerSample,
) {
	if len(inputs) == 0 {
		return
	}
	hints := domain.ArchitectureScoreHints{}
	if avg, ok := domain.AverageConsumerLag(consumers); ok {
		hints.AvgLag = avg
		hints.HasLag = true
	}
	priorDay := capturedAt.UTC().AddDate(0, 0, -1)
	if prior, ok, gerr := c.store.GetArchitectureScoreDaily(ctx, clusterID, priorDay); gerr == nil && ok {
		hints.Prior = &domain.ArchitectureScorePrior{
			Score:  prior.Score,
			AvgLag: prior.AvgLag,
			HasLag: prior.AvgLag > 0,
		}
	}
	snap := domain.ComputeArchitectureScore(inputs, hints)
	row := domain.ArchitectureScoreDailyRow{
		ClusterID:  clusterID,
		ScoreDay:   capturedAt.UTC(),
		Score:      snap.Score,
		Factors:    snap.Factors,
		AvgLag:     hints.AvgLag,
		CapturedAt: capturedAt.UTC(),
	}
	if err := c.store.UpsertArchitectureScoreDaily(ctx, row); err != nil {
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Str("cluster_id", clusterID).Msg("upsert architecture score failed")
	}
}

// appendAvgPayloadSamples derives stream:{name}:avg_payload_bytes from consecutive scrapes.
// Returns current-scrape avg payload by stream (empty when first scrape / no delta).
func (c *Collector) appendAvgPayloadSamples(clusterID string, samples *[]store.MetricSampleRow) map[string]float64 {
	current := map[string]streamCounters{}
	for _, s := range *samples {
		stream, kind, ok := domain.ParseStreamMetric(s.Metric)
		if !ok {
			continue
		}
		ctr := current[stream]
		switch kind {
		case domain.StreamMetricKindBytes:
			ctr.bytes = s.Value
		case domain.StreamMetricKindLastSeq:
			ctr.lastSeq = s.Value
		}
		current[stream] = ctr
	}

	c.streamMu.Lock()
	prev, seen := c.lastStream[clusterID]
	c.lastStream[clusterID] = current
	c.streamMu.Unlock()

	out := map[string]float64{}
	if !seen {
		return out
	}
	for stream, cur := range current {
		p, ok := prev[stream]
		if !ok {
			continue
		}
		deltaBytes := cur.bytes - p.bytes
		deltaMsgs := cur.lastSeq - p.lastSeq
		avg, ok := domain.AvgPayloadBytes(deltaBytes, deltaMsgs)
		if !ok {
			continue
		}
		out[stream] = avg
		*samples = append(*samples, store.MetricSampleRow{
			Metric: domain.StreamMetric(stream, domain.StreamMetricKindAvgPayloadBytes),
			Value:  avg,
		})
	}
	return out
}

func (c *Collector) upsertBottleneckRollups(
	ctx context.Context,
	client *natsclient.Client,
	clusterID string,
	capturedAt time.Time,
	consumers []domain.IncidentConsumerSample,
	payloadByStream map[string]float64,
) {
	bucketHour := capturedAt.UTC().Truncate(time.Hour)
	processing := c.loadFingerprintProcessing(ctx, client, clusterID, consumers)

	rows := make([]domain.BottleneckHourBucket, 0, len(consumers)+len(payloadByStream))
	for _, cs := range consumers {
		row := domain.BottleneckHourBucket{
			ClusterID:       clusterID,
			StreamName:      cs.StreamName,
			ConsumerName:    cs.ConsumerName,
			BucketHour:      bucketHour,
			AvgLag:          cs.Lag,
			MaxLag:          cs.Lag,
			AvgPayloadBytes: payloadByStream[cs.StreamName],
			Samples:         1,
		}
		if ms, ok := processing[cs.StreamName+"/"+cs.ConsumerName]; ok {
			row.AvgProcessingMs = &ms
		}
		rows = append(rows, row)
	}
	for stream, avg := range payloadByStream {
		rows = append(rows, domain.BottleneckHourBucket{
			ClusterID:       clusterID,
			StreamName:      stream,
			ConsumerName:    "",
			BucketHour:      bucketHour,
			AvgPayloadBytes: avg,
			Samples:         1,
		})
	}
	if len(rows) == 0 {
		return
	}
	if err := c.store.UpsertBottleneckHourBuckets(ctx, rows); err != nil {
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Str("cluster_id", clusterID).Msg("upsert bottleneck rollups failed")
	}
}

func (c *Collector) loadFingerprintProcessing(
	ctx context.Context,
	client *natsclient.Client,
	clusterID string,
	consumers []domain.IncidentConsumerSample,
) map[string]float64 {
	out := map[string]float64{}
	if client == nil || len(consumers) == 0 {
		return out
	}
	bucket := strings.TrimSpace(c.cfg.BehaviorFingerprintKVBucket)
	if commonstrings.IsEmpty(bucket) {
		bucket = domain.DefaultBehaviorFingerprintKVBucket
	}

	now := time.Now()
	var toFetch []domain.IncidentConsumerSample
	c.fpMu.Lock()
	for _, cs := range consumers {
		if cs.Lag <= 0 {
			continue
		}
		key := clusterID + "/" + cs.StreamName + "/" + cs.ConsumerName
		if entry, ok := c.fpCache[key]; ok && now.Sub(entry.at) < fingerprintCacheTTL {
			out[cs.StreamName+"/"+cs.ConsumerName] = entry.ms
			continue
		}
		toFetch = append(toFetch, cs)
	}
	c.fpMu.Unlock()

	if len(toFetch) == 0 {
		return out
	}

	const fingerprintFetchLimit = 8
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(fingerprintFetchLimit)
	for _, cs := range toFetch {
		g.Go(func() error {
			key := domain.BehaviorFingerprintKVKey(cs.StreamName, cs.ConsumerName)
			entry, err := client.GetKVEntry(gctx, bucket, key)
			if err != nil || entry == nil || commonstrings.IsEmpty(entry.Value) {
				return nil
			}
			raw, err := base64.StdEncoding.DecodeString(entry.Value)
			if err != nil {
				return nil
			}
			report := domain.ParseBehaviorFingerprintKV(raw, cs.StreamName, cs.ConsumerName)
			if !report.Available || report.Current == nil {
				return nil
			}
			ms := report.Current.ProcessingMs
			cacheKey := clusterID + "/" + cs.StreamName + "/" + cs.ConsumerName
			mu.Lock()
			out[cs.StreamName+"/"+cs.ConsumerName] = ms
			mu.Unlock()
			c.fpMu.Lock()
			c.fpCache[cacheKey] = fpCacheEntry{at: now, ms: ms}
			c.fpMu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
	return out
}

func (c *Collector) persistRouteTransitions(ctx context.Context, clusterID string, capturedAt time.Time, current []string) {
	c.routeMu.Lock()
	previous, seenBefore := c.lastRoutes[clusterID]
	c.lastRoutes[clusterID] = append([]string(nil), current...)
	c.routeMu.Unlock()

	if !seenBefore {
		// First scrape establishes baseline; avoid false disconnect storms.
		return
	}
	disconnected, reconnected := natsclient.DiffRouteNodes(previous, current)
	events := make([]domain.IncidentNodeEvent, 0, len(disconnected)+len(reconnected))
	for _, name := range disconnected {
		events = append(events, domain.IncidentNodeEvent{
			OccurredAt: capturedAt,
			NodeName:   name,
			EventType:  domain.IncidentNodeDisconnect,
		})
	}
	for _, name := range reconnected {
		events = append(events, domain.IncidentNodeEvent{
			OccurredAt: capturedAt,
			NodeName:   name,
			EventType:  domain.IncidentNodeReconnect,
		})
	}
	if len(events) == 0 {
		return
	}
	if err := c.store.InsertIncidentNodeEvents(ctx, clusterID, events); err != nil {
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Str("cluster_id", clusterID).Msg("insert incident node events failed")
	}
}

func (c *Collector) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.RequestTimeout)
	defer cancel()

	retention := c.cfg.MetricsSnapshot.Retention
	if retention <= 0 {
		return
	}
	now := time.Now().UTC()
	if err := c.store.EnsureSamplePartitions(ctx, now, retention); err != nil {
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Msg("ensure sample partitions failed")
	}
	cutoff := now.Add(-retention)
	deleted, err := c.store.DeleteMetricSamplesOlderThan(ctx, cutoff)
	if err != nil {
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Msg("cleanup failed")
		return
	}
	incidentDeleted, err := c.store.DeleteIncidentDataOlderThan(ctx, cutoff)
	if err != nil {
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Msg("incident cleanup failed")
	} else {
		deleted += incidentDeleted
	}
	auditDeleted, err := c.store.DeleteAuditOlderThan(ctx, cutoff)
	if err != nil {
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Msg("audit cleanup failed")
	} else {
		deleted += auditDeleted
	}
	alertsDeleted, err := c.store.DeleteClosedAlertsOlderThan(ctx, cutoff)
	if err != nil {
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Msg("closed alerts cleanup failed")
	} else {
		deleted += alertsDeleted
	}

	bottleneckRetention := c.cfg.MetricsSnapshot.BottleneckRetention
	if bottleneckRetention <= 0 {
		bottleneckRetention = 672 * time.Hour
	}
	bottleneckDeleted, err := c.store.DeleteBottleneckHourBucketsOlderThan(ctx, now.Add(-bottleneckRetention))
	if err != nil {
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Msg("bottleneck rollup cleanup failed")
	} else {
		deleted += bottleneckDeleted
	}

	const architectureScoreRetention = 180 * 24 * time.Hour
	scoreDeleted, err := c.store.DeleteArchitectureScoreDailyOlderThan(ctx, now.Add(-architectureScoreRetention))
	if err != nil {
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Msg("architecture score cleanup failed")
	} else {
		deleted += scoreDeleted
	}

	if deleted > 0 {
		tel.Info().Int64("deleted", deleted).Str("component", "metrics_snapshot").Msg("purged old samples")
	}
}
