// Package insights hosts the read-only analysis endpoints: topology, replicas,
// zombie detection, subject naming, event genome/catalog/wikipedia, chaos
// stories, architecture reports, hidden bottlenecks, and the raw monitoring
// passthroughs they build on.
package insights

import (
	"context"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
)

// Handler serves the insight endpoints. The two brokers back the SSE feeds:
// connz streams raw connection snapshots, replicas streams replica health.
type Handler struct {
	*apikit.Core

	connz    *snapshot.ConnzBroker
	replicas *snapshot.ConnzBroker
}

func NewHandler(svc *app.Services, cfg config.Config, hub *snapshot.Hub) *Handler {
	h := &Handler{Core: apikit.NewCore(svc, cfg, hub)}
	if svc == nil || svc.JetStream == nil {
		return h
	}
	h.connz = snapshot.NewConnzBroker(func(ctx context.Context, clusterID string) ([]byte, error) {
		client, err := svc.JetStream.GetExecutor(ctx, clusterID)
		if err != nil {
			return nil, err
		}
		return client.Monitoring(ctx, snapshot.ConnzAuthPath)
	}, snapshot.DefaultConnzInterval)
	h.replicas = snapshot.NewConnzBroker(func(ctx context.Context, clusterID string) ([]byte, error) {
		return fetchReplicasSnapshotJSON(ctx, svc, hub, clusterID, cfg.MaxMonitoringBodyBytes)
	}, snapshot.DefaultReplicasInterval).
		WithScrapeTimeout(snapshot.ReplicasScrapeTimeout(5)).
		WithFetchErrorThreshold(1).
		WithFetchErrorFallback(replicasOfflineFromLatest)
	return h
}
