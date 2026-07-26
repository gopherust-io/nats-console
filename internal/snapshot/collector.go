package snapshot

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/gopherust-io/nats-consol/internal/alerter"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/mail"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/tel"
)

// Collector scrapes NATS monitoring endpoints, stores normalized samples, and
// publishes raw payloads to the in-process Hub for UI/API reuse.
//
//nolint:govet // fieldalignment: config.Config is intentionally embedded by value
type Collector struct {
	mailer  mail.Sender
	store   *store.Store
	manager *natsclient.Manager
	hub     *Hub
	stop    chan struct{}
	done    chan struct{}
	cfg     config.Config
}

func Start(st *store.Store, manager *natsclient.Manager, cfg config.Config, mailer mail.Sender, hub *Hub) *Collector {
	if !cfg.MetricsSnapshotActive() {
		return nil
	}
	if mailer == nil {
		mailer = mail.NopSender{}
	}
	if hub == nil {
		hub = NewHub()
	}
	c := &Collector{
		store:   st,
		manager: manager,
		hub:     hub,
		cfg:     cfg,
		mailer:  mailer,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	go c.loop()
	go c.cleanupLoop()
	return c
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
	<-c.done
}

func (c *Collector) loop() {
	defer close(c.done)
	c.sample()

	ticker := time.NewTicker(c.cfg.SnapshotInterval())
	defer ticker.Stop()

	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			c.sample()
		}
	}
}

func (c *Collector) cleanupLoop() {
	ticker := time.NewTicker(c.cfg.SnapshotCleanupInterval())
	defer ticker.Stop()

	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			c.cleanup()
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

	result, err := natsclient.CollectClusterSnapshot(client, ctx)
	if err != nil {
		metrics.IncSnapshotErrors(clusterID)
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Str("cluster_id", clusterID).Msg("collect metrics failed")
		return
	}
	c.hub.Publish(clusterID, ClusterSnapshot{
		CapturedAt:  capturedAt,
		Varz:        result.Varz,
		Jsz:         result.Jsz,
		JszTopology: result.JszTopology,
	})
	if len(result.Samples) == 0 {
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
}

func (c *Collector) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.RequestTimeout)
	defer cancel()

	retention := c.cfg.SnapshotRetention()
	if retention <= 0 {
		return
	}
	cutoff := time.Now().UTC().Add(-retention)
	deleted, err := c.store.DeleteMetricSamplesOlderThan(ctx, cutoff)
	if err != nil {
		tel.Warn().Err(err).Str("component", "metrics_snapshot").Msg("cleanup failed")
		return
	}
	if deleted > 0 {
		tel.Info().Int64("deleted", deleted).Str("component", "metrics_snapshot").Msg("purged old samples")
	}
}
