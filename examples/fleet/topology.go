package main

import (
	"context"
	"fmt"
	"time"

	libnats "github.com/gopherust-io/nats"
)

const (
	dlqStreamName = "ORDERS_DLQ"
	// Keep DLQ subjects outside orders.> so they do not overlap the ORDERS stream.
	dlqSubjectFilter = "dlq.orders.>"
	dlqPoisonSubject = "dlq.orders.poison"
	ackWait          = 45 * time.Second

	kvIdempotencyBucket = "fleet-idempotency"
	kvUsersBucket       = "fleet-users"
	objMediaBucket      = "fleet-media"
	orderShardPrefix    = "orders.shard"
	orderShardCount     = 2
)

type streamDef struct {
	Name      string
	Subjects  []string
	Retention libnats.RetentionPolicy
}

type pullDurableDef struct {
	Stream         string
	Durable        string
	FilterSubject  string
	FilterSubjects []string
}

func fleetStreams() []streamDef {
	return []streamDef{
		// Explicit subjects so ORDERS_SHARD can own orders.shard.> without overlap.
		{Name: "ORDERS", Subjects: []string{"orders.created", "orders.updated", "orders.cancelled"}, Retention: libnats.WorkQueuePolicy},
		{Name: "ORDERS_SHARD", Subjects: []string{orderShardPrefix + ".>"}, Retention: libnats.WorkQueuePolicy},
		{Name: dlqStreamName, Subjects: []string{dlqSubjectFilter}, Retention: libnats.LimitsPolicy},
		{Name: "PAYMENTS", Subjects: []string{"payments.>"}, Retention: libnats.WorkQueuePolicy},
		{Name: "INVENTORY", Subjects: []string{"inventory.>"}, Retention: libnats.WorkQueuePolicy},
		{Name: "SHIPPING", Subjects: []string{"shipping.>"}, Retention: libnats.WorkQueuePolicy},
		{Name: "BILLING", Subjects: []string{"billing.>"}, Retention: libnats.WorkQueuePolicy},
		{Name: "FRAUD", Subjects: []string{"fraud.>"}, Retention: libnats.WorkQueuePolicy},
		{Name: "SEARCH", Subjects: []string{"search.>"}, Retention: libnats.WorkQueuePolicy},
		{Name: "WEBHOOKS", Subjects: []string{"webhooks.>"}, Retention: libnats.WorkQueuePolicy},
		{Name: "RETURNS", Subjects: []string{"returns.>"}, Retention: libnats.WorkQueuePolicy},
		{Name: "CART", Subjects: []string{"cart.>"}, Retention: libnats.WorkQueuePolicy},
		{Name: "MEDIA", Subjects: []string{"media.>"}, Retention: libnats.WorkQueuePolicy},
		{Name: "NOTIFICATIONS", Subjects: []string{"notify.>"}, Retention: libnats.LimitsPolicy},
		{Name: "AUDIT", Subjects: []string{"audit.>"}, Retention: libnats.LimitsPolicy},
		{Name: "TELEMETRY", Subjects: []string{"telemetry.>", "logs.app"}, Retention: libnats.InterestPolicy},
		{Name: "CATALOG", Subjects: []string{"catalog.>"}, Retention: libnats.LimitsPolicy},
		{Name: "USERS", Subjects: []string{"users.>"}, Retention: libnats.LimitsPolicy},
		{Name: "RECO", Subjects: []string{"reco.>"}, Retention: libnats.LimitsPolicy},
		{Name: "LOYALTY", Subjects: []string{"loyalty.>"}, Retention: libnats.LimitsPolicy},
		{Name: "PRICING", Subjects: []string{"pricing.>"}, Retention: libnats.LimitsPolicy},
	}
}

func fleetPullDurables() []pullDurableDef {
	return []pullDurableDef{
		{Stream: "ORDERS", Durable: "order-projector", FilterSubjects: []string{"orders.updated", "orders.cancelled"}},
		{Stream: "SEARCH", Durable: "search-indexer", FilterSubject: "search.index"},
		{Stream: "AUDIT", Durable: "audit-logger", FilterSubject: "audit.>"},
		{Stream: "TELEMETRY", Durable: "metrics-aggregator", FilterSubject: "telemetry.metrics"},
		{Stream: "TELEMETRY", Durable: "log-shipper", FilterSubject: "logs.app"},
		{Stream: "CATALOG", Durable: "catalog-sync", FilterSubject: "catalog.>"},
		{Stream: "USERS", Durable: "user-projector", FilterSubject: "users.>"},
		{Stream: "LOYALTY", Durable: "loyalty-worker", FilterSubject: "loyalty.>"},
		{Stream: "PRICING", Durable: "pricing-engine", FilterSubject: "pricing.>"},
	}
}

func ensureTopology(ctx context.Context, client libnats.Client) error {
	for _, s := range fleetStreams() {
		maxAge := 24 * time.Hour
		if s.Name == dlqStreamName {
			maxAge = 7 * 24 * time.Hour
		}
		if _, err := client.Streams().CreateOrUpdateStream(ctx, libnats.StreamConfig{
			Name:            s.Name,
			Subjects:        s.Subjects,
			Replicas:        1,
			Storage:         libnats.MemoryStorage,
			Retention:       s.Retention,
			MaxAge:          maxAge,
			Discard:         libnats.DiscardOld,
			DuplicateWindow: 2 * time.Minute,
		}); err != nil {
			return fmt.Errorf("stream %s: %w", s.Name, err)
		}
	}

	for _, d := range fleetPullDurables() {
		cfg := libnats.DurableConsumerConfig{
			Durable:        d.Durable,
			FilterSubject:  d.FilterSubject,
			FilterSubjects: d.FilterSubjects,
			AckPolicy:      libnats.AckExplicit,
			MaxAckPending:  1000,
			AckWait:        ackWait,
			MaxDeliver:     5,
			MaxWaiting:     512,
			DeliverPolicy:  libnats.DeliverNew,
		}
		if _, err := client.Consumers().CreateOrUpdateConsumer(ctx, d.Stream, cfg); err != nil {
			return fmt.Errorf("pull durable %s/%s: %w", d.Stream, d.Durable, err)
		}
	}

	if _, err := client.KV().CreateOrUpdate(ctx, libnats.KeyValueConfig{
		Bucket:      kvIdempotencyBucket,
		Description: "fleet payment idempotency claims",
		TTL:         10 * time.Minute,
		History:     1,
		Storage:     libnats.MemoryStorage,
	}); err != nil {
		return fmt.Errorf("kv %s: %w", kvIdempotencyBucket, err)
	}
	if _, err := client.KV().CreateOrUpdate(ctx, libnats.KeyValueConfig{
		Bucket:      kvUsersBucket,
		Description: "fleet user projector snapshots",
		TTL:         24 * time.Hour,
		History:     1,
		Storage:     libnats.MemoryStorage,
	}); err != nil {
		return fmt.Errorf("kv %s: %w", kvUsersBucket, err)
	}
	if _, err := client.Objects().BucketInfo(ctx, objMediaBucket); err != nil {
		if _, createErr := client.Objects().Create(ctx, libnats.ObjectStoreConfig{
			Bucket:      objMediaBucket,
			Description: "fleet media transcoder blobs",
			TTL:         24 * time.Hour,
			Storage:     libnats.MemoryStorage,
		}); createErr != nil {
			return fmt.Errorf("object store %s: %w", objMediaBucket, createErr)
		}
	}
	return nil
}

func buildFleetConfig() libnats.Config {
	cfg := libnats.ProdWorkerConfig()
	cfg.Conn.Address = envOr("NATS_URL", "nats://127.0.0.1:4222")
	cfg.Conn.ClientName = clientNameFor(fleetServiceName())

	// Shared SERVICE=all process: pool off so push handlers are not collapsed
	// into the first registered worker (poolOnce). Single-service Docker
	// containers re-enable pool via buildConfigForService.
	cfg.RuntimeConsumer.WorkerPoolEnabled = false
	cfg.RuntimeConsumer.AckWait = ackWait
	cfg.RuntimeConsumer.IdleHeartbeat = 0
	cfg.RuntimeConsumer.FlowControl = false

	cfg.Backpressure.Mode = libnats.BackpressureNak
	cfg.Backpressure.MaxAckPending = 1000

	var streams []string
	for _, s := range fleetStreams() {
		streams = append(streams, s.Name)
	}
	cfg.Metrics.TrackedStreams = streams
	return cfg
}
