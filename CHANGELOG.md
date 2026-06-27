# Changelog

All notable changes to NATS Consol are documented in this file.

## [0.2.0] - 2026-06-27

### Added

- **Multi-cluster registry** backed by PostgreSQL (`clusters` table, migrations, CRUD API)
- **Cluster-scoped API** — all JetStream operations under `/api/v1/clusters/{id}/...`
- **Default cluster bootstrap** from env (`NATS_URL`, `NATS_MONITORING_URL`, etc.) for backward-compatible docker-compose
- **Consumer CRUD** — create, get, delete consumers; stream update (PUT)
- **Message browser v2** — prev/next navigation, JSON/raw toggle
- **Live mode** — WebSocket tail at `/api/v1/clusters/{id}/live/ws`
- **KV Store** — bucket and key management API + UI
- **Object Store** — bucket and object management API + UI
- **OpenAPI spec** at `api/openapi.yaml`
- **Multi-cluster UI** — cluster picker (localStorage), Clusters page, dashboard with jsz metrics
- **Integration tests** with testcontainers (NATS + PostgreSQL)
- **GitLab CI** pipeline (`.gitlab-ci.yml`)

### Changed

- **Breaking:** v0.1 flat API paths removed; use cluster-scoped paths (default cluster is auto-seeded)
- Docker Compose now includes PostgreSQL service
- Config adds `DATABASE_URL`, `ENCRYPTION_KEY`, `DEFAULT_CLUSTER_NAME`

### Migration from v0.1

1. Add PostgreSQL (or use updated `docker compose up`)
2. Replace `/api/v1/streams` with `/api/v1/clusters/{clusterId}/streams`
3. Fetch cluster list via `GET /api/v1/clusters` — the default cluster matches your existing `NATS_URL` env

## [0.1.0] - Initial release

- Dashboard, streams CRUD, message browser, basic auth, Docker Compose quickstart
