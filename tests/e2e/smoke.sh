#!/usr/bin/env bash
# Smoke / E2E / acceptance tests against a running stack (docker compose).
# Usage: BASE_URL=http://localhost:8080 ./tests/e2e/smoke.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
AUTH="${AUTH:-admin:admin}"

echo "==> smoke: health"
curl -sf "${BASE_URL}/api/health" | grep -q '"ok"'

echo "==> smoke: auth config"
curl -sf "${BASE_URL}/api/v1/auth/config" | grep -q '"authEnabled"'

echo "==> smoke: login + clusters (acceptance)"
cookie_jar="$(mktemp)"
trap 'rm -f "$cookie_jar"' EXIT

curl -sf -c "$cookie_jar" -X POST "${BASE_URL}/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"${AUTH%%:*}\",\"password\":\"${AUTH#*:}\"}" \
  | grep -q '"username"'

curl -sf -b "$cookie_jar" "${BASE_URL}/api/v1/clusters" | grep -q '"clusters"'

cluster_id="$(curl -sf -b "$cookie_jar" "${BASE_URL}/api/v1/clusters" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -1)"
if [[ -z "$cluster_id" ]]; then
  echo "no cluster id found" >&2
  exit 1
fi

echo "==> smoke: streams list (cluster ${cluster_id})"
curl -sf -b "$cookie_jar" "${BASE_URL}/api/v1/clusters/${cluster_id}/streams" | grep -q '"streams"'

echo "==> smoke: create live stream"
if ! curl -sf -b "$cookie_jar" -X POST "${BASE_URL}/api/v1/clusters/${cluster_id}/streams" \
  -H 'Content-Type: application/json' \
  -d '{"name":"LIVE_SMOKE","subjects":["live.>"]}'; then
  curl -sf -b "$cookie_jar" "${BASE_URL}/api/v1/clusters/${cluster_id}/streams" | grep -q 'LIVE_SMOKE'
fi

echo "==> smoke: live websocket"
AUTH="${AUTH}" go run ./tests/e2e/ws_check.go "${BASE_URL}" "${cluster_id}" "${cookie_jar}"

echo "==> smoke: openapi spec"
curl -sf "${BASE_URL}/api/openapi.yaml" | grep -q 'openapi:'

echo "All smoke checks passed."
