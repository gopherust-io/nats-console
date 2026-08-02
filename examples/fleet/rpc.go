package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	natspkg "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"

	libnats "github.com/gopherust-io/nats"
)

func startRPCResponder(
	ctx context.Context,
	client libnats.Client,
	service, subject, queue string,
	min, max time.Duration,
	reply func(req map[string]any) (map[string]any, error),
) error {
	_, err := client.Responder().QueueSubscribe(ctx, queue, subject, func(msgCtx context.Context, msg *natspkg.Msg) error {
		start := time.Now()
		var req map[string]any
		if len(msg.Data) > 0 {
			if err := json.Unmarshal(msg.Data, &req); err != nil {
				zerolog.Ctx(msgCtx).Warn().Err(err).Str("service", service).Msg("rpc decode")
				req = map[string]any{}
			}
		}
		took := sleepWork(min, max)
		out, err := reply(req)
		if err != nil {
			zerolog.Ctx(msgCtx).Warn().
				Str("service", service).
				Str("subject", subject).
				Err(err).
				Msg("rpc handler error")
			return err
		}
		if out == nil {
			out = map[string]any{}
		}
		out["service"] = service
		out["took_ms"] = took.Milliseconds()
		if err := libnats.RespondJSON(msg, out); err != nil {
			return err
		}
		zerolog.Ctx(msgCtx).Info().
			Str("service", service).
			Str("subject", subject).
			Dur("took", took).
			Dur("elapsed", time.Since(start)).
			Msg("rpc replied")
		return nil
	})
	if err != nil {
		return fmt.Errorf("%s rpc subscribe: %w", service, err)
	}
	zerolog.Ctx(ctx).Info().
		Str("service", service).
		Str("subject", subject).
		Str("queue", queue).
		Msg("rpc responder started")
	return nil
}

func runRPCRequesterLoop(
	ctx context.Context,
	client libnats.Client,
	service string,
	every time.Duration,
	call func(ctx context.Context, client libnats.Client, id int) error,
) {
	every = max(scaledDuration(every), 50*time.Millisecond)
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	zerolog.Ctx(ctx).Info().
		Str("service", service).
		Dur("every", every).
		Msg("rpc requester started")

	i := 0
	for {
		select {
		case <-ctx.Done():
			zerolog.Ctx(ctx).Info().Str("service", service).Msg("rpc requester stopped")
			return
		case <-ticker.C:
			i++
			start := time.Now()
			if err := call(ctx, client, i); err != nil {
				zerolog.Ctx(ctx).Warn().
					Str("service", service).
					Int("id", i).
					Err(err).
					Dur("elapsed", time.Since(start)).
					Msg("rpc request failed")
				continue
			}
			zerolog.Ctx(ctx).Info().
				Str("service", service).
				Int("id", i).
				Dur("elapsed", time.Since(start)).
				Msg("rpc request ok")
		}
	}
}

func bestEffortRPC(ctx context.Context, client libnats.Client, subject string, req, resp any) {
	rpcCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	if err := client.Requester().RequestJSONInto(rpcCtx, subject, req, resp); err != nil {
		zerolog.Ctx(ctx).Warn().
			Str("subject", subject).
			Err(err).
			Msg("best-effort rpc failed")
	}
}

func startInventoryRPC(ctx context.Context, client libnats.Client) error {
	return startRPCResponder(ctx, client, "inventory-rpc", "rpc.inventory.get", "rpc-inventory",
		5*time.Millisecond, 25*time.Millisecond,
		func(req map[string]any) (map[string]any, error) {
			sku, _ := req["sku"].(string)
			if sku == "" {
				sku = "SKU-1"
			}
			return map[string]any{
				"sku":       sku,
				"available": 10 + rand.Intn(90), //nolint:gosec // G404: demo inventory jitter
				"warehouse": "wh-east",
			}, nil
		})
}

func startPricingRPC(ctx context.Context, client libnats.Client) error {
	return startRPCResponder(ctx, client, "pricing-rpc", "rpc.pricing.quote", "rpc-pricing",
		8*time.Millisecond, 40*time.Millisecond,
		func(req map[string]any) (map[string]any, error) {
			sku, _ := req["sku"].(string)
			if sku == "" {
				sku = "SKU-1"
			}
			return map[string]any{
				"sku":      sku,
				"price":    19.99 + float64(rand.Intn(50)), //nolint:gosec // G404: demo price jitter
				"currency": "USD",
			}, nil
		})
}

func startFraudRPC(ctx context.Context, client libnats.Client) error {
	return startRPCResponder(ctx, client, "fraud-rpc", "rpc.fraud.score", "rpc-fraud",
		15*time.Millisecond, 60*time.Millisecond,
		func(req map[string]any) (map[string]any, error) {
			if rand.Float64() < 0.03 { //nolint:gosec // G404: demo failure injection
				return nil, errors.New("fraud rpc transient failure")
			}
			score := rand.Float64() //nolint:gosec // G404: demo score
			return map[string]any{
				"score": score,
				"allow": score < 0.85,
				"id":    req["id"],
			}, nil
		})
}

func startUserRPC(ctx context.Context, client libnats.Client) error {
	return startRPCResponder(ctx, client, "user-rpc", "rpc.user.lookup", "rpc-user",
		5*time.Millisecond, 20*time.Millisecond,
		func(req map[string]any) (map[string]any, error) {
			uid := req["user_id"]
			if uid == nil {
				uid = 1
			}
			return map[string]any{
				"user_id": uid,
				"tier":    "gold",
				"active":  true,
			}, nil
		})
}

func startCheckoutGateway(ctx context.Context, client libnats.Client) error {
	go runRPCRequesterLoop(ctx, client, "checkout-gateway", 400*time.Millisecond,
		func(ctx context.Context, c libnats.Client, id int) error {
			var user, inv, price map[string]any
			if err := c.Requester().RequestJSONInto(ctx, "rpc.user.lookup", map[string]any{"user_id": id}, &user); err != nil {
				return err
			}
			if err := c.Requester().RequestJSONInto(ctx, "rpc.inventory.get", map[string]any{"sku": "SKU-1", "id": id}, &inv); err != nil {
				return err
			}
			if err := c.Requester().RequestJSONInto(ctx, "rpc.pricing.quote", map[string]any{"sku": "SKU-1", "id": id}, &price); err != nil {
				return err
			}
			zerolog.Ctx(ctx).Debug().
				Interface("user", user).
				Interface("inventory", inv).
				Interface("price", price).
				Msg("checkout gateway assembled")
			return nil
		})
	return nil
}

func startRiskGateway(ctx context.Context, client libnats.Client) error {
	go runRPCRequesterLoop(ctx, client, "risk-gateway", 600*time.Millisecond,
		func(ctx context.Context, c libnats.Client, id int) error {
			var score map[string]any
			return c.Requester().RequestJSONInto(ctx, "rpc.fraud.score", map[string]any{"id": id}, &score)
		})
	return nil
}
