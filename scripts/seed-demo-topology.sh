#!/usr/bin/env sh
# Seed JetStream demo streams, consumers, and sample messages for the Topology UI.
# Names align with examples/fleet (32-service live load demo).
# Requires: docker compose up (nats service running)
set -eu

NATS_URL="${NATS_URL:-nats://nats:4222}"
NETWORK="${DOCKER_NETWORK:-nats-consol_default}"
IMAGE="${NATS_BOX_IMAGE:-natsio/nats-box:0.14.5}"

run_nats() {
  docker run --rm --network "$NETWORK" "$IMAGE" nats -s "$NATS_URL" "$@"
}

add_stream() {
  name="$1"
  subjects="$2"
  retention="${3:-limits}"
  run_nats stream add "$name" \
    --subjects "$subjects" \
    --storage file \
    --retention "$retention" \
    --defaults 2>/dev/null || run_nats stream update "$name" --subjects "$subjects" || true
}

add_pull() {
  stream="$1"
  durable="$2"
  filter="$3"
  run_nats consumer add "$stream" "$durable" \
    --pull \
    --filter "$filter" \
    --ack explicit \
    --deliver all \
    --defaults 2>/dev/null || true
}

echo "→ Seeding demo JetStream topology on $NATS_URL (aligned with examples/fleet)"

# WorkQueue job streams (ORDERS uses explicit subjects so ORDERS_SHARD can own orders.shard.>)
add_stream ORDERS "orders.created,orders.updated,orders.cancelled" work
add_stream ORDERS_SHARD "orders.shard.>" work
add_stream PAYMENTS "payments.>" work
add_stream INVENTORY "inventory.>" work
add_stream SHIPPING "shipping.>" work
add_stream BILLING "billing.>" work
add_stream FRAUD "fraud.>" work
add_stream SEARCH "search.>" work
add_stream WEBHOOKS "webhooks.>" work
add_stream RETURNS "returns.>" work
add_stream CART "cart.>" work
add_stream MEDIA "media.>" work

# Limits / fan-out / DLQ (DLQ outside orders.* to avoid subject overlap)
add_stream ORDERS_DLQ "dlq.orders.>" limits
add_stream NOTIFICATIONS "notify.>" limits
add_stream AUDIT "audit.>" limits
add_stream CATALOG "catalog.>" limits
add_stream USERS "users.>" limits
add_stream RECO "reco.>" limits
add_stream LOYALTY "loyalty.>" limits
add_stream PRICING "pricing.>" limits

# Interest
add_stream TELEMETRY "telemetry.>,logs.app" interest

# Only pull durables that the live fleet also uses as pull (avoid clashing with
# push QueueSubscribeBound durables of the same name). Push workers appear when
# the fleet process is running.
add_pull ORDERS order-projector "orders.updated"
add_pull SEARCH search-indexer "search.index"
add_pull AUDIT audit-logger "audit.>"
add_pull TELEMETRY metrics-aggregator "telemetry.metrics"
add_pull TELEMETRY log-shipper "logs.app"
add_pull CATALOG catalog-sync "catalog.>"
add_pull USERS user-projector "users.>"
add_pull LOYALTY loyalty-worker "loyalty.>"
add_pull PRICING pricing-engine "pricing.>"

echo "→ Publishing sample messages"
run_nats pub orders.created '{"id":1001,"item":"widget","service":"seed"}'
run_nats pub orders.updated '{"id":1001,"service":"seed"}'
run_nats pub orders.cancelled '{"id":1002,"service":"seed"}'
run_nats pub orders.shard.0.created '{"id":1003,"shard_key":"user-1","service":"seed"}'
run_nats pub orders.shard.1.created '{"id":1004,"shard_key":"user-2","service":"seed"}'
run_nats pub payments.authorize '{"amount":42.5,"currency":"USD","service":"seed"}'
run_nats pub payments.capture '{"amount":42.5,"service":"seed"}'
run_nats pub inventory.reserve '{"sku":"SKU-1","service":"seed"}'
run_nats pub shipping.create '{"order_id":1001,"service":"seed"}'
run_nats pub billing.invoice '{"order_id":1001,"service":"seed"}'
run_nats pub fraud.check '{"payment_id":1,"service":"seed"}'
run_nats pub cart.checkout '{"cart_id":9,"service":"seed"}'
run_nats pub returns.requested '{"order_id":1001,"service":"seed"}'
run_nats pub search.index '{"entity":"product","service":"seed"}'
run_nats pub webhooks.deliver '{"url":"https://example.test/hook","service":"seed"}'
run_nats pub media.transcode '{"asset":"img-1","service":"seed"}'
run_nats pub notify.email '{"to":"demo@example.test","service":"seed"}'
run_nats pub notify.sms '{"to":"+10000000000","service":"seed"}'
run_nats pub notify.push '{"device":"demo","service":"seed"}'
run_nats pub audit.order '{"id":1001,"service":"seed"}'
run_nats pub telemetry.metrics '{"cpu":0.42,"mem":0.61,"service":"seed"}'
run_nats pub logs.app '{"level":"info","msg":"topology demo ready","service":"seed"}'
run_nats pub catalog.updated '{"sku":"SKU-1","service":"seed"}'
run_nats pub users.created '{"user_id":7,"service":"seed"}'
run_nats pub loyalty.enroll '{"user_id":7,"service":"seed"}'
run_nats pub pricing.updated '{"sku":"SKU-1","service":"seed"}'
run_nats pub reco.refresh '{"sku":"SKU-1","service":"seed"}'
run_nats pub dlq.orders.poison '{"id":17,"reason":"seed-sample","service":"seed"}'

echo ""
echo "Demo topology seeded (21 streams, fleet-aligned pull consumers)."
echo "For continuous load, RPC, and feature showcases, run the live fleet:"
echo "  make nats-cluster-up && make fleet-up"
echo "  # or: TEL_ENABLE=false go run ./examples/fleet"
echo ""
echo "Open http://localhost:8080/topology (login: admin / admin)"
