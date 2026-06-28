package snapshot

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/log"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/store"
)

// Collector scrapes NATS monitoring endpoints and stores normalized samples.
//
//nolint:govet // fieldalignment: config.Config is intentionally embedded by value
type Collector struct {
	cfg     config.Config
	store   *store.Store
	manager *natsclient.Manager
	stop    chan struct{}
	done    chan struct{}
}

func Start(st *store.Store, manager *natsclient.Manager, cfg config.Config) *Collector {
	if !cfg.MetricsSnapshotActive() {
		return nil
	}
	c := &Collector{
		store:   st,
		manager: manager,
		cfg:     cfg,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	go c.loop()
	go c.cleanupLoop()
	return c
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
		log.Error().Err(err).Str("component", "metrics_snapshot").Msg("list clusters failed")
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
		log.Warn().Err(err).Str("component", "metrics_snapshot").Str("cluster_id", clusterID).Msg("get client failed")
		return
	}

	samples, err := natsclient.CollectClusterMetrics(client, ctx)
	if err != nil {
		metrics.IncSnapshotErrors(clusterID)
		log.Warn().Err(err).Str("component", "metrics_snapshot").Str("cluster_id", clusterID).Msg("collect metrics failed")
		return
	}
	if len(samples) == 0 {
		return
	}
	if err := c.store.InsertMetricSamples(ctx, clusterID, capturedAt, samples); err != nil {
		metrics.IncSnapshotErrors(clusterID)
		log.Warn().Err(err).Str("component", "metrics_snapshot").Str("cluster_id", clusterID).Msg("insert samples failed")
		return
	}
	metrics.IncSnapshotSuccess(clusterID)
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
		log.Warn().Err(err).Str("component", "metrics_snapshot").Msg("cleanup failed")
		return
	}
	if deleted > 0 {
		log.Info().Int64("deleted", deleted).Str("component", "metrics_snapshot").Msg("purged old samples")
	}
}
