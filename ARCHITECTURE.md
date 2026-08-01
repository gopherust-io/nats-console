# NATS Consol — Architecture

Ops console for managing NATS/JetStream clusters: streams, consumers, KV, identity, audit, and live views.

## Overview

The backend follows **Domain-Driven Design (DDD)** with a **hexagonal (ports & adapters)** layout. Business rules live in the center; infrastructure and delivery mechanisms plug in through interfaces. The UI lives in `web/`; this document covers the Go backend.

Module: `github.com/gopherust-io/nats-consol` · Ecosystem: [gopherust-io](https://github.com/gopherust-io/gopherust-io/blob/main/ARCHITECTURE.md)

## Layer / package overview

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

### Bounded contexts

- **Cluster management** — register NATS clusters, credentials, default cluster bootstrap, connectivity tests.
- **JetStream operations** — streams, consumers, messages, KV, object store (via `port.JetStreamExecutor`).
- **Identity & access** — auth sessions, RBAC, invite links (`internal/auth`; still uses raw store during migration).
- **Audit** — request audit trail (`app.AuditService` + middleware writer).
- **Assistant** — Gemini-powered chat (`internal/assistant`).

## Packages

| Layer | Path | Responsibility |
|-------|------|----------------|
| **Domain** | `internal/domain` | Entities, value objects, domain errors (`ErrNotFound`), RBAC helpers |
| **Ports** | `internal/port` | Repository and gateway interfaces consumed by application services |
| **Application** | `internal/app` | Use cases: clusters, health, users, audit, access, alerts, NATS accounts, metrics, incidents, event catalog, admin |
| **Adapters** | `internal/adapter/postgres` | Postgres persistence via `UnitOfWork` |
| **Adapters** | `internal/adapter/nats` | NATS cluster gateway and JetStream executor |
| **Driving** | `internal/api` | FastHTTP handlers, middleware, routing |
| **Driving** | `internal/live` | WebSocket live stream viewer |
| **Composition** | `internal/bootstrap` | Wires adapters and services in `cmd/main` |

## Key design rules

Dependencies point **inward**:

- `domain` has no imports from other internal packages.
- `port` depends only on `domain` (and NATS SDK types where needed).
- `app` depends on `port` and `domain`.
- Adapters implement `port` interfaces and may use `internal/store` / `internal/nats`.
- HTTP handlers depend on `app.Services`, not on Postgres or NATS directly.

### Migration notes

- `internal/store` and `internal/nats` remain as infrastructure implementations behind adapters; HTTP handlers call `app.Services` for persistence (no `h.store.*`).
- `auth` and `audit.Writer` still accept `*store.Store` via `UnitOfWork.Raw()` — a future step is dedicated auth/audit adapters.
- Snapshot metrics collector still uses `UnitOfWork.Raw()` for writes.
- `internal/api` is the HTTP driving adapter; renaming to `internal/adapter/http` is optional.

## Core APIs / interfaces

```go
// Persistence — internal/port/repository.go
type UnitOfWork interface {
    ClusterRepository
    UserRepository
    AuditRepository
    EncryptionRepository
    MetricsRepository
    IncidentRepository
    EventCatalogRepository
    AlertRepository
    AccessRepository
    NATSAccountRepository
    Close()
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

## Request / call flow

Example: list streams

1. `GET /api/v1/clusters/{id}/streams` → `internal/api/handlers.go`
2. Handler calls `svc.JetStream.WithExecutor(...)`
3. `adapter/nats.Gateway` resolves cluster client from Postgres config
4. `JetStreamExecutor.StreamNames` delegates to `internal/nats.Client`
5. JSON response via `pkg/common/serializer`

## Bootstrap / lifecycle

`cmd/main.go` calls `bootstrap.New`, which:

1. Opens Postgres (`adapter/postgres`)
2. Initializes auth and seeds admin user
3. Creates NATS manager + gateway adapter
4. Builds `app.Services` and bootstraps default cluster
5. Optionally enables Gemini assistant

## Adding a feature

1. Add or extend types in `internal/domain`.
2. Add port methods in `internal/port` if a new external dependency is needed.
3. Implement the port in the appropriate adapter.
4. Add application logic in `internal/app`.
5. Expose via `internal/api` handler calling `app.Services`.

## Related docs

- [README](README.md)
- [Getting started](docs/getting-started.md)
- [User guide](docs/user-guide.md)
- [DevOps setup](docs/devops-setup.md)
- [Developer setup](docs/developer-setup.md)
- Live Architecture Painting — console UI at `/docs/live-architecture` (animated deploy / DDD-layer view)
- Architecture Review — `/docs/architecture-review` (Docs only; Topology `?view=review` redirects here)
- Architecture Generator — `/docs/architecture-generator` and Topology **Generate architecture**
- Chaos Story Generator — `/docs/chaos-story` (AI invent + narrative Simulate; Docs only, no fault injection)
