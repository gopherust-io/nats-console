#!/usr/bin/env bash
# Load / throughput / performance baseline using vegeta (https://github.com/tsenart/vegeta).
# Requires: vegeta, running stack at BASE_URL, valid AUTH credentials.
#
# Usage:
#   docker compose up -d
#   ./tests/performance/load.sh
#
# Override thresholds via env:
#   PERF_MIN_RPS=50 PERF_MAX_P99_MS=500 ./tests/performance/load.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
DURATION="${DURATION:-10s}"
RATE="${RATE:-20}"
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

echo "==> performance: ${RATE} req/s for ${DURATION} against ${BASE_URL}"
vegeta attack -duration="$DURATION" -rate="$RATE" -targets="$TARGETS" | vegeta report -type=text >"$REPORT"
cat "$REPORT"

MIN_RPS="${PERF_MIN_RPS:-10}"
MAX_P99_MS="${PERF_MAX_P99_MS:-2000}"

success_rate="$(grep 'Success' "$REPORT" | awk '{print $3}' | tr -d '%')"
p99_ms="$(grep '99%' "$REPORT" | awk '{print $2}' | sed 's/ms//')"

if awk -v s="$success_rate" -v min=99 'BEGIN { exit (s+0 >= min) ? 0 : 1 }'; then
  echo "success rate OK (${success_rate}%)"
else
  echo "success rate below 99%: ${success_rate}%" >&2
  exit 1
fi

if awk -v p="$p99_ms" -v max="$MAX_P99_MS" 'BEGIN { exit (p+0 <= max) ? 0 : 1 }'; then
  echo "p99 latency OK (${p99_ms}ms <= ${MAX_P99_MS}ms)"
else
  echo "p99 latency above threshold: ${p99_ms}ms > ${MAX_P99_MS}ms" >&2
  exit 1
fi

echo "Performance baseline passed."
