#!/usr/bin/env bash
# Stress / high-rate load baseline using vegeta.
# Same targets as load.sh with higher rate and looser latency thresholds.
#
# Usage:
#   docker compose up -d
#   ./tests/performance/stress.sh
#
# Override via env:
#   RATE=150 DURATION=45s PERF_MIN_RPS=50 PERF_MAX_P99_MS=5000 ./tests/performance/stress.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
DURATION="${DURATION:-30s}"
RATE="${RATE:-100}"
AUTH="${AUTH:-admin:admin}"

if ! command -v vegeta >/dev/null 2>&1; then
  echo "vegeta not found; install from https://github.com/tsenart/vegeta" >&2
  exit 1
fi

TARGETS="$(mktemp)"
REPORT="$(mktemp)"
trap 'rm -f "$TARGETS" "$REPORT"' EXIT

B64="$(printf '%s' "$AUTH" | base64 | tr -d '\n')"

cat >"$TARGETS" <<EOF
GET ${BASE_URL}/api/health
GET ${BASE_URL}/api/v1/auth/config
GET ${BASE_URL}/api/v1/clusters
Authorization: Basic ${B64}
EOF

echo "==> stress: ${RATE} req/s for ${DURATION} against ${BASE_URL}"
vegeta attack -duration="$DURATION" -rate="$RATE" -targets="$TARGETS" | vegeta report -type=text >"$REPORT"
cat "$REPORT"

MIN_RPS="${PERF_MIN_RPS:-50}"
MAX_P99_MS="${PERF_MAX_P99_MS:-5000}"

success_rate="$(grep '^Success' "$REPORT" | awk '{print $NF}' | tr -d '%')"
throughput="$(grep '^Requests' "$REPORT" | sed -E 's/^Requests[[:space:]]+\[[^]]+\][[:space:]]+//' | awk -F',' '{gsub(/ /, "", $2); print $2}')"
p99_ms="$(grep '^Latencies' "$REPORT" | sed -E 's/^Latencies[[:space:]]+\[[^]]+\][[:space:]]+//' | awk -F',' '{gsub(/ms| /, "", $4); print $4}')"

if [[ -z "$success_rate" || -z "$throughput" || -z "$p99_ms" ]]; then
  echo "failed to parse vegeta report" >&2
  exit 1
fi

if awk -v s="$success_rate" -v min=95 'BEGIN { exit (s+0 >= min) ? 0 : 1 }'; then
  echo "success rate OK (${success_rate}%)"
else
  echo "success rate below 95%: ${success_rate}%" >&2
  exit 1
fi

if awk -v r="$throughput" -v min="$MIN_RPS" 'BEGIN { exit (r+0 >= min) ? 0 : 1 }'; then
  echo "throughput OK (${throughput} req/s >= ${MIN_RPS} req/s)"
else
  echo "throughput below threshold: ${throughput} req/s < ${MIN_RPS} req/s" >&2
  exit 1
fi

if awk -v p="$p99_ms" -v max="$MAX_P99_MS" 'BEGIN { exit (p+0 <= max) ? 0 : 1 }'; then
  echo "p99 latency OK (${p99_ms}ms <= ${MAX_P99_MS}ms)"
else
  echo "p99 latency above threshold: ${p99_ms}ms > ${MAX_P99_MS}ms" >&2
  exit 1
fi

echo "Stress baseline passed."
