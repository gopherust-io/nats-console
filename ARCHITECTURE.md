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
│  internal/app (incl. JetStreamService)                      │
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
│  internal/repo, internal/nats (infrastructure)              │
└─────────────────────────────────────────────────────────────┘
```

Composition root: `internal/bootstrap` (wired from `cmd/main.go`).

### Bounded contexts

- **Cluster management** — register NATS clusters, credentials, default cluster bootstrap, connectivity tests.
- **JetStream operations** — streams, consumers, messages, KV, object store (via `app.JetStreamService` → `port.ClusterGateway` / `port.JetStreamExecutor`).
- **Identity & access** — auth sessions (`internal/auth` returns `domain.User`); authorization via `domain` authz + `app/policy`.
- **Audit** — request audit trail (`app.AuditService` + middleware writer).
- **Assistant** — Gemini-powered chat (`internal/assistant`).
- **Insights** — topology, zombies, architecture score/review, chaos story (via `app/monitoring` JSZ engine).

## Packages

| Layer | Path | Responsibility                                                                                                     |
|-------|------|--------------------------------------------------------------------------------------------------------------------|
| **Domain** | `internal/domain` | Entities, value objects, domain errors (`ErrNotFound`), authz (`Can*`, Action/Resource) |
| **Ports** | `internal/port` | Per-context repository interfaces + composed `DB`; NATS gateway / executor |
| **Application** | `internal/app` | Use cases: clusters, JetStream facade, health, users, audit, access, alerts, NATS accounts, metrics, incidents, event catalog, admin |
| **Application** | `internal/app/monitoring` | JSZ topology types, short-TTL fetch/parse cache, insight extractors (zombies, genome, catalog, …) |
| **Application** | `internal/app/query` | CQRS-lite monitoring reads: prefer snapshot hub, else live executor |
| **Application** | `internal/app/policy` | Authorize helpers wrapping domain authz for handlers |
| **Adapters** | `internal/adapter/postgres` | Postgres persistence via `DB` (wraps `internal/repo`)                                                              |
| **Adapters** | `internal/adapter/nats` | Cluster gateway; `GetExecutor`/`WithExecutor` go through session fabric |
| **Infrastructure** | `internal/nats` | Manager + `Session` fabric (health probe, backoff, dial/executor metrics) |
| **Driving** | `internal/api` | Router, auth/users/access/alerts; shared envelopes |
| **Driving** | `internal/api/apikit` | Shared HTTP/NATS core (`Action`/`Void`/`Raw`), pagination, validation, errors |
| **Driving** | `internal/api/jetstream` | Streams, consumers, messages, DLQ, incident capsules |
| **Driving** | `internal/api/kvobj` | KV and object store |
| **Driving** | `internal/api/accounts` | NATS account JWT users / exports |
| **Driving** | `internal/api/insights` | Topology, zombies, architecture, chaos, monitoring raw |
| **Driving** | `internal/api/ops` | Clusters, connection SSE, pprof, health/OpenAPI |
| **Driving** | `internal/live` | WebSocket live stream viewer                                                                                       |
| **Composition** | `internal/bootstrap` | Wires adapters and services; `cmd/main` loads telemetry, starts HTTP, signals, then `App.Close`                    |

## Key design rules

Dependencies point **inward**:

- `domain` has no imports from other internal packages.
- `port` depends only on `domain` (and NATS SDK types where needed).
- `app` depends on `port` and `domain`.
- Adapters implement `port` interfaces and may use `internal/repo` / `internal/nats`.
- HTTP handlers depend on `app.Services`, not on Postgres or NATS directly.
- `NewServices` takes `port.DB` for composition only; constructors receive the embedded repository interfaces they need.

### Migration notes

- Persistence lives in `internal/repo` (formerly referred to as store); HTTP handlers call `app.Services` for persistence (no direct repo access from handlers).
- Auth public APIs and middleware use `domain.User`; `repo.User` converts only at the DB boundary (`StoreUserToDomain` / postgres adapter).
- `auth` and `audit.Writer` still accept `*repo.DB` via `db.DB()` — a future step is dedicated auth/audit adapters.
- Snapshot metrics collector still uses `db.DB()` for writes.
- HTTP is split by bounded context under `internal/api/{jetstream,kvobj,accounts,insights,ops}`; renaming the root to `internal/adapter/http` is optional.

## Core APIs / interfaces

```go
// Persistence — internal/port/repository.go
type DB interface {
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
    Stop()
}

// NATS — internal/port/nats.go
type ClusterGateway interface {
    BootstrapDefault(ctx context.Context) error
    Test(ctx context.Context, clusterID string) (domain.ClusterTestResult, error)
    WithExecutor(ctx context.Context, clusterID string, fn func(JetStreamExecutor) error) error
    GetExecutor(ctx context.Context, clusterID string) (JetStreamExecutor, error)
    Evict(clusterID string)
    Touch(clusterID string)
    Stop()
}

// Application facade — internal/app/jetstream.go
type JetStreamService struct { /* wraps port.ClusterGateway */ }
// Handlers call svc.JetStream.WithExecutor / GetExecutor; live.Hub uses Gateway().
```

## Request / call flow

Example: list streams

1. `GET /api/v1/clusters/{id}/streams` → `internal/api/jetstream`
2. Handler uses `apikit.Core.Action` → `svc.JetStream.WithExecutor(...)`
3. `app.JetStreamService` → `adapter/nats.Gateway` → `Manager.Session` (health probe + scoped timeout)
4. `JetStreamExecutor.StreamNames` delegates to `internal/nats.Client`
5. JSON response via `pkg/common/serializer`

Example: topology insight

1. `GET .../topology` → `internal/api/insights`
2. `svc.Monitoring.FetchJSZ` (TTL cache) via `app/query` (hub snapshot preferred, else live)
3. Domain/monitoring extractors build the response tree

## Bootstrap / lifecycle

`cmd/main.go` initializes telemetry, then calls `bootstrap.New`, which:

1. Loads config and opens Postgres (`adapter/postgres`)
2. Initializes auth and seeds admin user
3. Creates NATS manager + gateway adapter
4. Builds `app.Services` (including `JetStreamService`, `Monitoring`, `Queries`) and bootstraps the default cluster
5. Optionally enables Gemini assistant
6. Starts metrics snapshot + audit writer, wires `SetSnapshotHub` / `ConfigureMonitoring`, and builds the HTTP handler

On signal: shut down the HTTP server, then `App.Close` (metrics, audit, gateway, db, mailer), then telemetry.

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
