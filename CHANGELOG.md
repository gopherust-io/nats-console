# Changelog

All notable changes to NATS Consol are documented in this file.

## [Unreleased]

## [0.14.0] - 2026-08-22

### Fixed

- Pre-release bug sweep: auth sessionError no longer infinite-spins; stream lists paginate fully; Live Tail reconnect + error detail; Jsz query sanitization; production `ENV` security gates; live WS error sanitization; Access/Consumer/Invite/KV/Capsule error UX; architecture pages no longer auto-fallback to sample on live errors.
- Dockerfile Go **1.27.0**; drop local `replace` for `gopherust-io/nats` (published v0.6.0); sonic Go 1.27-capable pin; pprof off by default in compose/`.env.example`.
- Docs: `go run ./cmd` (not `./cmd/server`); OpenAPI/`web` package version **0.14.0**.
- golangci `modernize` / errcheck cleanups so CI stays green on golangci-lint v2.13+.
- Skip SMTP dial when `SMTP_ENABLED=false` so compose smoke works without the `mail` profile.

### Changed

- OpenAPI is generated with **swag** (`make openapi` → `api/swagger.yaml`, embedded and served at `GET /api/openapi.yaml`). All router-registered API routes are annotated; CI checks for drift.
- Depend on [nats](https://github.com/gopherust-io/nats) **v0.6.0** (capsules, RR/pub compression, removed config presets). Dropped local `replace => ../nats`.
- Removed system **Usage** tab (limits already on Account Overview and JetStream hub).
- Replicas RAFT labels: rename Hot standby → **Follower** (lag still via Status “not current”).
- Dependency bumps: nats.go 1.53.1, x/crypto 0.55.0, testify 1.12.1, testcontainers 0.44.0, vite 8.2.1, motion 13.1.0, eslint group, Dompurify, GitHub Actions.

### Added

- **Incident Capsule Studio** — capture/list/load/dry-run via `Incidents()`; DLQ capture CTA + consumer panel.

## [0.13.0] - 2026-08-02

### Added

- **Fleet lab** — `examples/fleet/` multi-service JetStream topology (compose, RPC, work queues) for local demos.
- **HTTP response compression** middleware (`internal/api/middleware/compress.go`).

### Changed

- Depend on [env](https://github.com/gopherust-io/env) **v0.6.0**, [tel](https://github.com/gopherust-io/tel) **v0.4.0**, [nats](https://github.com/gopherust-io/nats) **v0.5.0**; goalign **v1.4.0**.
- Regenerate config loader for env v0.6; `config.Load` reloads Snapshot after dotenv.
- API/auth/replicas/UI and docs hardening across monitoring, RBAC, and assistant surfaces.

## [0.12.0] - 2026-08-01

### Added

- **Hidden Bottlenecks** — Docs hub mines recurring weekday×hour patterns from hourly rollups (lag × avg payload, optional fingerprint processing latency); not lag/CPU threshold alerts. `GET/POST …/hidden-bottlenecks` (+ `/ask`); sample Friday 18:00 showcase; Consumer Detail chip when matched (`METRICS_SNAPSHOT_BOTTLENECK_RETENTION`, default `672h`)
- **Chaos Story Generator** — Docs hub invents realistic multi-act disasters from live inventory names (`GET …/chaos-story`, `POST …/chaos-story/generate`, `GET /api/v1/chaos-story/demo`); Black Friday sample; **Simulate** is a client-side narrative playbook only (no fault injection)
- **Refresh tokens + device fingerprint** — short-lived RS256 access JWT (`SESSION_TTL`, default `15m`) plus opaque `nats_consol_refresh` cookie (`REFRESH_TOKEN_TTL`, default `168h`); access JWT claim `fph` and refresh rows bind to `SHA-256(User-Agent|ClientIP)`; `POST /api/v1/auth/refresh` rotates with CSRF; reuse of a replaced refresh revokes the user’s refresh family; SPA silently refreshes on 401
- **Event Catalog** — Swagger-style subject docs (`GET/PUT/DELETE …/event-catalog`): auto-discover concrete subjects from JetStream, enrich with owner / description / JSON Schema / example / deprecation in Postgres, and list matching consumers (`/docs/event-catalog`)
- **Event Wikipedia** — auto-assembled per-subject docs (`GET …/event-wikipedia`) under Docs: Purpose, History, Owner, Consumers, Examples, Schema, Related Events (Event Genome), Known Incidents (Audit deep links), Deprecation Status (`/docs/event-wikipedia`)
- **Consumer behavior fingerprinting** — Consumer Detail shows Normal vs Current msg/min and processing latency from worker-published JetStream KV (`nats_consol_fingerprints`, configurable via `BEHAVIOR_FINGERPRINT_KV_BUCKET`); anomaly chip when workers report a regression
- **Subject naming** — Topology **Subject naming** tab (`GET …/subject-naming`) flags wrong case, missing dots, non-dot separators, shallow hierarchy, and inconsistent subject variants with suggested `dot.lower` normalizations
- **Event genome** — Topology **Event genome** tab (`GET …/event-genome`) clusters semantically duplicate subjects (action synonyms like `created`/`new`, singular/plural) with a suggested canonical form
- **Slow consumer detection** — thresholds on pending / lag / ack-pending ratio; consumer API flags + UI badges; topology warnings use the same thresholds; alert metrics `jetstream.slow_consumers`, `jetstream.consumer_max_lag`, `jetstream.consumer_max_pending`, `jetstream.consumer_max_ack_pending`
- **Connection Inspector** — expanded Connections view with RTT, IP, TLS version, user/account, connected-since, published/received counters, and slow-consumer indicator from `connz`

### Removed

- HTTP/3 / QUIC support: in-process listener, outbound h3 transport, Compose Caddy edge, Helm `http3` gateway, `h3_check`, and `open-https-h3.sh`
- Related `HTTP3_*` config/env vars

### Changed

- Depend on [tel](https://github.com/gopherust-io/tel) **v0.3.0**, [env](https://github.com/gopherust-io/env) **v0.5.0**, [nats](https://github.com/gopherust-io/nats) **v0.4.0**; goalign **v1.3.0**
- **Architecture Review** — Docs-only under `/docs/architecture-review` (removed Topology tab); `/admin/topology?view=review` redirects to Docs
- **HTTP session JWTs** — login/invite sessions are **RS256** (RSA ≥2048) JWTs via `SESSION_PRIVATE_KEY` / `SESSION_PUBLIC_KEY` (replaces `SESSION_SECRET` HS256). Cookie delivery unchanged; `Authorization: Bearer` accepted for the same token. Existing HS256 cookies are invalid after upgrade (re-login).
- Bumped Go module dependencies (`go get -u` / tidy) and web packages to latest (Vite 8, Recharts 3, ESLint 10); TypeScript held at 6.0.x for `typescript-eslint` peer compatibility
- Database migrations now use **pressly/goose**; startup still applies pending SQL from `migrations/`, with a one-time bridge from legacy `schema_migrations` to `goose_db_version`
- Local Compose defaults to plain **http://localhost:8080** (`PUBLIC_BASE_URL`); TLS termination is left to an external reverse proxy or Ingress

## [0.11.0] - 2026-07-26

### Added

- Centered Sign In page and invite-accept flow (`/invite/:token`)
- People invite links (`POST /api/v1/people/invite`) with basic-auth password set (no Keycloak required)
- System/Account **Access** grants (`access_grants`) with Access roles and middleware enforcement
- Signing key groups, NATS user detail (rotate NKey, mint JWT via `NATS_ACCOUNT_SEED`, Assign Person)
- **In-app alerts** — metric threshold rules, snapshot-driven evaluation, open/closed feed, acknowledge, topbar bell badge
- **Alert email notifications** — optional SMTP delivery to console users when an alert first opens
- Optional **Mailpit** Compose profile (`--profile mail`) plus Mailpit/Gmail SMTP recipes in `.env.example` and devops docs

### Changed

- Default `docker compose up` no longer starts Keycloak; use `--profile sso` for optional OIDC demo
- Default Compose stack starts **Caddy** (HTTPS + HTTP/3 on `:443`); `PUBLIC_BASE_URL` defaults to `https://localhost`
- Helm sets `PUBLIC_BASE_URL=https://{{ http3.host }}` when `http3.enabled=true` unless `config.publicBaseUrl` is set
- `scripts/open-https-h3.sh` opens Chrome with QUIC flags so DevTools Protocol shows `h3` for local Caddy
- `h3_check` now performs a real HTTP/3 (QUIC) dial in addition to Alt-Svc checks
- Admin **Console Users** label; Access tabs on system and account levels
- Profiling is backend-only and on-demand (`/api/v1/pprof/*`, non-prod `/debug/pprof`); removed continuous sampler and Profiling UI

### Removed

- Admin Profiling page and continuous `PPROF_CONTINUOUS*` settings

## [0.10.0] - 2026-07-25

### Changed

- NATS client path uses [`github.com/gopherust-io/nats`](https://github.com/gopherust-io/nats) v0.2.0 (streams/consumers/KV/object store/monitoring).
- Process metrics moved from Prometheus `/metrics` to [tel](https://github.com/gopherust-io/tel) OTLP; JetStream `/metrics/history` unchanged.
- Logging remains zerolog.

## [0.8.0] - 2026-06-28

### Added

- **HTTP/3 + QUIC** — optional Caddy reverse proxy (`docker compose --profile http3`) and Helm `http3` gateway Deployment
- Outbound HTTP/3 transport with fallback for OIDC and AI assistant (`HTTP3_OUTBOUND_*`)
- Optional in-process HTTP/3 listener proxying to fasthttp (`HTTP3_ENABLED`)
- E2E `h3_check` smoke helper and Alt-Svc verification in `smoke.sh`

### Changed

- DevOps docs cover UDP 443, WebSocket compatibility, and protocols that remain HTTP/1.1 (NATS monitoring)

## [0.7.0] - 2026-06-28

### Added

- **Historical metrics** — background Postgres snapshots of JetStream account, varz, and jsz metrics
- `GET /api/v1/clusters/{id}/metrics/history` with downsampling and counter deltas
- Dashboard **Trends** section with time-range charts (1h / 6h / 24h / 7d)
- Migration `007_cluster_metrics.sql`, retention cleanup, Prometheus snapshot counters

### Changed

- Configurable snapshot collector via `METRICS_SNAPSHOT_*` env vars

## [0.6.0] - 2026-06-28

### Added

- **Message publish** — `POST /api/v1/clusters/{id}/streams/{name}/messages` and publish form on stream detail page
- **Encryption key rotation** — root-only `POST /api/v1/admin/rotate-encryption-key` with dry-run support
- **JWT resolver** — import/list/delete/export account JWTs per cluster (`/resolver/*`) + JWT Resolver UI page
- Integration tests for publish, resolver, and encryption rotation guard

### Changed

- Router passes store to admin/resolver handlers; OpenAPI documents publish and admin endpoints

## [0.5.0] - 2026-06-28

### Added

- **Multi-tenant RBAC** — `accessRules.clusterIds` enforced for operator, viewer, and scoped admin; cluster picker on Users page
- **Supercluster reliability** — per-source errors and `warnings` in API/UI for partial monitoring failures
- **Live WebSocket hardening** — single-writer pattern, read deadlines, NATS disconnect handling, max-message cap
- **CI race job** — `-race` detector for `internal/live`
- Security/integration tests for scoped operator/viewer and cluster create blocking

### Changed

- **Breaking:** Non-root users with empty `clusterIds` have **no cluster access** (previously operator/viewer saw all clusters). Assign clusters before/after upgrade.
- Scoped admins cannot `POST /api/v1/clusters`; only root and legacy unscoped admin can register clusters
- `/metrics` auth defaults to **on** in production (`METRICS_AUTH_ENABLED=true` required when `ENV=production`); admin/root only
- Raw `/debug/pprof` returns **404 in production**; use authenticated `/api/v1/pprof/*`
- Production config validates `PPROF_AUTH_ENABLED`, `HTTP_WRITE_TIMEOUT >= PPROF_CPU_MAX_SECONDS + buffer`
- Profiling UI links to API config instead of raw debug index

### Fixed

- Concurrent WebSocket writes and hub state races in live mode
- CPU profile collection truncated by write timeout middleware
- Supercluster handler 502 branch and silent monitoring parse failures

### Security

- Close scoped-user cluster creation bypass
- Stale JWT reload on pprof/metrics bypass routes
- Metrics restricted to admin/root when auth enabled

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
