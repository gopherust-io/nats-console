# NATS Consol

Open-source, self-hosted admin console for **NATS JetStream** — [github.com/gopherust-io/nats-consol](https://github.com/gopherust-io/nats-consol).

Manage streams, consumers, browse messages, tail live traffic, manage KV/Object stores, and monitor multi-cluster deployments from a modern web UI — without exposing NATS monitoring ports to the public internet.

## Features (v0.3)

- **Multi-cluster registry** with PostgreSQL persistence
- **Enterprise security** — AES-GCM credential encryption, audit log, OIDC SSO, RBAC
- Dashboard with JetStream account usage, server info, and jsz metrics
- Stream list, create, update, delete, purge
- Consumer CRUD with detail pages
- Message browser with prev/next navigation and JSON/raw view
- **Live mode** — real-time WebSocket tail per stream
- **KV Store** and **Object Store** management
- **Prometheus metrics** at `/metrics`
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

- **Username:** `admin`
- **Password:** `admin`

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

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_ADDR` | `:8080` | Console HTTP listen address |
| `ENV` | `development` | Set to `production` to enforce encryption key |
| `DATABASE_URL` | `postgres://…` | PostgreSQL connection string |
| `ENCRYPTION_KEY` | — | AES-GCM key for cluster tokens (**required in production**) |
| `SESSION_SECRET` | falls back to `ENCRYPTION_KEY` | JWT session signing secret |
| `SESSION_TTL` | `8h` | Session cookie lifetime |
| `DEFAULT_CLUSTER_NAME` | `default` | Name for env-seeded default cluster |
| `NATS_URL` | `nats://localhost:4222` | NATS client URL (default cluster seed) |
| `NATS_MONITORING_URL` | `http://localhost:8222` | NATS monitoring HTTP URL |
| `NATS_CREDS_FILE` | — | Optional creds file path |
| `NATS_TOKEN` | — | Optional NATS token |
| `STATIC_DIR` | — | Path to built frontend (`web/dist`) |
| `AUTH_ENABLED` | `true` | Enable authentication |
| `ADMIN_USERNAME` | `admin` | Bootstrap admin username |
| `ADMIN_PASSWORD` | `admin` | Bootstrap admin password |
| `OIDC_ENABLED` | `false` | Enable OIDC SSO |
| `OIDC_ISSUER` | — | OIDC provider issuer URL |
| `OIDC_CLIENT_ID` | — | OIDC client ID |
| `OIDC_CLIENT_SECRET` | — | OIDC client secret |
| `OIDC_REDIRECT_URL` | — | OIDC callback URL (must match IdP client config) |
| `BASIC_AUTH_ENABLED` | `true` | Allow username/password login; set `false` for SSO-only |
| `CORS_ALLOWED_ORIGINS` | — | Comma-separated allowed origins |
| `LOG_JSON` | `false` | Emit structured JSON logs |
| `METRICS_AUTH_ENABLED` | `false` | Require auth for `/metrics` |
| `REQUEST_TIMEOUT` | `10s` | Timeout for NATS/monitoring calls |

## RBAC Roles

| Role | Permissions |
|------|-------------|
| **admin** | Full access, audit log, user management |
| **operator** | CRUD streams/consumers/KV/objects; no cluster delete |
| **viewer** | Read-only (dashboard, browse, live tail) |

## SSO (OIDC)

NATS Consol supports OpenID Connect SSO alongside optional username/password login.

**Flow:**

1. User clicks **Sign in with SSO** → `GET /api/v1/auth/oidc/login`
2. Browser redirects to your IdP (Keycloak, Okta, Azure AD, etc.)
3. IdP redirects back to `GET /api/v1/auth/oidc/callback`
4. Server exchanges the code, creates or links a user, sets an HTTP-only session cookie
5. User lands on the dashboard; the UI loads profile via `GET /api/v1/auth/me`

**Required env vars:**

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

```bash
go test ./...
make test-integration   # requires Docker (testcontainers)
```

Set `SKIP_TESTCONTAINERS=1` to skip integration tests that need Docker.

## Roadmap (v0.4+)

- Account/JWT NATS resolver management
- Message publish API + UI
- Credential key rotation endpoint
- OpenAPI-generated CLI
- Historical metrics (long-term TSDB)

## License

Apache 2.0 — see [LICENSE](LICENSE).
