# Changelog

All notable changes to NATS Consol are documented in this file.

## [0.4.0] - 2026-06-28

### Added

- **Topology page** — stream overview table, focused stream detail, flow diagram, and grouped subjects/consumers for large clusters
- **Supercluster view** — gateway, route, leafnode, and stream replication API + UI
- **Continuous profiling** — runtime profiles, download endpoints, and admin Profiling page
- **AI assistant** — cluster-scoped chat panel with sanitized context
- **Root user access rules** — migration `005_user_root_access_rules.sql` and scoped cluster/connection visibility
- **Contract tests** — JSON shape guarantees (camelCase keys, non-null arrays) for web API compatibility
- **GitHub Actions CI** — lint and test workflow (replaces GitLab CI)
- **Documentation** — getting started, user guide, developer setup, and DevOps guides under `docs/`

### Changed

- **Settings page removed** — appearance simplified to theme switcher only; Inter + JetBrains Mono as default fonts; animations always enabled
- **Theme switcher** — moved to top-right content bar
- **Sidebar** — fixed width overflow and collapse button; hide “Show menu” when sidebar is open
- **RBAC hardening** — route guards for audit/users pages; scoped cluster and audit lists; stale JWT permissions rejected on user load failure
- **API lists** — pagination and list handlers never return JSON `null` arrays
- **Topology / supercluster helpers** — null-safe parsing and improved large-topology layout

### Fixed

- Live stream WebSocket JSON parse errors; stream consumer error surfacing; audit empty state on query failure
- KV/consumer write actions gated on `canWrite`; profiling error vs disabled state
- SPA static file handler path traversal (`safeStaticFilePath`)
- NATS testcontainer monitoring port in integration/contract tests

### Security

- Safe resolution of static asset paths under `STATIC_DIR` (blocks directory traversal)
- Safer pprof profile parameter extraction

## [0.3.0] - 2026-06-28

### Added

- **Credential encryption at rest** — AES-GCM via `ENCRYPTION_KEY`; cluster tokens encrypted in PostgreSQL, never returned in API responses
- **Versioned migrations** — `schema_migrations` table; only new SQL files applied on startup
- **Audit log** — Postgres-backed audit trail for mutating operations; `GET /api/v1/audit` (admin-only)
- **OIDC authentication** — SSO login flow with Basic auth fallback (`BASIC_AUTH_ENABLED`), session cookies, and `GET /api/v1/auth/config`
- **RBAC** — `admin`, `operator`, and `viewer` roles enforced on all routes
- **Prometheus metrics** — `/metrics` with HTTP and NATS operation counters
- **Structured logging** — `slog` with request IDs propagated to logs and audit entries
- **Readiness health check** — `GET /api/health` reports Postgres and default NATS cluster status
- **Helm chart** — `deploy/helm/nats-consol` with Deployment, Service, Ingress, Secrets, probes
- **Enterprise UI** — Audit Log page, Users & Roles page, logout, 401 redirect, role-based action hiding
- **OpenAPI v0.3** — auth, audit, metrics endpoints; spec served at `GET /api/openapi.yaml`

### Changed

- Production (`ENV=production`) requires `ENCRYPTION_KEY`
- CORS no longer sends invalid `Allow-Credentials: true` with `Allow-Origin: *`
- Admin user seeded from `ADMIN_USERNAME` / `ADMIN_PASSWORD` on first startup for backward compatibility
- Request handling uses context timeouts for NATS and monitoring calls

### Security

- Cluster tokens decrypted only when connecting to NATS (never exposed via GET)
- Session JWT stored in HTTP-only cookie; WebSocket still supports Basic auth query param

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
- **20 UI themes** — selectable appearance presets
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
