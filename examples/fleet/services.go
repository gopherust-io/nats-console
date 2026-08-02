package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	natspkg "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"

	libnats "github.com/gopherust-io/nats"
)

type fleetService struct {
	Name  string
	Start func(ctx context.Context, client libnats.Client) error
}

func allServices() []fleetService {
	return []fleetService{
		{Name: "order-api", Start: startOrderAPI},
		{Name: "order-fulfillment", Start: startOrderFulfillment},
		{Name: "order-projector", Start: startOrderProjector},
		{Name: "order-shard-0", Start: startOrderShard(0)},
		{Name: "order-shard-1", Start: startOrderShard(1)},
		{Name: "payment-api", Start: startPaymentAPI},
		{Name: "payment-processor", Start: startPaymentProcessor},
		{Name: "inventory-worker", Start: startInventoryWorker},
		{Name: "shipping-dispatcher", Start: startShippingDispatcher},
		{Name: "billing-worker", Start: startBillingWorker},
		{Name: "fraud-scanner", Start: startFraudScanner},
		{Name: "cart-worker", Start: startCartWorker},
		{Name: "returns-processor", Start: startReturnsProcessor},
		{Name: "search-indexer", Start: startSearchIndexer},
		{Name: "webhook-delivery", Start: startWebhookDelivery},
		{Name: "media-transcoder", Start: startMediaTranscoder},
		{Name: "email-notifier", Start: startEmailNotifier},
		{Name: "sms-notifier", Start: startSMSNotifier},
		{Name: "push-notifier", Start: startPushNotifier},
		{Name: "audit-logger", Start: startAuditLogger},
		{Name: "metrics-aggregator", Start: startMetricsAggregator},
		{Name: "log-shipper", Start: startLogShipper},
		{Name: "catalog-sync", Start: startCatalogSync},
		{Name: "user-projector", Start: startUserProjector},
		{Name: "loyalty-worker", Start: startLoyaltyWorker},
		{Name: "pricing-engine", Start: startPricingEngine},
		{Name: "inventory-rpc", Start: startInventoryRPC},
		{Name: "pricing-rpc", Start: startPricingRPC},
		{Name: "fraud-rpc", Start: startFraudRPC},
		{Name: "user-rpc", Start: startUserRPC},
		{Name: "checkout-gateway", Start: startCheckoutGateway},
		{Name: "risk-gateway", Start: startRiskGateway},
	}
}

func startSelectedServices(ctx context.Context, client libnats.Client, want string) error {
	want = strings.ToLower(strings.TrimSpace(want))
	services := allServices()
	names := make([]string, 0, len(services))
	for _, s := range services {
		names = append(names, s.Name)
	}

	startOne := func(s fleetService) error {
		zerolog.Ctx(ctx).Info().Str("service", s.Name).Msg("starting service")
		if err := s.Start(ctx, client); err != nil {
			return fmt.Errorf("%s: %w", s.Name, err)
		}
		return nil
	}

	if want == "" || want == "all" {
		for _, s := range services {
			if err := startOne(s); err != nil {
				return err
			}
		}
		zerolog.Ctx(ctx).Info().Int("count", len(services)).Msg("fleet services running")
		return nil
	}

	for _, s := range services {
		if s.Name == want {
			return startOne(s)
		}
	}
	return fmt.Errorf("unknown SERVICE=%q want one of: all|%s", want, strings.Join(names, "|"))
}

func startPaymentAPI(ctx context.Context, client libnats.Client) error {
	go runPublisherLoop(ctx, client, "payment-api",
		[]string{"payments.authorize", "payments.capture", "payments.refund"},
		400*time.Millisecond,
		func(_ int, subject string) string {
			parts := strings.Split(subject, ".")
			return parts[len(parts)-1]
		})
	return nil
}

func startInventoryWorker(ctx context.Context, client libnats.Client) error {
	h := workHandler(client, workOpts{
		Service: "inventory-worker",
		Min:     10 * time.Millisecond,
		Max:     50 * time.Millisecond,
		NakRate: 0.01,
	})
	return startPushQueue(ctx, client, "INVENTORY", "inventory-worker", "inventory-worker-q", "inventory.>", h)
}

func startShippingDispatcher(ctx context.Context, client libnats.Client) error {
	h := workHandler(client, workOpts{
		Service: "shipping-dispatcher",
		Min:     15 * time.Millisecond,
		Max:     60 * time.Millisecond,
		After: func(msgCtx context.Context, c libnats.Client, ev fleetEvent, msg *natspkg.Msg) error {
			if msg.Subject == "shipping.create" {
				cascadePub(msgCtx, c, "shipping-dispatcher", ev.ID,
					"shipping.label", "label",
					"notify.sms", "sms",
				)
			}
			return nil
		},
	})
	return startPushQueue(ctx, client, "SHIPPING", "shipping-dispatcher", "shipping-dispatcher-q", "shipping.>", h)
}

func startFraudScanner(ctx context.Context, client libnats.Client) error {
	h := workHandler(client, workOpts{
		Service: "fraud-scanner",
		Min:     20 * time.Millisecond,
		Max:     70 * time.Millisecond,
		NakRate: 0.05,
	})
	return startPushQueue(ctx, client, "FRAUD", "fraud-scanner", "fraud-scanner-q", "fraud.check", h)
}

func startCartWorker(ctx context.Context, client libnats.Client) error {
	go runPublisherLoop(ctx, client, "cart-worker",
		[]string{"cart.checkout", "cart.abandoned"},
		700*time.Millisecond,
		func(_ int, subject string) string {
			if strings.HasSuffix(subject, "checkout") {
				return "checkout"
			}
			return "abandoned"
		})

	h := workHandler(client, workOpts{
		Service: "cart-worker",
		Min:     10 * time.Millisecond,
		Max:     40 * time.Millisecond,
		After: func(msgCtx context.Context, c libnats.Client, ev fleetEvent, msg *natspkg.Msg) error {
			if strings.HasSuffix(msg.Subject, "checkout") {
				cascadePub(msgCtx, c, "cart-worker", ev.ID,
					"orders.created", "created",
					"pricing.quote", "quote",
					"notify.push", "push",
				)
			} else {
				cascadePub(msgCtx, c, "cart-worker", ev.ID,
					"notify.email", "email",
					"pricing.quote", "quote",
				)
			}
			return nil
		},
	})
	return startPushQueue(ctx, client, "CART", "cart-worker", "cart-worker-q", "cart.>", h)
}

func startReturnsProcessor(ctx context.Context, client libnats.Client) error {
	go runPublisherLoop(ctx, client, "returns-processor",
		[]string{"returns.requested"},
		1200*time.Millisecond,
		func(int, string) string { return "requested" })

	h := workHandler(client, workOpts{
		Service: "returns-processor",
		Min:     20 * time.Millisecond,
		Max:     80 * time.Millisecond,
		After: func(msgCtx context.Context, c libnats.Client, ev fleetEvent, _ *natspkg.Msg) error {
			cascadePub(msgCtx, c, "returns-processor", ev.ID,
				"inventory.release", "release",
				"payments.refund", "refund",
				"notify.email", "email",
				"loyalty.adjust", "adjust",
			)
			return nil
		},
	})
	return startPushQueue(ctx, client, "RETURNS", "returns-processor", "returns-processor-q", "returns.>", h)
}

func startSearchIndexer(ctx context.Context, client libnats.Client) error {
	h := workHandler(client, workOpts{
		Service: "search-indexer",
		Min:     15 * time.Millisecond,
		Max:     50 * time.Millisecond,
	})
	return startPull(ctx, client, "SEARCH", "search-indexer", h)
}

func startWebhookDelivery(ctx context.Context, client libnats.Client) error {
	h := workHandler(client, workOpts{
		Service: "webhook-delivery",
		Min:     50 * time.Millisecond,
		Max:     200 * time.Millisecond,
		NakRate: 0.03,
	})
	return startPushQueue(ctx, client, "WEBHOOKS", "webhook-delivery", "webhook-delivery-q", "webhooks.deliver", h)
}

func startSMSNotifier(ctx context.Context, client libnats.Client) error {
	h := workHandler(client, workOpts{
		Service: "sms-notifier",
		Min:     10 * time.Millisecond,
		Max:     40 * time.Millisecond,
	})
	return startPushQueue(ctx, client, "NOTIFICATIONS", "sms-notifier", "sms-notifier-q", "notify.sms", h)
}

func startPushNotifier(ctx context.Context, client libnats.Client) error {
	h := workHandler(client, workOpts{
		Service: "push-notifier",
		Min:     5 * time.Millisecond,
		Max:     25 * time.Millisecond,
	})
	return startPushQueue(ctx, client, "NOTIFICATIONS", "push-notifier", "push-notifier-q", "notify.push", h)
}

func startAuditLogger(ctx context.Context, client libnats.Client) error {
	h := workHandler(client, workOpts{
		Service: "audit-logger",
		Min:     2 * time.Millisecond,
		Max:     15 * time.Millisecond,
	})
	return startPull(ctx, client, "AUDIT", "audit-logger", h)
}

func startMetricsAggregator(ctx context.Context, client libnats.Client) error {
	h := workHandler(client, workOpts{
		Service: "metrics-aggregator",
		Min:     2 * time.Millisecond,
		Max:     10 * time.Millisecond,
	})
	if err := startPull(ctx, client, "TELEMETRY", "metrics-aggregator", h); err != nil {
		return err
	}
	go runPublisherLoop(ctx, client, "metrics-aggregator",
		[]string{"telemetry.metrics"},
		500*time.Millisecond,
		func(int, string) string { return "metrics" })
	return nil
}

func startLogShipper(ctx context.Context, client libnats.Client) error {
	h := workHandler(client, workOpts{
		Service: "log-shipper",
		Min:     2 * time.Millisecond,
		Max:     10 * time.Millisecond,
	})
	if err := startPull(ctx, client, "TELEMETRY", "log-shipper", h); err != nil {
		return err
	}
	go runPublisherLoop(ctx, client, "log-shipper",
		[]string{"logs.app"},
		600*time.Millisecond,
		func(int, string) string { return "log" })
	return nil
}

func startCatalogSync(ctx context.Context, client libnats.Client) error {
	go runPublisherLoop(ctx, client, "catalog-sync",
		[]string{"catalog.updated", "catalog.published"},
		900*time.Millisecond,
		func(_ int, subject string) string {
			if strings.HasSuffix(subject, "published") {
				return "published"
			}
			return "updated"
		})

	h := workHandler(client, workOpts{
		Service: "catalog-sync",
		Min:     10 * time.Millisecond,
		Max:     40 * time.Millisecond,
		After: func(msgCtx context.Context, c libnats.Client, ev fleetEvent, _ *natspkg.Msg) error {
			cascadePub(msgCtx, c, "catalog-sync", ev.ID,
				"search.index", "index",
				"reco.refresh", "refresh",
				"pricing.quote", "quote",
			)
			return nil
		},
	})
	return startPull(ctx, client, "CATALOG", "catalog-sync", h)
}

func startLoyaltyWorker(ctx context.Context, client libnats.Client) error {
	h := workHandler(client, workOpts{
		Service: "loyalty-worker",
		Min:     10 * time.Millisecond,
		Max:     40 * time.Millisecond,
	})
	return startPull(ctx, client, "LOYALTY", "loyalty-worker", h)
}

func startPricingEngine(ctx context.Context, client libnats.Client) error {
	go runPublisherLoop(ctx, client, "pricing-engine",
		[]string{"pricing.updated"},
		1100*time.Millisecond,
		func(int, string) string { return "updated" })

	h := workHandler(client, workOpts{
		Service: "pricing-engine",
		Min:     15 * time.Millisecond,
		Max:     60 * time.Millisecond,
		After: func(msgCtx context.Context, c libnats.Client, ev fleetEvent, _ *natspkg.Msg) error {
			cascadePub(msgCtx, c, "pricing-engine", ev.ID,
				"catalog.updated", "updated",
				"reco.refresh", "refresh",
			)
			return nil
		},
	})
	return startPull(ctx, client, "PRICING", "pricing-engine", h)
}
