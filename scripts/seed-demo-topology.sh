#!/usr/bin/env sh
# Seed JetStream demo streams, consumers, and sample messages for the Topology UI.
# Requires: docker compose up (nats service running)
set -eu

NATS_URL="${NATS_URL:-nats://nats:4222}"
NETWORK="${DOCKER_NETWORK:-nats-consol_default}"
IMAGE="${NATS_BOX_IMAGE:-natsio/nats-box:0.14.5}"

run_nats() {
  docker run --rm --network "$NETWORK" "$IMAGE" nats -s "$NATS_URL" "$@"
}

echo "→ Seeding demo JetStream topology on $NATS_URL"

run_nats stream add ORDERS \
  --subjects "orders.>,events.orders" \
  --storage file \
  --retention limits \
  --defaults 2>/dev/null || run_nats stream update ORDERS --subjects "orders.>,events.orders"

run_nats stream add PAYMENTS \
  --subjects "payments.>" \
  --storage file \
  --retention limits \
  --defaults 2>/dev/null || run_nats stream update PAYMENTS --subjects "payments.>"

run_nats stream add TELEMETRY \
  --subjects "telemetry.>,logs.app" \
  --storage file \
  --retention limits \
  --defaults 2>/dev/null || run_nats stream update TELEMETRY --subjects "telemetry.>,logs.app"

run_nats consumer add ORDERS orders-fulfillment \
  --pull \
  --filter "orders.created" \
  --ack explicit \
  --deliver all \
  --defaults 2>/dev/null || true

run_nats consumer add ORDERS orders-analytics \
  --pull \
  --filter "orders.>" \
  --ack explicit \
  --deliver all \
  --defaults 2>/dev/null || true

run_nats consumer add PAYMENTS payment-processor \
  --pull \
  --filter "payments.card" \
  --ack explicit \
  --deliver all \
  --defaults 2>/dev/null || true

run_nats consumer add PAYMENTS payment-audit \
  --pull \
  --filter "payments.>" \
  --ack explicit \
  --deliver all \
  --defaults 2>/dev/null || true

run_nats consumer add TELEMETRY metrics-aggregator \
  --pull \
  --filter "telemetry.metrics" \
  --ack explicit \
  --deliver all \
  --defaults 2>/dev/null || true

run_nats consumer add TELEMETRY log-shipper \
  --pull \
  --filter "logs.app" \
  --ack explicit \
  --deliver all \
  --defaults 2>/dev/null || true

echo "→ Publishing sample messages"
run_nats pub orders.created '{"id":1001,"item":"widget"}'
run_nats pub orders.shipped '{"id":1001,"carrier":"fedex"}'
run_nats pub events.orders '{"type":"order.placed"}'
run_nats pub payments.card '{"amount":42.5,"currency":"USD"}'
run_nats pub payments.refund '{"amount":10}'
run_nats pub telemetry.metrics '{"cpu":0.42,"mem":0.61}'
run_nats pub logs.app '{"level":"info","msg":"topology demo ready"}'

echo ""
echo "Demo topology seeded:"
echo "  ORDERS      ← orders.>, events.orders"
echo "    ├─ orders-fulfillment  (filter: orders.created)"
echo "    └─ orders-analytics    (filter: orders.>)"
echo "  PAYMENTS    ← payments.>"
echo "    ├─ payment-processor   (filter: payments.card)"
echo "    └─ payment-audit       (filter: payments.>)"
echo "  TELEMETRY   ← telemetry.>, logs.app"
echo "    ├─ metrics-aggregator  (filter: telemetry.metrics)"
echo "    └─ log-shipper         (filter: logs.app)"
echo ""
echo "Open http://localhost:8080/topology (login: admin / admin)"
