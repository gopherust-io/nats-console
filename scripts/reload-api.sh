#!/usr/bin/env bash
# Rebuild the Linux API binary and hot-swap it into the running compose console.
# Migrations are copied too: the server reads them from disk at startup.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

container="${CONSOLE_CONTAINER:-nats-consol-console-1}"
arch="$(docker exec "$container" uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"

mkdir -p bin
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -o bin/nats-consol-linux ./cmd
docker cp bin/nats-consol-linux "$container:/app/nats-consol"
docker exec "$container" sh -c 'rm -f /app/migrations/*.sql'
docker cp migrations/. "$container:/app/migrations/"
docker restart "$container"
