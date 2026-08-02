package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	natspkg "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"

	libnats "github.com/gopherust-io/nats"
)

type fleetEvent struct {
	Service string         `json:"service"`
	TS      string         `json:"ts"`
	Kind    string         `json:"kind,omitempty"`
	ID      int            `json:"id"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// runID distinguishes Msg-Ids across process restarts so DuplicateWindow
// does not drop republished demo traffic.
var runID = strconv.FormatInt(time.Now().UnixNano(), 10)

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func rateScale() float64 {
	raw := strings.TrimSpace(os.Getenv("RATE_SCALE"))
	if raw == "" {
		return 1
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 {
		return 1
	}
	return v
}

func scaledDuration(base time.Duration) time.Duration {
	s := rateScale()
	if s >= 1 {
		return time.Duration(float64(base) / s)
	}
	return time.Duration(float64(base) / s)
}

func sleepWork(min, max time.Duration) time.Duration {
	if max < min {
		max = min
	}
	d := min
	if max > min {
		d = min + time.Duration(rand.Int63n(int64(max-min)+1)) //nolint:gosec // G404: demo work jitter
	}
	time.Sleep(d)
	return d
}

func publishJSON(ctx context.Context, client libnats.Client, subject, msgID string, payload map[string]any) error {
	return client.Publisher().PublishWithMsgID(ctx, subject, msgID, libnats.Message{
		Data:        payload,
		MessageType: libnats.JSON,
	})
}

func publishEvent(ctx context.Context, client libnats.Client, service, subject, kind string, id int, meta map[string]any) error {
	msgID := fmt.Sprintf("%s-%s-%s-%d", service, runID, kind, id)
	payload := map[string]any{
		"id":      id,
		"service": service,
		"kind":    kind,
		"ts":      time.Now().UTC().Format(time.RFC3339Nano),
	}
	if len(meta) > 0 {
		payload["meta"] = meta
	}
	if err := publishJSON(ctx, client, subject, msgID, payload); err != nil {
		return err
	}
	zerolog.Ctx(ctx).Info().
		Str("service", service).
		Str("subject", subject).
		Str("msg_id", msgID).
		Int("id", id).
		Msg("published")
	return nil
}

func runPublisherLoop(
	ctx context.Context,
	client libnats.Client,
	service string,
	subjects []string,
	every time.Duration,
	kindFor func(i int, subject string) string,
) {
	every = max(scaledDuration(every), 50*time.Millisecond)
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	zerolog.Ctx(ctx).Info().
		Str("service", service).
		Strs("subjects", subjects).
		Dur("every", every).
		Msg("publisher started")

	i := 0
	for {
		select {
		case <-ctx.Done():
			zerolog.Ctx(ctx).Info().Str("service", service).Msg("publisher stopped")
			return
		case <-ticker.C:
			i++
			subject := subjects[i%len(subjects)]
			kind := kindFor(i, subject)
			if err := publishEvent(ctx, client, service, subject, kind, i, nil); err != nil {
				zerolog.Ctx(ctx).Error().
					Str("service", service).
					Str("subject", subject).
					Err(err).
					Msg("publish failed")
			}
		}
	}
}

type workOpts struct {
	Service     string
	Min         time.Duration
	Max         time.Duration
	NakRate     float64 // 0–1
	PoisonEvery int     // 0 disables; else id % N == 0 → DLQ
	After       func(ctx context.Context, client libnats.Client, ev fleetEvent, msg *natspkg.Msg) error
}

func workHandler(client libnats.Client, opts workOpts) libnats.MsgHandler {
	primary := func(msgCtx context.Context, msg *natspkg.Msg) error {
		start := time.Now()
		var ev fleetEvent
		if err := json.Unmarshal(msg.Data, &ev); err != nil {
			zerolog.Ctx(msgCtx).Error().Err(err).Str("service", opts.Service).Msg("decode")
			if opts.PoisonEvery > 0 {
				return fmt.Errorf("decode: %w", libnats.ErrSendToDLQ)
			}
			return fmt.Errorf("decode: %w", err)
		}

		if opts.PoisonEvery > 0 && ev.ID > 0 && ev.ID%opts.PoisonEvery == 0 {
			zerolog.Ctx(msgCtx).Warn().
				Str("service", opts.Service).
				Int("id", ev.ID).
				Str("subject", msg.Subject).
				Msg("poison routed to dlq")
			return fmt.Errorf("simulated poison id=%d: %w", ev.ID, libnats.ErrSendToDLQ)
		}

		took := sleepWork(opts.Min, opts.Max)

		if opts.NakRate > 0 && rand.Float64() < opts.NakRate { //nolint:gosec // G404: demo nak injection
			zerolog.Ctx(msgCtx).Warn().
				Str("service", opts.Service).
				Int("id", ev.ID).
				Str("subject", msg.Subject).
				Msg("simulated nak")
			return fmt.Errorf("simulated transient failure service=%s id=%d", opts.Service, ev.ID)
		}

		if opts.After != nil {
			if err := opts.After(msgCtx, client, ev, msg); err != nil {
				return err
			}
		}

		zerolog.Ctx(msgCtx).Info().
			Str("service", opts.Service).
			Int("id", ev.ID).
			Str("subject", msg.Subject).
			Str("msg_id", msg.Header.Get(libnats.HeaderMsgID)).
			Dur("took", took).
			Dur("elapsed", time.Since(start)).
			Msg("processed")
		return nil
	}

	if opts.PoisonEvery > 0 {
		return libnats.WithDLQ(libnats.DLQConfig{
			Publisher:  client.Publisher(),
			Subject:    dlqPoisonSubject,
			MaxDeliver: 5,
			Autopsy:    libnats.AutopsyConfig{Enabled: true},
		}, primary)
	}
	return primary
}

func startPushQueue(
	ctx context.Context,
	client libnats.Client,
	stream, durable, queue, filter string,
	handler libnats.MsgHandler,
) error {
	_, err := startPushQueueSub(ctx, client, stream, durable, queue, filter, handler)
	return err
}

func startPushQueueSub(
	ctx context.Context,
	client libnats.Client,
	stream, durable, queue, filter string,
	handler libnats.MsgHandler,
) (libnats.Subscription, error) {
	sub, err := client.Consumer().QueueSubscribeBound(ctx, stream, durable, queue, filter, handler)
	if err != nil {
		return nil, fmt.Errorf("%s queue subscribe: %w", durable, err)
	}
	zerolog.Ctx(ctx).Info().
		Str("stream", stream).
		Str("durable", durable).
		Str("queue", queue).
		Str("filter", filter).
		Msg("push consumer started")
	return sub, nil
}

func startPull(
	ctx context.Context,
	client libnats.Client,
	stream, durable string,
	handler libnats.MsgHandler,
) error {
	pull, err := client.Consumer().Pull(stream, durable)
	if err != nil {
		return fmt.Errorf("%s pull: %w", durable, err)
	}
	zerolog.Ctx(ctx).Info().
		Str("stream", stream).
		Str("durable", durable).
		Msg("pull consumer started")
	go func() {
		if err := pull.Process(ctx, handler,
			libnats.WithFetchBatch(50),
			libnats.WithProcessMaxWait(3*time.Second),
			libnats.WithProcessHeartbeat(500*time.Millisecond),
		); err != nil && ctx.Err() == nil {
			zerolog.Ctx(ctx).Error().Err(err).Str("durable", durable).Msg("pull process")
		}
	}()
	return nil
}

func cascadePub(ctx context.Context, client libnats.Client, from string, id int, pairs ...string) {
	for i := 0; i+1 < len(pairs); i += 2 {
		subject, kind := pairs[i], pairs[i+1]
		_ = publishEvent(ctx, client, from, subject, kind, id, map[string]any{"from": from})
	}
}
