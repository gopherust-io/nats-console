package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	libnats "github.com/gopherust-io/nats"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/rs/zerolog"
)

type clientKind int

const (
	clientDefault clientKind = iota
	clientOps
	clientPoolNak
	clientFanOut
)

// dedicatedClients holds extra clients opened only in SERVICE=all mode.
type dedicatedClients struct {
	mu      sync.Mutex
	clients []libnats.Client
}

func (d *dedicatedClients) add(c libnats.Client) {
	d.mu.Lock()
	d.clients = append(d.clients, c)
	d.mu.Unlock()
}

func (d *dedicatedClients) shutdown() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, c := range d.clients {
		if err := c.Connector().Shutdown(); err != nil {
			zerolog.Ctx(context.Background()).Warn().Err(err).Msg("dedicated client shutdown")
		}
	}
	d.clients = nil
}

var fleetExtra = &dedicatedClients{}

func fleetServiceName() string {
	return strings.ToLower(envOr("SERVICE", "all"))
}

func multiServiceMode() bool {
	s := fleetServiceName()
	return commonstrings.IsEmpty(s) || s == "all"
}

func clientNameFor(service string) string {
	if commonstrings.IsEmpty(service) || service == "all" {
		return "fleet-all"
	}
	return "fleet-" + service
}

func applyPoolNak(cfg *libnats.Config) {
	cfg.RuntimeConsumer.WorkerPoolEnabled = true
	cfg.RuntimeConsumer.WorkerPoolSize = 4
	cfg.RuntimeConsumer.WorkerBufferSize = 32
	cfg.Backpressure.Mode = libnats.BackpressureNak
	cfg.Backpressure.MaxAckPending = 200
}

func applyOpsWorker(cfg *libnats.Config) {
	cfg.RuntimeConsumer.WorkerPoolEnabled = true
	cfg.RuntimeConsumer.WorkerPoolSize = 4
	cfg.RuntimeConsumer.WorkerBufferSize = 64
	cfg.Backpressure.Mode = libnats.BackpressureNak
	cfg.Metrics.TrackedConsumers = []libnats.TrackedConsumer{
		{Stream: "ORDERS", Durable: "order-fulfillment"},
	}
}

func applyFanOut(cfg *libnats.Config) {
	cfg.RuntimeConsumer.WorkerPoolEnabled = false
	cfg.RuntimeConsumer.AckWait = ackWait
	cfg.RuntimeConsumer.IdleHeartbeat = 0
	cfg.RuntimeConsumer.FlowControl = false
	cfg.RuntimeConsumer.PendingMsgLimit = 0
	cfg.RuntimeConsumer.PendingMsgBuffer = 0
	cfg.Backpressure.Mode = libnats.BackpressureBlock
	cfg.Backpressure.MaxAckPending = 500
	cfg.Backpressure.PendingMsgLimit = 0
	cfg.Backpressure.PendingMsgBuffer = 0
}

func buildConfigForService(service string) libnats.Config {
	cfg := buildFleetConfig()
	cfg.Conn.ClientName = clientNameFor(service)
	switch service {
	case "order-fulfillment":
		applyOpsWorker(&cfg)
	case "payment-processor", "media-transcoder":
		applyPoolNak(&cfg)
	case "email-notifier":
		applyFanOut(&cfg)
	}
	return cfg
}

// clientFor returns the process client in single-service mode (Docker), or a
// dedicated client when SERVICE=all so presets do not share one worker pool.
func clientFor(ctx context.Context, shared libnats.Client, name string, kind clientKind) (libnats.Client, error) {
	if !multiServiceMode() {
		return shared, nil
	}
	switch kind {
	case clientOps:
		return newServiceClient(ctx, name, applyOpsWorker)
	case clientPoolNak:
		return newServiceClient(ctx, name, applyPoolNak)
	case clientFanOut:
		return newServiceClient(ctx, name, applyFanOut)
	default:
		return shared, nil
	}
}

func newServiceClient(ctx context.Context, name string, mutate func(*libnats.Config)) (libnats.Client, error) {
	cfg := buildFleetConfig()
	cfg.Conn.ClientName = clientNameFor(name)
	if mutate != nil {
		mutate(&cfg)
	}
	c, err := libnats.NewClient(ctx, &cfg)
	if err != nil {
		return nil, fmt.Errorf("client %s: %w", name, err)
	}
	fleetExtra.add(c)
	return c, nil
}
