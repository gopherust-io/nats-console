# User guide

A friendly tour of NATS Consol for **developers** and **operators** who use the web UI to work with JetStream.

---

## Signing in

The login screen adapts to how your admin configured the server:

| Method | When you see it |
|--------|-----------------|
| **Username & password** | `BASIC_AUTH_ENABLED=true` (default) |
| **SSO buttons** (Google, GitHub, etc.) | Provider env vars enabled |
| **Continue with SSO** | Generic OIDC / Keycloak |

Your role controls what you can click:

| Role | What you can do |
|------|-----------------|
| **Viewer** | Read dashboard, browse streams/messages, live tail, KV/objects |
| **Operator** | Everything viewers can do + create/edit/delete streams, consumers, KV, objects |
| **Admin** | Operator powers + users, audit log, cluster management (may be scoped) |
| **Root** | Full access; creates other admins with optional limits |

New SSO users usually start as **viewer** until an admin promotes them under **Users & Roles**.

---

## Navigation basics

```text
Sidebar
├── Overview
│   ├── Dashboard      ← account usage & health
│   └── Clusters       ← register / test NATS endpoints
├── JetStream
│   ├── Topology       ← stream/consumer map
│   ├── Supercluster   ← routes, gateways, leaf nodes
│   ├── Streams        ← core JetStream work
│   ├── KV Stores
│   └── Object Stores
└── Administration
    ├── Settings
    ├── Audit Log      ← admins
    ├── Users & Roles  ← admins
    └── Profiling      ← admins (if enabled)
```

**Active cluster** — always check the dropdown at the top of the sidebar. All JetStream pages use that cluster.

---

## Dashboard

Your home base. Shows for the active cluster:

- JetStream memory / storage usage  
- Stream and consumer counts  
- Server info from NATS monitoring  

If numbers look stale, switch cluster away and back, or refresh the page.

---

## Clusters

Register every NATS JetStream deployment your team should manage.

### Add a cluster

1. **Clusters → Add Cluster**  
2. Fill in:
   - **Name** — friendly label (`production-us`, `staging`, …)  
   - **NATS URL** — client connection, e.g. `nats://nats.internal:4222`  
   - **Monitoring URL** — HTTP monitoring port, e.g. `http://nats.internal:8222`  
3. Optionally add **token** or **creds file** content for auth (stored encrypted)  
4. **Test** — verifies the console can reach NATS + JetStream  

### Tips

- The console server must reach both URLs from **its** network (not from your laptop, unless you're on VPN).  
- Credentials are encrypted at rest; they are never shown again in the API after save.  
- Only **admins** (with permission) can delete cluster registrations.

---

## Streams & consumers

### Streams list

Create, edit, delete, and purge streams. Lists respect pagination — use search/filters where available.

### Stream detail

| Tab / action | Purpose |
|--------------|---------|
| **Overview** | Config, state, subjects |
| **Consumers** | Create and inspect pull/push consumers |
| **Messages** | Fetch by sequence; prev/next navigation |
| **Live** | WebSocket tail — watch messages as they arrive |
| **Purge** | Delete all messages (operator+) |

### Live mode

1. Open a stream → **Live**  
2. Keep the tab open — messages stream in via WebSocket  
3. Publish from your app or `nats pub` to see traffic  

Live sessions are rate-limited server-side to protect NATS.

---

## KV Stores

Key-Value buckets backed by JetStream.

- **List buckets** — see all KV stores on the cluster  
- **Open a bucket** — browse keys  
- **Key detail** — value, revision, history  
- **Put / delete** — operator+  

Great for feature flags, small config, leader election metadata.

---

## Object Stores

Large blob storage on JetStream.

- Browse buckets and objects  
- Upload / download / delete objects (operator+)  

Use for files, artifacts, or anything too big for KV.

---

## Topology

A visual tree of streams and their consumers — helpful when onboarding or debugging complex setups.

- Stream nodes show name and basic stats  
- Consumer nodes hang under their stream  
- Refresh to pick up changes  

---

## Supercluster

**Read-only** view of NATS server mesh features:

- **Routes** — cluster routing mesh  
- **Gateways** — supercluster gateways  
- **Leaf nodes** — edge connections  
- **JetStream meta / replication** — when present  

If you see "Standalone cluster", your NATS server simply isn't configured with routes/gateways yet — that's normal for single-node dev setups. Supercluster **configuration** still happens in NATS server config files, not in this UI.

---

## Administration

### Settings

Theme and UI preferences (icon style, etc.).

### Audit log (admin)

Every mutating API call (create/update/delete) is logged with:

- Who (`actor`)  
- What (`action`, resource)  
- When, request ID, client IP  

Useful for compliance and "who purged that stream?" moments.

### Users & roles (admin / root)

- **Root** creates delegated **admin** users with optional **access rules**:
  - Limit to specific `clusterIds` (required for operator/viewer and scoped admins)
  - Allow/deny user management, audit, cluster delete
  - Restrict which roles they may assign
- Legacy admins without access rules keep full admin powers
- **Operator** and **viewer** users must have at least one cluster in `clusterIds` — they only see and act on those clusters

**Upgrading to v0.5:** Assign `clusterIds` to existing operator/viewer accounts; empty scope no longer grants access to all clusters.

### Profiling (admin, optional)

When ops enables `PPROF_ENABLED=true`:

- Live goroutine and memory stats  
- Collect heap / CPU / goroutine profiles  
- Bar-chart view of hot functions  
- Download raw `.pprof` for `go tool pprof`  

This profiles the **console server process**, not your NATS workloads.

---

## AI assistant (optional)

If your admin set `AI_ENABLED=true` and a Gemini API key:

1. Click the **AI** button (bottom-right)  
2. Ask JetStream questions in plain English  

The assistant only sees JetStream/console context — not your Postgres rows or raw credentials.

---

## Keyboard & UX tips

- **Sidebar** — collapses on small screens; use the menu button to reopen  
- **Cluster switch** — your choice is remembered in the browser  
- **Errors** — red banners usually include the API message; 403 means "wrong role", 401 means "sign in again"  

---

## Common questions

**Why can't I create a stream?**  
You need **operator** or higher. Ask an admin to check your roles.

**Why is my cluster empty?**  
Wrong cluster selected, or NATS credentials expired. Run **Test** on the Clusters page.

**Can the UI connect directly to NATS?**  
No — by design. All traffic goes through the Consol API so credentials and monitoring stay server-side.

**Where are messages stored?**  
In JetStream on the NATS server — the console only reads them through the API.

---

## Need the API?

Integrate automation via REST: see [OpenAPI](../api/openapi.yaml) or `GET /api/openapi.yaml` on your server.

All JSON uses **camelCase** field names to match the frontend.
