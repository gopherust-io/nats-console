# NATS Consol — Backend Architecture

The backend follows **Domain-Driven Design (DDD)** with a **hexagonal (ports & adapters)** layout. Business rules live in the center; infrastructure and delivery mechanisms plug in through interfaces.

## Layer overview

```
┌─────────────────────────────────────────────────────────────┐
│  Driving adapters (HTTP, WebSocket)                         │
│  internal/api, internal/live                                │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│  Application services                                       │
│  internal/app                                               │
└──────────────────────────┬──────────────────────────────────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
┌───────▼───────┐  ┌───────▼───────┐  ┌───────▼───────┐
│  Domain       │  │  Ports        │  │  (same ports) │
│  internal/    │  │  internal/    │  │               │
│  domain       │  │  port         │  │               │
└───────────────┘  └───────┬───────┘  └───────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│  Driven adapters (Postgres, NATS, Gemini)                   │
│  internal/adapter/postgres, internal/adapter/nats           │
│  internal/store, internal/nats (legacy implementations)     │
└─────────────────────────────────────────────────────────────┘
```

## Packages

| Layer | Path | Responsibility |
|-------|------|----------------|
| **Domain** | `internal/domain` | Entities, value objects, domain errors (`ErrNotFound`), RBAC helpers |
| **Ports** | `internal/port` | Repository and gateway interfaces consumed by application services |
| **Application** | `internal/app` | Use cases: clusters, health, users, audit |
| **Adapters** | `internal/adapter/postgres` | Postgres persistence via `UnitOfWork` |
| **Adapters** | `internal/adapter/nats` | NATS cluster gateway and JetStream executor |
| **Driving** | `internal/api` | FastHTTP handlers, middleware, routing |
| **Driving** | `internal/live` | WebSocket live stream viewer |
| **Composition** | `internal/bootstrap` | Wires adapters and services in `cmd/server` |

## Bounded contexts

- **Cluster management** — register NATS clusters, credentials, default cluster bootstrap, connectivity tests.
- **JetStream operations** — streams, consumers, messages, KV, object store (via `port.JetStreamExecutor`).
- **Identity & access** — auth sessions, OIDC, RBAC (`internal/auth`; still uses raw store during migration).
- **Audit** — request audit trail (`app.AuditService` + middleware writer).
- **Assistant** — Gemini-powered chat (`internal/assistant`).

## Dependency rule

Dependencies point **inward**:

- `domain` has no imports from other internal packages.
- `port` depends only on `domain` (and NATS SDK types where needed).
- `app` depends on `port` and `domain`.
- Adapters implement `port` interfaces and may use `internal/store` / `internal/nats`.
- HTTP handlers depend on `app.Services`, not on Postgres or NATS directly.

## Key interfaces

```go
// Persistence — internal/port/repository.go
type UnitOfWork interface {
    ClusterRepository
    UserRepository
    AuditRepository
    Ping(ctx context.Context) error
}

// NATS — internal/port/nats.go
type ClusterGateway interface {
    BootstrapDefault(ctx context.Context) error
    Test(ctx context.Context, clusterID string) (domain.ClusterTestResult, error)
    WithExecutor(ctx context.Context, clusterID string, fn func(JetStreamExecutor) error) error
    GetExecutor(ctx context.Context, clusterID string) (JetStreamExecutor, error)
    Evict(clusterID string)
    Close()
}
```

## Request flow (example: list streams)

1. `GET /api/v1/clusters/{id}/streams` → `internal/api/handlers.go`
2. Handler calls `svc.JetStream.WithExecutor(...)`
3. `adapter/nats.Gateway` resolves cluster client from Postgres config
4. `JetStreamExecutor.StreamNames` delegates to `internal/nats.Client`
5. JSON response via `pkg/common/serializer`

## Bootstrap

`cmd/server/main.go` calls `bootstrap.New`, which:

1. Opens Postgres (`adapter/postgres`)
2. Initializes auth and seeds admin user
3. Creates NATS manager + gateway adapter
4. Builds `app.Services` and bootstraps default cluster
5. Optionally enables Gemini assistant

## Migration notes

- `internal/store` and `internal/nats` remain as infrastructure implementations; they are not imported from HTTP handlers.
- `auth` and `audit.Writer` still accept `*store.Store` via `UnitOfWork.Raw()` — a future step is dedicated auth/audit adapters.
- `internal/api` is the HTTP driving adapter; renaming to `internal/adapter/http` is optional.

## Adding a feature

1. Add or extend types in `internal/domain`.
2. Add port methods in `internal/port` if a new external dependency is needed.
3. Implement the port in the appropriate adapter.
4. Add application logic in `internal/app`.
5. Expose via `internal/api` handler calling `app.Services`.
