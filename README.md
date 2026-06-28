# NATS Consol

Open-source, self-hosted admin console for **NATS JetStream** — [github.com/gopherust-io/nats-consol](https://github.com/gopherust-io/nats-consol).

Manage streams, consumers, browse messages, tail live traffic, manage KV/Object stores, and monitor multi-cluster deployments from a modern web UI — without exposing NATS monitoring ports to the public internet.

**📖 Documentation:** friendly guides for everyone — [docs/README.md](docs/README.md)
- [Getting started](docs/getting-started.md) · [User guide](docs/user-guide.md) · [DevOps setup](docs/devops-setup.md) · [Developer setup](docs/developer-setup.md)

## Features (v0.5)

- **Multi-cluster registry** with PostgreSQL persistence
- **Multi-tenant RBAC** — operator/viewer/admin scoped by `accessRules.clusterIds`; root and legacy unscoped admin retain full access
- **Enterprise security** — AES-GCM credential encryption, audit log, OIDC SSO, hardened pprof/metrics
- Dashboard with JetStream account usage, server info, and jsz metrics
- Stream list, create, update, delete, purge
- Consumer CRUD with detail pages
- Message browser with prev/next navigation and JSON/raw view
- **Live mode** — real-time WebSocket tail per stream (race-safe hub)
- **KV Store** and **Object Store** management
- **Supercluster view** — gateways, routes, leafnodes, replication with partial-failure warnings
- **Continuous profiling** — admin-only `/api/v1/pprof/*` (raw `/debug/pprof` disabled in production)
- **Prometheus metrics** at `/metrics` (auth on by default in production)
- **Helm chart** for Kubernetes deployment
- OpenAPI spec served at `/api/openapi.yaml`
- Docker Compose quickstart with NATS + PostgreSQL + JetStream

## Quick Start

```bash
git clone https://github.com/gopherust-io/nats-consol.git
cd nats-consol
docker compose up --build
```

Open http://localhost:8080 and sign in:

- **Basic auth:** `admin` / `admin`
- **SSO:** click **Continue with SSO** → `sso-user` / `sso-user` (Keycloak dev IdP; first login creates a `viewer`)

Add Google, GitHub, GitLab, or Microsoft buttons by setting their `OIDC_*_ENABLED` env vars — see [SSO](#sso-oidc).

NATS is exposed locally on:

- Client: `nats://localhost:4222`
- Monitoring: `http://localhost:8222`

PostgreSQL: `postgres://natsconsol:natsconsol@localhost:5432/natsconsol`

On first startup, a **default cluster** is seeded from `NATS_URL` / `NATS_MONITORING_URL` env vars (backward compatible with v0.1 docker-compose).

## Architecture

```
Browser → NATS Consol (Go API + React UI) → PostgreSQL (cluster registry, users, audit)
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

## Linting

Backend (requires [golangci-lint](https://golangci-lint.run/) v2+):

```bash
make lint-go
make lint-go-fix   # auto-fix (modernize, fieldalignment, tagalign, etc.)
```

Enabled Go linters include `modernize`, `govet`/`fieldalignment` (struct layout), `errorlint`, `gosec`, `exptostd`, `intrange`, `perfsprint`, `tagalign`, `embeddedstructfieldcheck`, and the standard set (`staticcheck`, `errcheck`, `unused`, …). Config: `.golangci.yml`.

Frontend (uses local `npm` when available, otherwise Docker):

```bash
make lint-web
# or explicitly via Docker:
make lint-web-docker
```

Both:

```bash
make lint
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_ADDR` | `:8080` | Console HTTP listen address |
| `HTTP_READ_TIMEOUT` | `10s` | HTTP server read timeout |
| `HTTP_WRITE_TIMEOUT` | `30s` | HTTP server write timeout |
| `HTTP_IDLE_TIMEOUT` | `60s` | HTTP server idle timeout |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | Public base URL when OIDC redirect URL is not set |
| `ENV` | `development` | Set to `production` to enforce encryption key |
| `DATABASE_URL` | `postgres://…` | PostgreSQL connection string |
| `DB_MAX_CONNS` | `25` | PostgreSQL connection pool max size |
| `DB_MIN_CONNS` | `2` | PostgreSQL connection pool min size |
| `DB_MAX_CONN_LIFETIME` | `1h` | Max lifetime of a pooled connection |
| `DB_MAX_CONN_IDLE_TIME` | `30m` | Max idle time before a connection is closed |
| `DB_HEALTH_CHECK_PERIOD` | `1m` | Interval between pool health checks |
| `ENCRYPTION_KEY` | — | AES-GCM key for cluster tokens (**required in production**) |
| `SESSION_SECRET` | falls back to `ENCRYPTION_KEY` | JWT session signing secret |
| `SESSION_TTL` | `8h` | Session cookie lifetime |
| `DEFAULT_CLUSTER_NAME` | `default` | Name for env-seeded default cluster |
| `NATS_URL` | `nats://localhost:4222` | NATS client URL (default cluster seed) |
| `NATS_CLIENT_CACHE_TTL` | `5m` | How long to cache NATS client connections per cluster |
| `NATS_MONITORING_URL` | `http://localhost:8222` | NATS monitoring HTTP URL |
| `NATS_CREDS_FILE` | — | Optional creds file path |
| `NATS_TOKEN` | — | Optional NATS token |
| `STATIC_DIR` | — | Path to built frontend (`web/dist`) |
| `AUTH_ENABLED` | `true` | Enable authentication |
| `ADMIN_USERNAME` | `admin` | Bootstrap admin username |
| `ADMIN_PASSWORD` | `admin` | Bootstrap admin password |
| `OIDC_ENABLED` | `false` | Enable OIDC SSO |
| `OIDC_ISSUER` | — | OIDC provider issuer URL |
| `OIDC_DISCOVERY_URL` | — | Optional internal discovery URL (Docker: `http://keycloak:8080/realms/...` when issuer is public `localhost`) |
| `OIDC_CLIENT_ID` | — | OIDC client ID |
| `OIDC_CLIENT_SECRET` | — | OIDC client secret |
| `OIDC_REDIRECT_URL` | — | OIDC callback URL (must match IdP client config) |
| `OIDC_PUBLIC_URL` | — | Public base URL for social SSO callbacks (`{url}/api/v1/auth/oidc/{provider}/callback`) |
| `OIDC_GOOGLE_ENABLED` | `false` | Enable Google SSO |
| `OIDC_GITHUB_ENABLED` | `false` | Enable GitHub SSO |
| `OIDC_GITLAB_ENABLED` | `false` | Enable GitLab SSO |
| `OIDC_MICROSOFT_ENABLED` | `false` | Enable Microsoft SSO |
| `BASIC_AUTH_ENABLED` | `true` | Allow username/password login; set `false` for SSO-only |
| `CORS_ALLOWED_ORIGINS` | — | Comma-separated allowed origins |
| `LOG_JSON` | `false` | Emit structured JSON logs |
| `LOG_LEVEL` | `info` | Log level: `trace`, `debug`, `info`, `warn`, `error`, `fatal` |
| `METRICS_AUTH_ENABLED` | `false` | Require auth for `/metrics` |
| `PPROF_ENABLED` | `false` | Enable Go pprof endpoints and profiling UI (admin) |
| `PPROF_AUTH_ENABLED` | `true` | Require admin auth for pprof |
| `PPROF_CPU_MAX_SECONDS` | `120` | Max CPU profile duration |
| `REQUEST_TIMEOUT` | `10s` | Timeout for NATS/monitoring calls |
| `PAGINATION_DEFAULT_LIMIT` | `100` | Default page size for list APIs |
| `PAGINATION_MAX_LIMIT` | `500` | Maximum allowed page size for list APIs |
| `AUDIT_DEFAULT_LIMIT` | `50` | Default page size for audit log when `limit` is omitted |
| `LIVE_WS_MAX_MESSAGES` | `1000` | Max messages per live WebSocket session |
| `LIVE_WS_IDLE_TIMEOUT` | `5m` | Close idle live WebSocket connections after |
| `LIVE_WS_RATE_LIMIT` | `100ms` | Minimum interval between live message frames |
| `MAX_REQUEST_BODY_SIZE` | `1048576` | Maximum API request body size in bytes (1 MiB) |
| `AUTH_RATE_LIMIT` | `10` | Max auth attempts per IP per window |
| `AUTH_RATE_LIMIT_WINDOW` | `1m` | Window for auth rate limiting |
| `AI_ENABLED` | `false` | Enable JetStream AI assistant (Gemini) |
| `AI_API_KEY` | — | Google Gemini API key |
| `AI_MODEL` | `gemini-2.5-flash` | Gemini model name |
| `AI_MAX_TOKENS` | `4096` | Max response tokens |
| `AI_REQUEST_TIMEOUT` | `60s` | LLM request timeout |
| `AI_CONTEXT_CACHE_TTL` | `45s` | How long to cache JetStream context for the assistant |
| `AI_GEMINI_API_BASE` | `https://generativelanguage.googleapis.com/v1beta` | Gemini API base URL |

## JetStream AI Assistant

Built-in assistant scoped **only to NATS JetStream and this console**. Uses **Google Gemini** with your API key — billing via your Google account. Message payloads, credentials, and database data are never sent to the model.

```bash
AI_ENABLED=true
AI_API_KEY=your-gemini-api-key
AI_MODEL=gemini-2.5-flash
```

Open the **AI** floating button in the console (bottom-right) after signing in.

**API:** `POST /api/v1/clusters/{clusterId}/assistant/chat`

## RBAC Roles

| Role | Permissions |
|------|-------------|
| **root** | Single bootstrap superuser (`is_root`); full access; creates delegated admins with access rules |
| **admin** | Full access when unscoped (legacy), or limited via `accessRules` when created by root |
| **operator** | CRUD streams/consumers/KV/objects within assigned clusters; no cluster delete |
| **viewer** | Read-only (dashboard, browse, live tail) within assigned clusters |

The bootstrap account (`ADMIN_USERNAME` / `ADMIN_PASSWORD`) is seeded as the **root** user on first start. Root can create additional admin users with configurable **access rules**. **Operator and viewer users must be assigned at least one cluster** via `accessRules.clusterIds` (multi-tenant scoping).

| Access rule | Meaning |
|-------------|---------|
| `clusterIds` | **Required** for non-root users (except legacy unscoped admin). Limits API access to listed cluster UUIDs. Empty list = no cluster access. |
| `manageUsers` | Create, update, delete users and assign roles (delegated admin only) |
| `viewAudit` | Read the audit log |
| `deleteClusters` | Delete cluster registrations |
| `assignableRoles` | Roles this admin may grant to others |

**Migration (v0.5):** After upgrading, assign `clusterIds` to existing operator/viewer (and scoped admin) accounts that previously had implicit access to all clusters. Users with empty `clusterIds` lose cluster access until clusters are assigned.

Root cannot be deleted or demoted by non-root users. Only one root account may exist.

## Security

NATS Consol applies defense-in-depth for browser and API traffic:

- **HTTP headers** — `Content-Security-Policy`, `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`, and `Strict-Transport-Security` when `PUBLIC_BASE_URL` uses HTTPS.
- **Cookies** — Session and OAuth state cookies are `HttpOnly`, `SameSite=Lax`, and `Secure` in production or behind HTTPS. A separate CSRF cookie pairs with the `X-CSRF-Token` header for cookie-authenticated mutations.
- **CSRF** — State-changing API requests authenticated via session cookie require a matching CSRF token. Basic-auth and OIDC login flows are unchanged; the SPA sends the token automatically.
- **CORS** — Cross-origin access is denied unless the origin is listed in `CORS_ALLOWED_ORIGINS` (no wildcard reflection).
- **Rate limiting** — Login and OIDC callback endpoints are limited per client IP (`AUTH_RATE_LIMIT`, `AUTH_RATE_LIMIT_WINDOW`).
- **Request limits** — Body size capped via `MAX_REQUEST_BODY_SIZE`; server read/write/idle timeouts via `HTTP_*_TIMEOUT`.
- **RBAC & audit** — All `/api/*` routes except health/auth config require authentication when `AUTH_ENABLED=true`. Mutations are audit-logged. Cluster tokens/creds are never returned in API JSON.
- **Production** — Set `ENV=production`, `ENCRYPTION_KEY`, `SESSION_SECRET`, a strong `ADMIN_PASSWORD`, and `AUTH_ENABLED=true`. The server refuses to start if these are missing or weak.

Run `make test-security` for automated checks (headers, cookies, CSRF, rate limits, RBAC, secret leakage).

## SSO (OIDC)

NATS Consol supports multiple SSO providers alongside optional username/password login.

**Built-in providers:** Google, GitHub, GitLab, Microsoft, plus any generic OIDC IdP (Keycloak, Okta, etc.).

**Flow:**

1. User clicks a provider button → `GET /api/v1/auth/oidc/{provider}/login`
2. Browser redirects to the IdP
3. IdP redirects back to `GET /api/v1/auth/oidc/{provider}/callback`
4. Server exchanges the code, creates or links a user, sets an HTTP-only session cookie
5. User lands on the dashboard; the UI loads profile via `GET /api/v1/auth/me`

**Social / enterprise providers** — set `OIDC_PUBLIC_URL` and enable each provider:

```bash
OIDC_PUBLIC_URL=https://nats-consol.example.com

OIDC_GOOGLE_ENABLED=true
OIDC_GOOGLE_CLIENT_ID=...
OIDC_GOOGLE_CLIENT_SECRET=...
# Redirect URI: https://nats-consol.example.com/api/v1/auth/oidc/google/callback

OIDC_GITHUB_ENABLED=true
OIDC_GITHUB_CLIENT_ID=...
OIDC_GITHUB_CLIENT_SECRET=...
# Callback URL: https://nats-consol.example.com/api/v1/auth/oidc/github/callback

OIDC_GITLAB_ENABLED=true
OIDC_GITLAB_CLIENT_ID=...
OIDC_GITLAB_CLIENT_SECRET=...
# Redirect URI: https://nats-consol.example.com/api/v1/auth/oidc/gitlab/callback

OIDC_MICROSOFT_ENABLED=true
OIDC_MICROSOFT_CLIENT_ID=...
OIDC_MICROSOFT_CLIENT_SECRET=...
OIDC_MICROSOFT_TENANT=common
# Redirect URI: https://nats-consol.example.com/api/v1/auth/oidc/microsoft/callback
```

**Generic OIDC (Keycloak, Okta, etc.)** — uses legacy callback path `/api/v1/auth/oidc/callback`:

```bash
AUTH_ENABLED=true
OIDC_ENABLED=true
OIDC_ISSUER=https://your-idp/realms/your-realm          # issuer URL (.well-known/openid-configuration)
OIDC_CLIENT_ID=nats-consol
OIDC_CLIENT_SECRET=your-client-secret                   # omit for public PKCE clients if your IdP allows
OIDC_REDIRECT_URL=https://nats-consol.example.com/api/v1/auth/oidc/callback
SESSION_SECRET=long-random-secret-at-least-16-chars
ENCRYPTION_KEY=another-long-random-secret
ENV=production
```

Set `BASIC_AUTH_ENABLED=false` to hide the password form and allow SSO only.

First-time SSO users are provisioned with the **viewer** role. An admin can promote roles under **Users & Roles**.

### Keycloak

1. Create realm (or use existing)
2. **Clients → Create client**
   - Client type: OpenID Connect
   - Client ID: `nats-consol`
   - Client authentication: On (confidential) or Off (public + PKCE)
   - Valid redirect URI: `https://nats-consol.example.com/api/v1/auth/oidc/callback`
   - Web origins: `https://nats-consol.example.com`
3. Copy **Client secret** from the Credentials tab
4. Set env:

```bash
OIDC_ISSUER=https://keycloak.example.com/realms/myrealm
OIDC_CLIENT_ID=nats-consol
OIDC_CLIENT_SECRET=<client-secret>
OIDC_REDIRECT_URL=https://nats-consol.example.com/api/v1/auth/oidc/callback
```

### Okta

1. **Applications → Create App Integration → OIDC → Web Application**
2. Sign-in redirect URI: `https://nats-consol.example.com/api/v1/auth/oidc/callback`
3. Copy Client ID and Client secret
4. Issuer is shown on the **Sign On** tab (e.g. `https://dev-123456.okta.com/oauth2/default`)

```bash
OIDC_ISSUER=https://dev-123456.okta.com/oauth2/default
OIDC_CLIENT_ID=<client-id>
OIDC_CLIENT_SECRET=<client-secret>
OIDC_REDIRECT_URL=https://nats-consol.example.com/api/v1/auth/oidc/callback
```

### Azure AD (Entra ID)

1. **App registrations → New registration**
   - Redirect URI (Web): `https://nats-consol.example.com/api/v1/auth/oidc/callback`
2. Create a **client secret** under Certificates & secrets
3. Issuer:

```bash
OIDC_ISSUER=https://login.microsoftonline.com/<tenant-id>/v2.0
OIDC_CLIENT_ID=<application-client-id>
OIDC_CLIENT_SECRET=<client-secret>
OIDC_REDIRECT_URL=https://nats-consol.example.com/api/v1/auth/oidc/callback
```

### Local development with SSO

Run the backend on `:8080` (or proxy the Vite dev server). The redirect URL must match exactly what the IdP expects — for local testing:

```bash
OIDC_REDIRECT_URL=http://localhost:8080/api/v1/auth/oidc/callback
```

When using `npm run dev` on `:5173`, OIDC login still goes through the backend (`/api/v1/auth/oidc/login`); ensure the IdP allows that callback URL on port 8080.

## API

See [`api/openapi.yaml`](api/openapi.yaml) or live spec at `GET /api/openapi.yaml`.

Key endpoints:

- `GET /api/health` — readiness (postgres + default NATS cluster)
- `GET /metrics` — Prometheus metrics
- `GET /api/v1/auth/config` — `{ oidc_enabled, basic_enabled, auth_enabled }`
- `POST /api/v1/auth/login` — session login (when basic auth enabled)
- `GET /api/v1/auth/me` — current user profile
- `POST /api/v1/auth/logout` — clear session cookie
- `GET /api/v1/auth/oidc/login` — OIDC redirect
- `GET /api/v1/audit` — audit log (admin)
- `GET /api/v1/users` — user list (admin)
- `GET/POST /api/v1/clusters`
- Cluster-scoped JetStream, KV, Object Store, and live WebSocket paths under `/api/v1/clusters/{id}/…`

## Helm

```bash
helm upgrade --install nats-consol ./deploy/helm/nats-consol \
  --set secrets.databaseUrl='postgres://…' \
  --set secrets.encryptionKey='your-32-char-key'
```

## Testing

Tests are grouped by intent, not by framework. One shared `tests/testutil` package backs integration, contract, security, and database suites (testcontainers + in-memory HTTP).

| Category | Command | Docker / stack required |
|----------|---------|-------------------------|
| **Unit** | `make test-unit` | No |
| **Integration** (API + NATS + DB) | `make test-integration` | Yes (testcontainers) |
| **Database** | included in `make test-integration` | Yes |
| **Contract** (camelCase JSON vs frontend) | `make test-contract` | Yes |
| **Security** (auth, RBAC, headers, CSRF, rate limits, no secrets in responses) | `make test-security` | Yes |
| **Regression** (CI gate) | `make test-regression` | Yes |
| **Smoke / E2E / Acceptance** | `make test-smoke` | Yes (`docker compose up`) |
| **Load / Throughput / Performance** | `make test-performance` | Yes + [vegeta](https://github.com/tsenart/vegeta) |

Quick start:

```bash
make test-unit                              # fast, no Docker
make test-regression                        # integration + contract + security
docker compose up --build -d && make test-smoke
docker compose up -d && make test-performance   # needs vegeta installed
```

Set `SKIP_TESTCONTAINERS=1` to skip Docker-backed Go tests.

Environment variables for smoke/performance scripts:

| Variable | Default | Description |
|----------|---------|-------------|
| `BASE_URL` | `http://localhost:8080` | Running console URL |
| `AUTH` | `admin:admin` | Basic auth for smoke/perf |
| `PERF_MIN_RPS` | (derived) | Performance script checks success rate ≥ 99% |
| `PERF_MAX_P99_MS` | `2000` | Max p99 latency (ms) for health/config/clusters |

CI (`.github/workflows/test.yml`) runs unit + regression on every PR; performance runs on pushes to `main`.

## Roadmap (v0.4+)

- Account/JWT NATS resolver management
- Message publish API + UI
- Credential key rotation endpoint
- OpenAPI-generated CLI
- Historical metrics (long-term TSDB)

## License

Apache 2.0 — see [LICENSE](LICENSE).
