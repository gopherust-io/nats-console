# NATS Consol

Open-source, self-hosted admin console for **NATS JetStream** — [github.com/gopherust-io/nats-consol](https://github.com/gopherust-io/nats-consol).

Manage streams, consumers, browse messages, tail live traffic, manage KV/Object stores, and monitor multi-cluster deployments from a modern web UI — without exposing NATS monitoring ports to the public internet.

## Features (v0.2)

- **Multi-cluster registry** with PostgreSQL persistence
- Dashboard with JetStream account usage, server info, and jsz metrics
- Stream list, create, update, delete, purge
- Consumer CRUD with detail pages
- Message browser with prev/next navigation and JSON/raw view
- **Live mode** — real-time WebSocket tail per stream
- **KV Store** and **Object Store** management
- Cluster connectivity testing
- Basic auth for the console UI/API
- OpenAPI spec (`api/openapi.yaml`)
- Docker Compose quickstart with NATS + PostgreSQL + JetStream

## Quick Start

```bash
git clone https://github.com/gopherust-io/nats-consol.git
cd nats-consol
docker compose up --build
```

Open http://localhost:8080 and sign in:

- **Username:** `admin`
- **Password:** `admin`

NATS is exposed locally on:

- Client: `nats://localhost:4222`
- Monitoring: `http://localhost:8222`

PostgreSQL: `postgres://natsconsol:natsconsol@localhost:5432/natsconsol`

On first startup, a **default cluster** is seeded from `NATS_URL` / `NATS_MONITORING_URL` env vars (backward compatible with v0.1 docker-compose).

## Architecture

```
Browser → NATS Consol (Go API + React UI) → PostgreSQL (cluster registry)
                                          → NATS JetStream (4222)
                                          → NATS Monitoring (8222)
```

The UI never talks to NATS directly. The backend acts as a secure gateway. All JetStream operations are **cluster-scoped**:

```
/api/v1/clusters/{clusterId}/streams
/api/v1/clusters/{clusterId}/live/ws
/api/v1/clusters/{clusterId}/kv/buckets
/api/v1/clusters/{clusterId}/objects/buckets
```

## Local Development

Requirements:

- Go 1.26+
- Node.js 22+
- PostgreSQL 16+
- NATS Server with JetStream enabled

Start dependencies:

```bash
docker compose up postgres nats -d
```

Backend:

```bash
cp .env.example .env   # optional local overrides
go generate ./...      # after changing internal/config/config.go
export DATABASE_URL=postgres://natsconsol:natsconsol@localhost:5432/natsconsol?sslmode=disable
export NATS_URL=nats://localhost:4222
export NATS_MONITORING_URL=http://localhost:8222
export AUTH_ENABLED=false
go run ./cmd/server
```

Config is loaded via [gopherust-io/env](https://github.com/gopherust-io/env) (`envgen` codegen). Install the generator once:

```bash
go install github.com/gopherust-io/env/cmd/envgen@latest
```

Frontend:

```bash
cd web
npm install
npm run dev
```

Frontend dev server: http://localhost:5173 (proxies `/api` to `:8080`)

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_ADDR` | `:8080` | Console HTTP listen address |
| `DATABASE_URL` | `postgres://natsconsol:natsconsol@localhost:5432/natsconsol?sslmode=disable` | PostgreSQL connection string |
| `ENCRYPTION_KEY` | — | Optional key for credential encryption (future) |
| `DEFAULT_CLUSTER_NAME` | `default` | Name for env-seeded default cluster |
| `NATS_URL` | `nats://localhost:4222` | NATS client URL (default cluster seed) |
| `NATS_MONITORING_URL` | `http://localhost:8222` | NATS monitoring HTTP URL |
| `NATS_CREDS_FILE` | — | Optional creds file path |
| `NATS_TOKEN` | — | Optional NATS token |
| `STATIC_DIR` | — | Path to built frontend (`web/dist`) |
| `AUTH_ENABLED` | `true` | Enable basic auth |
| `ADMIN_USERNAME` | `admin` | Console admin username |
| `ADMIN_PASSWORD` | `admin` | Console admin password |
| `REQUEST_TIMEOUT` | `10s` | HTTP client timeout for NATS monitoring calls |

## API

See [`api/openapi.yaml`](api/openapi.yaml) for the full v0.2 contract.

Key endpoints:

- `GET /api/health`
- `GET/POST /api/v1/clusters`
- `GET/PUT/DELETE /api/v1/clusters/{id}`
- `POST /api/v1/clusters/{id}/test`
- `GET /api/v1/clusters/{id}/account`
- `GET/POST /api/v1/clusters/{id}/streams`
- `PUT/DELETE /api/v1/clusters/{id}/streams/{name}`
- `POST/GET/DELETE /api/v1/clusters/{id}/streams/{name}/consumers[/{consumer}]`
- `GET /api/v1/clusters/{id}/streams/{name}/messages?seq=&direction=`
- `GET /api/v1/clusters/{id}/live/ws?stream=`
- `GET/POST /api/v1/clusters/{id}/kv/buckets[/{bucket}/keys/{key}]`
- `GET/POST /api/v1/clusters/{id}/objects/buckets[/{bucket}/objects/{name}]`
- `GET /api/v1/clusters/{id}/monitoring/varz|jsz`

## Testing

```bash
go test ./...
make test-integration   # requires Docker (testcontainers)
```

Set `SKIP_TESTCONTAINERS=1` to skip integration tests that need Docker.

## Roadmap (v0.3+)

- Account/JWT management
- RBAC / SSO (OIDC)
- Credential encryption at rest
- Helm chart
- Historical metrics (Prometheus)
- CLI generated from OpenAPI

## License

Apache 2.0 — see [LICENSE](LICENSE).
