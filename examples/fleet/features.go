package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	natspkg "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"

	libnats "github.com/gopherust-io/nats"
	"github.com/gopherust-io/nats/idempotency"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func startOrderFulfillment(ctx context.Context, shared libnats.Client) error {
	client, err := clientFor(ctx, shared, "order-fulfillment", clientOps)
	if err != nil {
		return err
	}

	rec := libnats.NewFlightRecorder(128)

	primary := workHandler(shared, workOpts{
		Service:     "order-fulfillment",
		Min:         10 * time.Millisecond,
		Max:         40 * time.Millisecond,
		NakRate:     0.02,
		PoisonEvery: 0, // DLQ wrapped below with recorder
		After: func(msgCtx context.Context, c libnats.Client, ev fleetEvent, _ *natspkg.Msg) error {
			var inv map[string]any
			bestEffortRPC(msgCtx, shared, "rpc.inventory.get", map[string]any{
				"sku": "SKU-1",
				"id":  ev.ID,
			}, &inv)
			cascadePub(msgCtx, c, "order-fulfillment", ev.ID,
				"inventory.reserve", "reserve",
				"shipping.create", "create",
				"notify.email", "email",
				"audit.order", "order",
				"search.index", "index",
			)
			return nil
		},
	})

	// Re-wrap with poison + DLQ + shadow using flight recorder.
	inner := func(msgCtx context.Context, msg *natspkg.Msg) error {
		var ev fleetEvent
		if err := json.Unmarshal(msg.Data, &ev); err != nil {
			return fmt.Errorf("decode: %w", libnats.ErrSendToDLQ)
		}
		if ev.ID > 0 && ev.ID%17 == 0 {
			zerolog.Ctx(msgCtx).Warn().Int("id", ev.ID).Msg("poison routed to dlq")
			return fmt.Errorf("simulated poison id=%d: %w", ev.ID, libnats.ErrSendToDLQ)
		}
		return primary(msgCtx, msg)
	}
	shadow := func(msgCtx context.Context, msg *natspkg.Msg) error {
		var ev fleetEvent
		if err := json.Unmarshal(msg.Data, &ev); err != nil {
			return err
		}
		if ev.ID > 0 && ev.ID%17 == 0 {
			return errors.New("shadow also rejects poison")
		}
		return nil
	}

	handler := libnats.WithDLQ(libnats.DLQConfig{
		Publisher:  client.Publisher(),
		Subject:    dlqPoisonSubject,
		MaxDeliver: 5,
		Recorder:   rec,
		Autopsy:    libnats.AutopsyConfig{Enabled: true},
	}, client.WithShadow(libnats.ShadowConfig{
		SampleRate: 0.25,
		Recorder:   rec,
	}, inner, shadow))

	supCfg := libnats.SupervisorConfig{
		MaxRetries:     10,
		InitialBackoff: time.Second,
		CheckInterval:  time.Second,
	}
	rec.AttachSupervisor(&supCfg)

	sub, err := client.SuperviseQueueSubscribeBound(ctx,
		"ORDERS", "order-fulfillment", "order-fulfillment-q", "orders.created",
		handler, supCfg)
	if err != nil {
		return fmt.Errorf("order-fulfillment supervise: %w", err)
	}

	liveCfg := libnats.SoftLivenessConfig{
		PollInterval:  2 * time.Second,
		StallAfter:    15 * time.Second,
		RisingWindows: 3,
	}
	rec.AttachSoftLiveness(&liveCfg)
	live, err := client.WatchSoftLiveness(ctx, sub, liveCfg)
	if err != nil {
		_ = sub.Stop()
		return fmt.Errorf("order-fulfillment soft liveness: %w", err)
	}

	if _, err := client.KV().CreateOrUpdate(ctx, libnats.KeyValueConfig{
		Bucket:      libnats.DefaultBehaviorFingerprintKVBucket,
		Description: "nats-consol consumer behavior fingerprints",
		History:     1,
	}); err != nil {
		live.Stop()
		_ = sub.Stop()
		return fmt.Errorf("fingerprint kv: %w", err)
	}

	fp, err := client.WatchBehaviorFingerprint(ctx, sub, libnats.BehaviorFingerprintConfig{
		ReportBucket: libnats.DefaultBehaviorFingerprintKVBucket,
		OnAnomaly: func(ev libnats.BehaviorAnomalyEvent) {
			zerolog.Ctx(ctx).Warn().
				Str("stream", ev.Stream).
				Str("durable", ev.Durable).
				Msg("behavior fingerprint anomaly")
		},
	})
	if err != nil {
		live.Stop()
		_ = sub.Stop()
		return fmt.Errorf("behavior fingerprint: %w", err)
	}

	go func() {
		<-ctx.Done()
		fp.Stop()
		live.Stop()
		_ = sub.Stop()
		dumpFlightRecorder(ctx, rec)
	}()

	zerolog.Ctx(ctx).Info().Str("service", "order-fulfillment").Msg("ops toolkit worker started")
	return nil
}

func dumpFlightRecorder(ctx context.Context, rec *libnats.FlightRecorder) {
	if rec == nil || len(rec.Snapshot()) == 0 {
		return
	}
	zerolog.Ctx(ctx).Warn().Int("incidents", len(rec.Snapshot())).Msg("flight recorder dump")
	_ = rec.WriteJSON(os.Stderr)
}

func startPaymentProcessor(ctx context.Context, shared libnats.Client) error {
	client, err := clientFor(ctx, shared, "payment-processor", clientPoolNak)
	if err != nil {
		return err
	}

	kv, err := client.KV().CreateOrUpdate(ctx, libnats.KeyValueConfig{
		Bucket:   kvIdempotencyBucket,
		TTL:      10 * time.Minute,
		History:  1,
		Storage:  libnats.FileStorage,
		Replicas: streamReplicas(),
	})
	if err != nil {
		return fmt.Errorf("idempotency kv: %w", err)
	}

	base := workHandler(shared, workOpts{
		Service: "payment-processor",
		Min:     20 * time.Millisecond,
		Max:     80 * time.Millisecond,
		NakRate: 0.02,
		After: func(msgCtx context.Context, c libnats.Client, ev fleetEvent, _ *natspkg.Msg) error {
			var score map[string]any
			bestEffortRPC(msgCtx, shared, "rpc.fraud.score", map[string]any{"id": ev.ID}, &score)
			cascadePub(msgCtx, c, "payment-processor", ev.ID,
				"fraud.check", "check",
				"billing.invoice", "invoice",
				"audit.payment", "payment",
				"webhooks.deliver", "deliver",
			)
			return nil
		},
	})

	handler := idempotency.WithHandler(idempotency.NewKVStore(kv), idempotency.MsgIDFromHeader, base)
	return startPushQueue(ctx, client, "PAYMENTS", "payment-processor", "payment-processor-q", "payments.>", handler)
}

func startOrderProjector(ctx context.Context, client libnats.Client) error {
	h := workHandler(client, workOpts{
		Service: "order-projector",
		Min:     5 * time.Millisecond,
		Max:     20 * time.Millisecond,
	})
	supCfg := libnats.SupervisorConfig{
		MaxRetries:     10,
		InitialBackoff: time.Second,
		CheckInterval:  time.Second,
	}
	go func() {
		if err := client.SupervisePullProcess(ctx, "ORDERS", "order-projector", h, supCfg,
			libnats.WithFetchBatch(50),
			libnats.WithProcessMaxWait(3*time.Second),
			libnats.WithProcessHeartbeat(500*time.Millisecond),
			libnats.WithProcessConcurrency(4),
		); err != nil && ctx.Err() == nil {
			zerolog.Ctx(ctx).Error().Err(err).Msg("order-projector supervise pull")
		}
	}()
	zerolog.Ctx(ctx).Info().Str("service", "order-projector").Msg("supervised pull started")
	return nil
}

func startMediaTranscoder(ctx context.Context, shared libnats.Client) error {
	client, err := clientFor(ctx, shared, "media-transcoder", clientPoolNak)
	if err != nil {
		return err
	}

	go runPublisherLoop(ctx, client, "media-transcoder",
		[]string{"media.transcode"},
		1500*time.Millisecond,
		func(int, string) string { return "transcode" })

	h := workHandler(client, workOpts{
		Service: "media-transcoder",
		Min:     100 * time.Millisecond,
		Max:     300 * time.Millisecond,
		NakRate: 0.02,
		After: func(msgCtx context.Context, c libnats.Client, ev fleetEvent, _ *natspkg.Msg) error {
			name := fmt.Sprintf("asset-%d.bin", ev.ID)
			payload := commonstrings.StringToBytes(fmt.Sprintf("fleet-media-%d-%s", ev.ID, runID))
			if _, err := c.Objects().Put(msgCtx, objMediaBucket, name, payload); err != nil {
				zerolog.Ctx(msgCtx).Warn().Err(err).Str("object", name).Msg("object put failed")
			} else {
				zerolog.Ctx(msgCtx).Info().Str("bucket", objMediaBucket).Str("object", name).Msg("object stored")
			}
			cascadePub(msgCtx, c, "media-transcoder", ev.ID,
				"notify.push", "push",
				"webhooks.deliver", "deliver",
			)
			return nil
		},
	})
	return startPushQueue(ctx, client, "MEDIA", "media-transcoder", "media-transcoder-q", "media.>", h)
}

func startEmailNotifier(ctx context.Context, shared libnats.Client) error {
	client, err := clientFor(ctx, shared, "email-notifier", clientFanOut)
	if err != nil {
		return err
	}
	h := workHandler(client, workOpts{
		Service: "email-notifier",
		Min:     10 * time.Millisecond,
		Max:     40 * time.Millisecond,
	})
	return startPushQueue(ctx, client, "NOTIFICATIONS", "email-notifier", "email-notifier-q", "notify.email", h)
}

func startBillingWorker(ctx context.Context, client libnats.Client) error {
	h := workHandler(client, workOpts{
		Service: "billing-worker",
		Min:     30 * time.Millisecond,
		Max:     100 * time.Millisecond,
		NakRate: 0.02,
	})
	sub, err := startPushQueueSub(ctx, client, "BILLING", "billing-worker", "billing-worker-q", "billing.invoice", h)
	if err != nil {
		return err
	}
	lastSeq := func(ctx context.Context, stream string) (uint64, error) {
		info, err := client.Streams().StreamInfo(ctx, stream)
		if err != nil || info == nil {
			return 0, err
		}
		return info.State.LastSeq, nil
	}
	sc, err := libnats.WatchSlowConsumer(ctx, sub, lastSeq, libnats.SlowConsumerConfig{
		PollInterval:     2 * time.Second,
		SustainFor:       4 * time.Second,
		PendingThreshold: 50,
		LagThreshold:     50,
		AckPendingRatio:  0.5,
		OnSlow: func(ev libnats.SlowConsumerEvent) {
			zerolog.Ctx(ctx).Warn().
				Str("stream", ev.Stream).
				Str("durable", ev.Durable).
				Uint64("pending", ev.Pending).
				Int("ack_pending", ev.AckPending).
				Strs("reasons", ev.Reasons).
				Msg("slow consumer detected")
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("billing slow consumer: %w", err)
	}
	go func() {
		<-ctx.Done()
		sc.Stop()
	}()
	return nil
}

func startOrderAPI(ctx context.Context, client libnats.Client) error {
	go runPublisherLoop(ctx, client, "order-api",
		[]string{"orders.created", "orders.updated", "orders.cancelled"},
		300*time.Millisecond,
		func(_ int, subject string) string {
			switch {
			case strings.HasSuffix(subject, "created"):
				return "created"
			case strings.HasSuffix(subject, "updated"):
				return "updated"
			default:
				return "cancelled"
			}
		})

	// Sharded publish path for ORDERS_SHARD consumers.
	go func() {
		every := scaledDuration(500 * time.Millisecond)
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				i++
				key := fmt.Sprintf("user-%d", i%16)
				subject := libnats.ShardSubject(orderShardPrefix, key, orderShardCount, "created")
				if err := publishEvent(ctx, client, "order-api", subject, "created", i, map[string]any{
					"shard_key": key,
				}); err != nil {
					zerolog.Ctx(ctx).Warn().Err(err).Str("subject", subject).Msg("shard publish failed")
				}
			}
		}
	}()
	return nil
}

func startOrderShard(shard int) func(context.Context, libnats.Client) error {
	name := fmt.Sprintf("order-shard-%d", shard)
	filter := fmt.Sprintf("%s.%d.>", orderShardPrefix, shard)
	return func(ctx context.Context, client libnats.Client) error {
		h := workHandler(client, workOpts{
			Service: name,
			Min:     8 * time.Millisecond,
			Max:     30 * time.Millisecond,
		})
		return startPushQueue(ctx, client, "ORDERS_SHARD", name, name+"-q", filter, h)
	}
}

func startUserProjector(ctx context.Context, client libnats.Client) error {
	go runPublisherLoop(ctx, client, "user-projector",
		[]string{"users.created", "users.updated"},
		800*time.Millisecond,
		func(_ int, subject string) string {
			if strings.HasSuffix(subject, "created") {
				return "created"
			}
			return "updated"
		})

	h := workHandler(client, workOpts{
		Service: "user-projector",
		Min:     8 * time.Millisecond,
		Max:     30 * time.Millisecond,
		After: func(msgCtx context.Context, c libnats.Client, ev fleetEvent, _ *natspkg.Msg) error {
			key := fmt.Sprintf("user-%d", ev.ID)
			snap, _ := json.Marshal(map[string]any{
				"id":      ev.ID,
				"kind":    ev.Kind,
				"ts":      ev.TS,
				"service": "user-projector",
			})
			if _, err := c.KVKeys().Put(msgCtx, kvUsersBucket, key, snap); err != nil {
				zerolog.Ctx(msgCtx).Warn().Err(err).Str("key", key).Msg("user kv put failed")
			} else {
				zerolog.Ctx(msgCtx).Info().Str("bucket", kvUsersBucket).Str("key", key).Msg("user snapshot stored")
			}
			cascadePub(msgCtx, c, "user-projector", ev.ID,
				"loyalty.enroll", "enroll",
				"audit.user", "user",
			)
			return nil
		},
	})
	return startPull(ctx, client, "USERS", "user-projector", h)
}
