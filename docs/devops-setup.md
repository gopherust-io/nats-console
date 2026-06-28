# DevOps setup guide

Deploy NATS Consol safely for your team. This guide is for **platform engineers**, **SREs**, and anyone responsible for production infrastructure.

---

## What you're deploying

One **stateless** console pod/process plus:

| Dependency | Required? | Notes |
|------------|-----------|-------|
| **PostgreSQL 16+** | Yes | Cluster registry, users, audit log |
| **NATS JetStream** | Yes | At least one cluster to manage |
| **NATS monitoring HTTP** | Recommended | Dashboard, supercluster, varz/jsz |
| **OIDC IdP** | Optional | SSO instead of passwords |
| **Gemini API key** | Optional | AI assistant only |

The console **never** replaces NATS — it sits beside it as a control plane UI.

---

## Deployment options

| Method | Best for |
|--------|----------|
| [Docker Compose](../docker-compose.yml) | Demos, small teams, single host |
| [Helm chart](../deploy/helm/nats-consol/) | Kubernetes production |
| **Binary + systemd** | VM/bare metal (`make build`, set `STATIC_DIR`) |

---

## Production checklist

Before pointing real users at the console:

- [ ] `ENV=production`  
- [ ] Strong random `ENCRYPTION_KEY` (32+ chars) — encrypts stored NATS tokens/creds  
- [ ] Strong random `SESSION_SECRET` — signs session cookies  
- [ ] `ADMIN_PASSWORD` changed from default  
- [ ] `AUTH_ENABLED=true`  
- [ ] HTTPS in front (ingress / load balancer)  
- [ ] `PUBLIC_BASE_URL` or `OIDC_*` URLs match your public hostname  
- [ ] PostgreSQL backups enabled  
- [ ] Network: console → NATS `:4222` and monitoring `:8222` only from private networks  
- [ ] Set `METRICS_AUTH_ENABLED=true` in production (required when `ENV=production`)
- [ ] Keep `PPROF_ENABLED=false` unless admins need runtime profiling
- [ ] Set `PPROF_CONTINUOUS=false` in production (continuous CPU profiling can use 20–35% CPU)
- [ ] Consider `LOG_LEVEL=warn` and `METRICS_SNAPSHOT_INTERVAL=120s` under load

The server **refuses to start** in production if encryption key, session secret, or weak admin password is missing.

---

## Docker Compose (single host)

Minimal production-ish compose pattern:

```yaml
# excerpt — see full docker-compose.yml for reference
console:
  environment:
    ENV: production
    DATABASE_URL: postgres://user:pass@postgres:5432/natsconsol?sslmode=require
    ENCRYPTION_KEY: ${ENCRYPTION_KEY}      # from secrets manager
    SESSION_SECRET: ${SESSION_SECRET}
    ADMIN_USERNAME: admin
    ADMIN_PASSWORD: ${ADMIN_PASSWORD}
    AUTH_ENABLED: "true"
    PUBLIC_BASE_URL: https://nats-consol.example.com
    STATIC_DIR: /app/web
    NATS_URL: nats://nats:4222
    NATS_MONITORING_URL: http://nats:8222
    LOG_JSON: "true"
```

Put TLS termination on a reverse proxy (nginx, Caddy, Traefik) in front of `:8080`. For **HTTP/3 (QUIC)**, prefer Caddy and open **UDP 443** to clients.

### HTTP/3 with Docker Compose

```bash
docker compose --profile http3 up --build
```

Open **https://localhost** (Caddy terminates TLS + HTTP/3; console stays on internal `:8080`).

Set on the console service when using the profile:

- `PUBLIC_BASE_URL=https://localhost`
- `OIDC_PUBLIC_URL=https://localhost`
- `OIDC_REDIRECT_URL=https://localhost/api/v1/auth/oidc/callback`

Caddy config: [`deploy/caddy/Caddyfile.dev`](deploy/caddy/Caddyfile.dev). Live WebSocket tail uses HTTP/1.1 upgrade through the proxy (Caddy handles `Upgrade` automatically).

### HTTP/3 with Helm

Enable the optional Caddy gateway (LoadBalancer with TCP+UDP 443):

```bash
helm upgrade --install nats-consol ./deploy/helm/nats-consol \
  --set http3.enabled=true \
  --set http3.host=nats-consol.example.com \
  --set http3.tlsSecretName=nats-consol-tls
```

Or use a **Caddy Ingress Controller** and set `ingress.annotations` / `http3.enabled` for `h1,h2,h3` protocols.

**Not migrated to HTTP/3:** NATS JetStream (`4222`), NATS monitoring (`8222`), Postgres — these remain on their native protocols.

---

## Kubernetes (Helm)

```bash
helm upgrade --install nats-consol ./deploy/helm/nats-consol \
  --namespace nats-consol --create-namespace \
  --set secrets.databaseUrl='postgres://user:pass@postgres:5432/natsconsol?sslmode=require' \
  --set secrets.encryptionKey='your-long-random-encryption-key' \
  --set secrets.sessionSecret='your-long-random-session-secret' \
  --set secrets.adminPassword='your-strong-admin-password' \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=nats-consol.example.com \
  --set config.natsUrl=nats://nats.nats.svc:4222 \
  --set config.monitoringUrl=http://nats.nats.svc:8222
```

### Probes

Helm defaults use `GET /api/health`:

- **200** — Postgres OK + default NATS cluster reachable  
- **503** — dependency down (pod not ready)

Tune `probes.liveness` / `probes.readiness` in `values.yaml` for your cluster.

### Secrets

Store in Kubernetes Secrets or external secret operator:

| Key | Purpose |
|-----|---------|
| `databaseUrl` | Postgres DSN |
| `encryptionKey` | AES-GCM for cluster credentials |
| `sessionSecret` | JWT session signing |
| `adminPassword` | Bootstrap root password (first install only) |
| `oidcClientSecret` | OIDC confidential clients |

---

## Connecting to NATS clusters

Each registered cluster needs:

```text
NATS URL          → nats://host:4222  (or tls://)
Monitoring URL    → http://host:8222   (NATS --http_port)
Token / creds     → optional, encrypted in Postgres
```

### Network policy (recommended)

```text
Internet ──► Ingress/TLS ──► Console :8080
                              │
                              ├──► PostgreSQL :5432
                              ├──► NATS :4222 (each cluster)
                              └──► NATS monitoring :8222
```

Users' browsers **only** reach the console. They never touch NATS ports.

### NATS server requirements

- JetStream enabled (`--jetstream` or `jetstream {}` in config)  
- Monitoring port enabled (`--http_port=8222` or `http_port: 8222`)  
- If NATS uses TLS or NKeys, provide matching creds when registering the cluster in the UI  

---

## Authentication & SSO

### Basic auth only

```bash
AUTH_ENABLED=true
BASIC_AUTH_ENABLED=true
ADMIN_USERNAME=admin
ADMIN_PASSWORD=<strong>
```

Bootstrap user is **root** on first start.

### OIDC / SSO

Enable generic OIDC:

```bash
OIDC_ENABLED=true
OIDC_ISSUER=https://your-idp/realms/your-realm
OIDC_CLIENT_ID=nats-consol
OIDC_CLIENT_SECRET=<secret>
OIDC_REDIRECT_URL=https://nats-consol.example.com/api/v1/auth/oidc/callback
OIDC_PUBLIC_URL=https://nats-consol.example.com
```

Social providers (Google, GitHub, GitLab, Microsoft) use per-provider callbacks:

```text
https://nats-consol.example.com/api/v1/auth/oidc/{provider}/callback
```

Set `BASIC_AUTH_ENABLED=false` for SSO-only login.

See the main [README SSO section](../README.md#sso-oidc) for Keycloak, Okta, and Azure AD examples.

---

## Observability

| Endpoint | Auth | Use |
|----------|------|-----|
| `GET /api/health` | Public | Liveness/readiness |
| `GET /metrics` | Admin/root* | Prometheus metrics |

\* `METRICS_AUTH_ENABLED=true` is required when `ENV=production`. Scrapers must authenticate as admin/root.

Metrics include HTTP latency, active NATS connections, reconnects, and WebSocket counts.

### Logging

```bash
LOG_JSON=true
LOG_LEVEL=info   # debug for troubleshooting
```

Structured JSON logs include request ID, path, status, duration.

### Profiling (optional)

```bash
PPROF_ENABLED=true
PPROF_AUTH_ENABLED=true   # required in production
PPROF_CONTINUOUS=false    # keep false in production unless actively debugging
PPROF_CPU_MAX_SECONDS=120
HTTP_WRITE_TIMEOUT=125s   # must be >= PPROF_CPU_MAX_SECONDS + buffer
```

Exposes `/api/v1/pprof/*` for admins. Raw `/debug/pprof` returns **404 in production**. **Off by default** — enable only when debugging console performance.

---

## Security features (built-in)

- CSP, HSTS (when HTTPS), frame denial, nosniff  
- HttpOnly session cookies + CSRF on cookie-authenticated mutations  
- Per-IP rate limit on login/OIDC callbacks  
- RBAC on all `/api/*` routes (except health/auth config)  
- Audit log for mutations  
- Cluster secrets never returned in API responses  

Run automated checks:

```bash
make test-security
```

---

## Environment variables (operations focus)

| Variable | Production note |
|----------|-----------------|
| `ENV` | Must be `production` |
| `DATABASE_URL` | Use `sslmode=require` when Postgres supports TLS |
| `ENCRYPTION_KEY` | **Required** — rotate with care (re-encrypt clusters) |
| `SESSION_SECRET` | **Required** — rotating logs everyone out |
| `CORS_ALLOWED_ORIGINS` | Set if UI is on a different origin |
| `HTTP_*_TIMEOUT` | Increase if NATS clusters are high-latency |
| `NATS_CLIENT_CACHE_TTL` | How long idle NATS connections stay pooled |
| `MAX_REQUEST_BODY_SIZE` | Default 1 MiB |
| `AUTH_RATE_LIMIT` | Brute-force protection on login |

Full list: [README configuration table](../README.md#configuration).

---

## Encryption key rotation

Root-only API to re-encrypt cluster tokens and stored JWTs when rotating `ENCRYPTION_KEY`:

```bash
# Dry-run (count rows, no writes)
curl -X POST -b cookies.txt -H "Content-Type: application/json" \
  "https://consol.example/api/v1/admin/rotate-encryption-key?dryRun=true" \
  -d '{"currentKey":"old-key-at-least-16-ch","newKey":"new-key-at-least-16-ch"}'

# Apply rotation
curl -X POST -b cookies.txt -H "Content-Type: application/json" \
  "https://consol.example/api/v1/admin/rotate-encryption-key" \
  -d '{"currentKey":"old-key-at-least-16-ch","newKey":"new-key-at-least-16-ch"}'
```

After a successful rotation:

1. Update `ENCRYPTION_KEY` in your deployment (Helm values, env file, etc.)
2. Restart all Consol instances so the new key is loaded
3. Verify login and cluster connectivity

Rotation fails closed if any stored secret cannot be decrypted with `currentKey`.

---

## Historical metrics snapshots

Background collector stores normalized JetStream/varz samples in PostgreSQL for Dashboard trends.

| Env | Default | Purpose |
|-----|---------|---------|
| `METRICS_SNAPSHOT_ENABLED` | `true` | Enable background collector |
| `METRICS_SNAPSHOT_INTERVAL` | `60s` | Scrape interval per cluster (use `120s` in production to halve monitoring + DB load) |
| `METRICS_SNAPSHOT_RETENTION` | `168h` | Sample TTL (7 days) |
| `METRICS_SNAPSHOT_CLEANUP_INTERVAL` | `1h` | Purge job frequency |

Rough sizing: ~14 metric keys × 1 sample/min/cluster ≈ 20k rows/cluster/day. With 7-day retention, plan ~140k rows per cluster unless you shorten retention.

Query history via `GET /api/v1/clusters/{id}/metrics/history?from=&to=&step=`.

---

## Upgrades

1. Backup PostgreSQL  
2. Deploy new image/binary  
3. Migrations run automatically on startup  
4. Verify `GET /api/health`  
5. Smoke test: login → list streams on default cluster  

```bash
make test-smoke   # against running stack
```

---

## Troubleshooting

| Symptom | Likely cause |
|---------|--------------|
| Health 503 | Postgres down or default NATS unreachable |
| Login 429 | Rate limited — wait or adjust `AUTH_RATE_LIMIT` |
| Cluster test fails | Network, wrong URL, or NATS auth |
| SSO redirect mismatch | `OIDC_REDIRECT_URL` ≠ IdP client config |
| Empty supercluster | Single-node NATS — expected |
| CSRF errors | Session cookie blocked — check SameSite / HTTPS |

---

## Support runbook snippet

```bash
# Health
curl -s https://nats-consol.example.com/api/health | jq

# Metrics (if unauthenticated)
curl -s https://nats-consol.example.com/metrics | head

# Logs — look for component=http request lines with status 5xx
```

For application teams, point them to the [User guide](./user-guide.md).
