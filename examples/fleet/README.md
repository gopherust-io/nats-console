# JetStream demo fleet (32 services)

Runs **32 logical services** that publish, consume (push queue + pull), cascade work,
call **request/reply**, and showcase [`gopherust-io/nats`](https://github.com/gopherust-io/nats)
library features (idempotency, ops toolkit, shard, KV, object store).

For **Account → Connections**, run each service as its own Docker container so each
appears as a distinct `connz` client named `fleet-<service>`.

## Docker (recommended for Connections)

Included by the root [`docker-compose.yml`](../../docker-compose.yml) under Compose
profile **`fleet`** (same Docker Desktop project: `nats-consol`). Requires JetStream
reachable on that network (default hostname `nats`) and a sibling checkout of
`gopherust-io/nats` (this repo’s `go.mod` replace).

```bash
# From nats-consol: docker compose up -d   # postgres + nats + console

make fleet-up
# or: docker compose -p nats-consol -f examples/fleet/docker-compose.yml --profile fleet up -d --build
```

Then open Consol → Account → **Connections**. You should see names like
`fleet-order-api`, `fleet-payment-processor`, `fleet-checkout-gateway`, …

```bash
curl -s 'http://127.0.0.1:8222/connz?limit=1024' | jq -r '.connections[].name' | grep '^fleet-' | sort
```

### Overrides

| Variable | Default | Meaning |
|----------|---------|---------|
| `NATS_URL` | `nats://nats:4222` | NATS URL on the `nats-consol` network |
| `TEL_ENABLE` | `false` | OTLP on/off |

Examples:

```bash
# Consol single-node compose (service name "nats")
NATS_URL=nats://nats:4222 make fleet-up

# After make nats-cluster-up (nodes share the project default network)
FLEET_NATS_URL=nats://nats-cluster-1:4222 make fleet-up
```

Stop (removes fleet services only; leaves postgres/console/nats):

```bash
make fleet-down
# or: docker compose -p nats-consol -f examples/fleet/docker-compose.yml --profile fleet down
```

## Local (one process)

```bash
# All services, few connections (fleet-all + optional dedicated clients)
TEL_ENABLE=false NATS_URL=nats://127.0.0.1:4222 go run ./examples/fleet

# One service → one connection named fleet-<SERVICE>
SERVICE=payment-processor go run ./examples/fleet
RATE_SCALE=2 go run ./examples/fleet
```

## Topology

| Retention | Streams |
|-----------|---------|
| WorkQueue | `ORDERS` (explicit `orders.created\|updated\|cancelled`), `ORDERS_SHARD` (`orders.shard.>`), `PAYMENTS`, `INVENTORY`, `SHIPPING`, `BILLING`, `FRAUD`, `SEARCH`, `WEBHOOKS`, `RETURNS`, `CART`, `MEDIA` |
| Limits | `ORDERS_DLQ` (`dlq.orders.>`), `NOTIFICATIONS`, `AUDIT`, `CATALOG`, `USERS`, `RECO`, `LOYALTY`, `PRICING` |
| Interest | `TELEMETRY` |

KV: `fleet-idempotency`, `fleet-users`. Object store: `fleet-media`.

Core NATS RPC subjects (`rpc.>`) are **not** on a JetStream stream.

## Feature matrix

| Feature | Service(s) |
|---------|------------|
| Queue push (`QueueSubscribeBound`) | Most workers |
| Pull + fetch batch | projector, search, audit, telemetry, catalog, users, loyalty, pricing |
| Ops toolkit (DLQ, shadow, supervisor, soft liveness, flight recorder, fingerprint) | `order-fulfillment` |
| Idempotency KV claim | `payment-processor` |
| Worker pool + `BackpressureNak` | `media-transcoder`, `payment-processor`, `order-fulfillment` |
| `ProdFanOut` / `BackpressureBlock` | `email-notifier` |
| Supervise pull + concurrency | `order-projector` |
| Shard publish/consume | `order-api` → `order-shard-0` / `order-shard-1` |
| KV projection | `user-projector` → `fleet-users` |
| Object store | `media-transcoder` → `fleet-media` |
| Slow-consumer watch | `billing-worker` |
| Request/reply (queue responders) | `inventory-rpc`, `pricing-rpc`, `fraud-rpc`, `user-rpc` |
| Request/reply (gateways) | `checkout-gateway`, `risk-gateway` |
| Best-effort RPC from JS workers | fulfillment → inventory RPC; payments → fraud RPC |

In Docker / `SERVICE=<name>`, each process uses **one** NATS connection (`fleet-<name>`).
`SERVICE=all` may open extra dedicated clients for pool isolation.

## Request/reply

| Service | Subject / behavior |
|---------|-------------------|
| `inventory-rpc` | Queue `rpc.inventory.get` |
| `pricing-rpc` | Queue `rpc.pricing.quote` |
| `fraud-rpc` | Queue `rpc.fraud.score` (~3% errors) |
| `user-rpc` | Queue `rpc.user.lookup` |
| `checkout-gateway` | user → inventory → pricing |
| `risk-gateway` | fraud score |

## Environment

| Variable | Default | Meaning |
|----------|---------|---------|
| `NATS_URL` | `nats://127.0.0.1:4222` | NATS URL(s) |
| `SERVICE` | `all` | `all` or a single service name |
| `RATE_SCALE` | `1` | Higher = faster publishers / RPC loops |
| `TEL_ENABLE` | (tel default) | Force OTLP on/off |

## Notes

- NATS `ClientName` is `fleet-<SERVICE>` (or `fleet-all`).
- Msg-Ids include a per-process run id so restarts are not dropped by `DuplicateWindow`.
- If an older `ORDERS` stream still uses `orders.>`, delete/recreate it so `ORDERS_SHARD` can own `orders.shard.>`.
- Static inventory without live containers: `make seed-demo`.
