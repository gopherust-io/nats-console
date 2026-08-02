# DevOps setup guide

Deploy NATS Consol safely for your team. This guide is for **platform engineers**, **SREs**, and anyone responsible for production infrastructure.

---

## What you're deploying

One **stateless** console pod/process plus:

| Dependency | Required? | Notes |
|------------|-----------|-------|
| **PostgreSQL 16+** | Yes | Cluster registry, users, audit log |
| **NATS JetStream** | Yes | At least one cluster to manage |
| **NATS monitoring HTTP** | Recommended | Dashboard, varz/jsz |
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
- [ ] RSA session key pair (`SESSION_PRIVATE_KEY` / `SESSION_PUBLIC_KEY`, ≥2048-bit) — signs/verifies access JWTs  
- [ ] `TRUSTED_PROXIES` set correctly if behind a reverse proxy (refresh fingerprint uses client IP)  
- [ ] `ADMIN_PASSWORD` changed from default  
- [ ] HTTPS in front (ingress / load balancer)  
- [ ] `PUBLIC_BASE_URL` matches your public hostname  
- [ ] PostgreSQL backups enabled  
- [ ] Network: console → NATS `:4222` and monitoring `:8222` only from private networks  
- [ ] Configure OTLP collector (`TEL_COLLECTOR_GRPC_ADDR`) when process metrics are needed
- [ ] Keep `PPROF_ENABLED=false` unless admins need runtime profiling
- [ ] Consider `LOG_LEVEL=warn` and `METRICS_SNAPSHOT_INTERVAL=120s` under load

The server **refuses to start** if encryption key, session RSA keys, weak admin password (in production), insecure Postgres (`sslmode=disable`), plaintext NATS (`nats://`), or HTTP monitoring is configured.

Production connection checklist:

- [ ] `DATABASE_URL` with `sslmode=require` (minimum) or `verify-full` (preferred)
- [ ] `NATS_URL` with `tls://` (or `wss://`)
- [ ] `NATS_CREDS_FILE` or `NATS_TOKEN`
- [ ] `NATS_MONITORING_URL` with `https://`
- [ ] `NATS_TLS_CA_FILE` (and optional client cert/key for mTLS)
- [ ] `NATS_TLS_INSECURE_SKIP_VERIFY=false`

---

## Docker Compose (single host)

Copy `.env.example` → `.env` and set **all required secrets** before `docker compose up`. Compose no longer embeds passwords.

**Local lab** (default `.env.example` block) uses plaintext Postgres/NATS on the compose network — suitable only for development. For production-ish compose:

```yaml
# excerpt — secrets come from .env / a secrets manager
console:
  environment:
    ENV: production
    DATABASE_URL: postgres://user:pass@postgres:5432/natsconsol?sslmode=require
    ENCRYPTION_KEY: ${ENCRYPTION_KEY}
    SESSION_PRIVATE_KEY: ${SESSION_PRIVATE_KEY}
    SESSION_PUBLIC_KEY: ${SESSION_PUBLIC_KEY}
    ADMIN_USERNAME: admin
    ADMIN_PASSWORD: ${ADMIN_PASSWORD}
    PUBLIC_BASE_URL: https://nats-consol.example.com
    STATIC_DIR: /app/web
    NATS_URL: tls://nats:4222
    NATS_MONITORING_URL: https://nats:8222
    NATS_TOKEN: ${NATS_TOKEN}
    NATS_TLS_CA_FILE: /run/secrets/nats-ca.pem
    LOG_JSON: "true"
```

Put TLS termination on a reverse proxy (nginx, Caddy, Traefik) or Kubernetes Ingress in front of `:8080`. The console itself serves plain HTTP; TLS and HTTP/2 are a DevOps concern outside the app.

Local Compose exposes the console at **http://localhost:8080** (`PUBLIC_BASE_URL` defaults to the same). Set `config.publicBaseUrl` (Helm) or `PUBLIC_BASE_URL` to your public HTTPS URL when a proxy terminates TLS.

Enable Ingress in the Helm chart when you want cluster-managed TLS:

```bash
helm upgrade --install nats-consol ./deploy/helm/nats-consol \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=nats-consol.example.com \
  --set config.publicBaseUrl=https://nats-consol.example.com
```

---

## Kubernetes (Helm)

```bash
helm upgrade --install nats-consol ./deploy/helm/nats-consol \
  --namespace nats-consol --create-namespace \
  --set secrets.databaseUrl='postgres://user:pass@postgres:5432/natsconsol?sslmode=verify-full' \
  --set secrets.encryptionKey='your-long-random-encryption-key' \
  --set secrets.sessionPrivateKey="$(cat session.pem)" \
  --set secrets.sessionPublicKey="$(cat session.pub.pem)" \
  --set secrets.adminPassword='your-strong-admin-password' \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=nats-consol.example.com \
  --set config.natsUrl=tls://nats.nats.svc:4222 \
  --set config.monitoringUrl=https://nats.nats.svc:8222 \
  --set config.natsTlsCaFile=/etc/nats-tls/ca.pem \
  --set config.natsToken='your-nats-auth-token'
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
| `sessionPrivateKey` | RSA private PEM for RS256 session JWTs |
| `sessionPublicKey` | Matching RSA public PEM |
| `adminPassword` | Bootstrap root password (first install only) |

---

## Connecting to NATS clusters

Cluster registrations are **devops-managed**. On first start with an empty registry, the console seeds a default cluster from env (`NATS_URL`, monitoring URL, optional token/creds). Additional clusters are added via Postgres / ops tooling—not the console API or UI.

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
- If NATS uses TLS or NKeys, provide matching creds when registering the cluster (env bootstrap or DB)

### Company-wide (many teams / many systems)

Prefer **one** production consol that registers every NATS deployment as a system, then scope teams with `clusterIds` and Access grants. Map microservices to streams/subjects (and NATS accounts), not to separate consol installs.

Full playbook: [Company-wide scaling](./company-scale.md).

---

## Authentication

### Username & password

```bash
ADMIN_USERNAME=admin
ADMIN_PASSWORD=<strong>
```

Bootstrap user is **root** on first start. Authentication is always enabled.

### Invite links

Admins create pending users under **Admin → People** and share one-time `/invite/<token>` URLs. Invited users set a password and sign in. Access remains invite-only — there is no public sign-up.

---

## Observability

| Endpoint | Auth | Use |
|----------|------|-----|
| `GET /api/health` | Public | Liveness/readiness |
| OTLP (tel) | Collector | Process metrics/traces (`nats_consol_*` instruments) |
| `GET /api/v1/clusters/{id}/metrics/history` | Authenticated | JetStream/server history for the UI |

Process metrics (HTTP latency, NATS dials/reconnects, WebSocket counts) export via [tel](https://github.com/gopherust-io/tel) OTLP, not a Prometheus scrape endpoint.

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
PPROF_CPU_MAX_SECONDS=120
HTTP_WRITE_TIMEOUT=125s   # must be >= PPROF_CPU_MAX_SECONDS + buffer
```

Exposes on-demand `/api/v1/pprof/*` for admins. Raw `/debug/pprof` returns **404 in production**. **Off by default** — enable only when debugging console performance.

### Alert email (optional)

When enabled, each **newly opened** alert is emailed to console users with a real address (not `*@local`) who can access that cluster. Requires `METRICS_SNAPSHOT_ENABLED=true`. Off by default. Set each user’s email under **Console Users**.

#### Local: Mailpit (Docker Compose)

Catch mail in a web UI without sending to a real inbox:

```bash
docker compose --profile mail up --build
```

In `.env` (console container uses `SMTP_HOST=mailpit` on the Compose network):

```bash
SMTP_ENABLED=true
SMTP_HOST=mailpit
SMTP_PORT=1025
SMTP_TLS=false
SMTP_FROM=nats-consol@local.test
# SMTP_USERNAME / SMTP_PASSWORD can be empty
```

Open **http://localhost:8025** to read captured messages. Default `docker compose up` does **not** start Mailpit.

#### Production / real inbox: Gmail SMTP

1. Enable 2-Step Verification on the Google account.  
2. Create an **App password** (Google Account → Security → App passwords).  
3. Set `.env` (never commit the app password):

```bash
SMTP_ENABLED=true
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_TLS=true
SMTP_USERNAME=you@gmail.com
SMTP_PASSWORD=your-16-char-app-password
SMTP_FROM=you@gmail.com
PUBLIC_BASE_URL=https://consol.example.com   # used in email links
```

Other providers (SendGrid, SES, Mailgun, corporate relay) work the same way: set `SMTP_HOST` / port / credentials / `SMTP_FROM`.

---

## Security features (built-in)

- CSP, HSTS (when HTTPS), frame denial, nosniff  
- HttpOnly session cookies + CSRF on cookie-authenticated mutations  
- Per-IP rate limit on login  
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
| `DATABASE_URL` | **Required** — `sslmode=require`, `verify-ca`, or `verify-full` |
| `ENCRYPTION_KEY` | **Required** — rotate with care (re-encrypt clusters) |
| `SESSION_PRIVATE_KEY` | **Required** — RSA private PEM (≥2048-bit); rotating logs everyone out |
| `SESSION_PUBLIC_KEY` | **Required** — matching RSA public PEM |
| `SESSION_TTL` | Access JWT lifetime (default `15m`) |
| `REFRESH_TOKEN_TTL` | Refresh cookie lifetime (default `168h`); bound to UA+IP fingerprint — set `TRUSTED_PROXIES` behind proxies |
| `NATS_URL` | Use `tls://` / `wss://` when seeding a default cluster |
| `NATS_CREDS_FILE` / `NATS_TOKEN` | Required when `NATS_URL` is set |
| `NATS_MONITORING_URL` | Use `https://` |
| `NATS_TLS_CA_FILE` | CA PEM for NATS server verification |
| `NATS_TLS_CERT_FILE` / `NATS_TLS_KEY_FILE` | Optional mTLS client cert |
| `NATS_TLS_INSECURE_SKIP_VERIFY` | Must stay `false` |
| `CORS_ALLOWED_ORIGINS` | Set if UI is on a different origin |
| `HTTP_*_TIMEOUT` | Increase if NATS clusters are high-latency |
| `NATS_CLIENT_CACHE_TTL` | How long idle NATS connections stay pooled |
| `MAX_REQUEST_BODY_SIZE` | Default 1 MiB |
| `HTTP_RESPONSE_COMPRESSION` | Default `true`; disable if an edge proxy already compresses |
| `AUTH_RATE_LIMIT` | Brute-force protection on login |

Full list: [README configuration table](../README.md#configuration).

---

## Encryption key rotation

Root-only API to re-encrypt cluster tokens when rotating `ENCRYPTION_KEY`:

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
| `METRICS_SNAPSHOT_ENABLED` | `true` | Enable background collector (also drives alert rule evaluation) |
| `METRICS_SNAPSHOT_INTERVAL` | `60s` | Scrape interval per cluster (use `120s` in production to halve monitoring + DB load) |
| `METRICS_SNAPSHOT_RETENTION` | `168h` | Sample TTL (7 days) |
| `METRICS_SNAPSHOT_BOTTLENECK_RETENTION` | `672h` | Hourly bottleneck rollup TTL (28 days) |
| `METRICS_SNAPSHOT_CLEANUP_INTERVAL` | `1h` | Purge job frequency |
| `SLOW_CONSUMER_PENDING_THRESHOLD` | `1000` | Pending msgs ≥ this → count as slow |
| `SLOW_CONSUMER_LAG_THRESHOLD` | `1000` | Stream lag ≥ this → count as slow |
| `SLOW_CONSUMER_ACK_PENDING_RATIO` | `0.9` | Ack-pending ≥ ratio × MaxAckPending → count as slow |
| `BEHAVIOR_FINGERPRINT_KV_BUCKET` | `nats_consol_fingerprints` | KV bucket for worker-reported behavior fingerprints |
| `SMTP_ENABLED` | `false` | Email console users when an alert first opens |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_FROM` | — / `587` / — | Required when SMTP is enabled |

Rough sizing: ~18 metric keys × 1 sample/min/cluster ≈ 26k rows/cluster/day. With 7-day retention, plan ~180k rows per cluster unless you shorten retention.

Alert on aggregates such as `jetstream.slow_consumers` or `jetstream.consumer_max_lag` (see [user-guide.md](user-guide.md#slow-consumers)).

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
| CSRF errors | Session cookie blocked — check SameSite / HTTPS |

---

## Support runbook snippet

```bash
# Health
curl -s https://nats-consol.example.com/api/health | jq

# Logs — look for component=http request lines with status 5xx
```

For application teams, point them to the [User guide](./user-guide.md).
