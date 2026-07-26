# Developer setup guide

Work on the NATS Consol codebase locally — backend (Go), frontend (React), and tests.

---

## Prerequisites

| Tool | Version |
|------|---------|
| Go | 1.26+ |
| Node.js | 22+ |
| Docker | For Postgres, NATS, testcontainers |
| golangci-lint | v2+ (`make lint-go`) |
| envgen (optional) | Only after editing `internal/config/config.go` |

---

## Repository layout

```text
nats-consol/
├── cmd/server/          # Main entrypoint
├── internal/
│   ├── api/             # HTTP routes, handlers, middleware
│   ├── app/             # Application services
│   ├── domain/          # DTOs & business types
│   ├── nats/            # NATS client & connections
│   ├── store/           # Postgres access
│   └── auth/            # Sessions, RBAC
├── web/                 # React + Vite frontend
├── migrations/          # SQL migrations
├── tests/               # integration, contract, security, e2e
├── docs/                # You are here
└── deploy/helm/         # Kubernetes chart
```

---

## Fastest local loop

### 1. Dependencies

```bash
cp .env.example .env
# Edit .env — set POSTGRES_PASSWORD, ADMIN_PASSWORD, ENCRYPTION_KEY, SESSION_SECRET
docker compose up postgres nats -d
```

### 2. Backend

```bash
# .env already loaded by the server; for a host-run process use localhost DSN:
export DATABASE_URL=postgres://natsconsol:${POSTGRES_PASSWORD}@localhost:5432/natsconsol?sslmode=disable
export NATS_URL=nats://localhost:4222
export NATS_MONITORING_URL=http://localhost:8222
export ADMIN_PASSWORD=change-me-local-admin
export ENCRYPTION_KEY=dev-encryption-key-min-16-chars
export SESSION_SECRET=dev-session-secret-min-16-chars
# Optional: AUTH_ENABLED=false for a faster local loop (not for production)

go run ./cmd/server
```

API listens on **http://localhost:8080**.

With `AUTH_ENABLED=false`, the UI treats you as a dev admin automatically.

### 3. Frontend (hot reload)

```bash
cd web
npm install
npm run dev
```

Open **http://localhost:5173**.

Vite proxies:

- `/api/*` → `:8080`  
- `/debug/*` → `:8080` (pprof, when enabled)

---

## Full stack with Docker (matches production closer)

```bash
cp .env.example .env   # required — fill secrets
docker compose up --build
```

Includes Postgres + NATS + console. UI at http://localhost:8080. Login with the `ADMIN_USERNAME` / `ADMIN_PASSWORD` from your `.env`.

> The stock compose stack is a **local plaintext lab**. Production must use `ENV=production`, Postgres `sslmode=require|verify-full`, `tls://` NATS, and HTTPS monitoring (see [devops-setup.md](devops-setup.md)).

### Test alert emails locally

```bash
docker compose --profile mail up --build
```

Uncomment the Mailpit SMTP block in `.env` (see `.env.example`). Captured mail UI: http://localhost:8025. For Gmail App Password setup, see [devops-setup.md](devops-setup.md#alert-email-optional).

---

## Building a release binary

```bash
make build
```

Produces `bin/nats-consol` and `web/dist/`. Run with:

```bash
STATIC_DIR=web/dist \
DATABASE_URL=postgres://... \
NATS_URL=nats://localhost:4222 \
./bin/nats-consol
```

---

## Configuration codegen

`internal/config/config.go` uses struct tags + `envgen`:

```bash
go install github.com/gopherust-io/env/cmd/envgen@latest
go generate ./internal/config/...
```

Commit both `config.go` and `config_env_gen.go` when adding env vars.

---

## API conventions

- REST under `/api/v1/…`  
- **camelCase** JSON on all frontend-facing responses  
- Typed DTOs in `internal/domain/` and `internal/api/responses.go` — avoid `map[string]any` for API output  
- NATS monitoring passthrough uses server-native snake_case; console API DTOs use camelCase  

OpenAPI: [`api/openapi.yaml`](../api/openapi.yaml)

---

## Frontend conventions

- React 19 + React Router + TanStack Query  
- API client: `web/src/lib/api.ts` (handles Basic auth, CSRF, credentials)  
- New page checklist:
  1. `web/src/pages/FooPage.tsx`
  2. Lazy route in `web/src/App.tsx`
  3. Nav link in `web/src/components/Layout.tsx`
  4. Icon in `web/src/components/ui/NavIcon.tsx`

---

## Testing

```bash
make test-unit          # fast, no Docker
make test-regression    # integration + contract + security (Docker)
make test-web           # vitest + Playwright e2e (mocked API)
make test-smoke         # shell script against running compose stack
make test-performance   # vegeta load test
make test-stress        # vegeta higher-rate stress test
```

Skip testcontainers:

```bash
SKIP_TESTCONTAINERS=1 go test ./...
```

| Suite | Tag / path | What it checks |
|-------|------------|----------------|
| Unit | default packages | Handlers, domain, crypto, … |
| Integration | `tests/integration` | API + real Postgres + NATS |
| Contract | `tests/contract` | JSON camelCase vs frontend |
| Security | `tests/security` | CSRF, headers, RBAC, secrets |
| Web e2e | `web/e2e` | Playwright against preview (mocked `/api`) |
| Smoke | `tests/e2e/smoke.sh` | Health, login, streams, live WS on compose |
| Performance | `tests/performance/load.sh` | vegeta baseline (main CI) |
| Stress | `tests/performance/stress.sh` | vegeta higher RPS (main CI) |

All Go tests use **testify** (`require` / `assert`). Manual UI checks after deploy: [manual-test-checklist.md](manual-test-checklist.md).

---

## Linting

```bash
make lint          # Go + web
make lint-go-fix   # auto-fix struct alignment, modernize, etc.
```

CI runs on every pull request to `main` (`.github/workflows/test.yml`): Go lint/tests/build, web lint/typecheck/build/Playwright e2e, parallel regression suites, race detector (live WebSocket), and compose smoke, plus an **All checks passed** gate. Performance and stress baselines run on pushes to `main` only (advisory).

---

## Useful Makefile targets

| Target | Description |
|--------|-------------|
| `make dev` | `go run ./cmd/server` |
| `make dev-web` | Vite dev server |
| `make docker-up` | `docker compose up --build -d` |
| `make seed-demo` | Sample streams for topology demo |
| `make tidy` | `go mod tidy` |

---

## Debugging tips

### Enable pprof locally

```bash
PPROF_ENABLED=true go run ./cmd/server
```

On-demand profiles: `GET /api/v1/pprof/profile/{heap|cpu|goroutine|...}` (admin auth when enabled). Non-prod also exposes `/debug/pprof`.

### Structured logs

```bash
LOG_JSON=true LOG_LEVEL=debug go run ./cmd/server
```

### NATS connection issues

- Check `GET /api/v1/clusters/{id}/connection` for cached client status  
- Manager code: `internal/nats/manager.go`  

---

## Contributing workflow

1. Fork / branch  
2. `make lint && make test-regression`  
3. Keep diffs focused — match existing patterns  
4. Add contract tests if you change API JSON shape  
5. Open PR with test plan  

---

## Where to learn more

- [User guide](./user-guide.md) — feature behavior from an operator's view  
- [DevOps setup](./devops-setup.md) — production deployment  
- [Main README](../README.md) — env reference, RBAC details
