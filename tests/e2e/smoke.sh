#!/usr/bin/env bash
# Smoke / E2E / acceptance tests against a running stack (docker compose).
# Usage:
#   BASE_URL=http://localhost:8080 ./tests/e2e/smoke.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
AUTH="${AUTH:-admin:admin}"

CURL_OPTS=()
if [[ "${BASE_URL}" == https://* ]]; then
  # Optional: skip TLS verify when smoke runs behind an external HTTPS proxy with a lab cert.
  CURL_OPTS+=(-k)
fi

curl_sf() {
  # Avoid unbound-variable errors with empty arrays under `set -u`.
  if ((${#CURL_OPTS[@]})); then
    curl -sf "${CURL_OPTS[@]}" "$@"
  else
    curl -sf "$@"
  fi
}

csrf_from_jar() {
  local jar="$1"
  # Netscape cookie jar: domain flag path secure expiry name value
  awk -F'\t' '$6 == "nats_consol_csrf" { print $7; exit }' "$jar"
}

echo "==> smoke: health"
curl_sf "${BASE_URL}/api/health" | grep -q '"ok"'

echo "==> smoke: auth config"
curl_sf "${BASE_URL}/api/v1/auth/config" | grep -q '"authEnabled"'

echo "==> smoke: login + clusters (acceptance)"
cookie_jar="$(mktemp)"
trap 'rm -f "$cookie_jar"' EXIT

curl_sf -c "$cookie_jar" -X POST "${BASE_URL}/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"${AUTH%%:*}\",\"password\":\"${AUTH#*:}\"}" \
  | grep -q '"username"'

csrf="$(csrf_from_jar "$cookie_jar")"
if [[ -z "$csrf" ]]; then
  echo "csrf cookie missing after login" >&2
  exit 1
fi

# API responses use { data, meta } envelopes (not legacy { clusters } / { streams }).
curl_sf -b "$cookie_jar" "${BASE_URL}/api/v1/clusters" | grep -q '"data"'

cluster_id="$(curl_sf -b "$cookie_jar" "${BASE_URL}/api/v1/clusters" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -1)"
if [[ -z "$cluster_id" ]]; then
  echo "no cluster id found" >&2
  exit 1
fi

echo "==> smoke: streams list (cluster ${cluster_id})"
curl_sf -b "$cookie_jar" "${BASE_URL}/api/v1/clusters/${cluster_id}/streams" | grep -q '"data"'

echo "==> smoke: create live stream"
if ! curl_sf -b "$cookie_jar" -X POST "${BASE_URL}/api/v1/clusters/${cluster_id}/streams" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: ${csrf}" \
  -d '{"name":"LIVE_SMOKE","subjects":["live.>"]}'; then
  curl_sf -b "$cookie_jar" "${BASE_URL}/api/v1/clusters/${cluster_id}/streams" | grep -q 'LIVE_SMOKE'
fi

echo "==> smoke: live websocket"
AUTH="${AUTH}" TLS_INSECURE="${TLS_INSECURE:-}" go run ./tests/e2e/ws_check.go "${BASE_URL}" "${cluster_id}" "${cookie_jar}"

echo "==> smoke: openapi spec"
# Download fully before grepping: pipefail + grep -q would SIGPIPE curl on large YAML.
openapi_tmp="$(mktemp)"
curl_sf "${BASE_URL}/api/openapi.yaml" -o "$openapi_tmp"
# swag emits OpenAPI 2 (swagger: "2.0"); accept either 2.x or 3.x docs.
if ! grep -qE '^(openapi:|swagger:)' "$openapi_tmp"; then
  echo "openapi/swagger document marker missing" >&2
  rm -f "$openapi_tmp"
  exit 1
fi
rm -f "$openapi_tmp"

echo "All smoke checks passed."
