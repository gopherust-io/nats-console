# User guide

A friendly tour of NATS Consol for **developers** and **operators** who use the web UI to work with JetStream.

---

## Signing in

Sign in with **username and password**. There is no public sign-up and no SSO — an administrator invites you, you set a password at `/invite/:token`, then use that password on the login page.

Your role controls what you can click:

| Role | What you can do |
|------|-----------------|
| **Viewer** | Read dashboard, browse streams/messages, live tail, KV/objects |
| **Operator** | Everything viewers can do + manage NATS users/access (not JetStream create/edit/delete) |
| **Admin** | Create/edit/delete streams, consumers, KV, object stores; users, audit; delete clusters (may be scoped) |
| **Root** | Full access; creates other admins with optional limits |

New invited users start with the roles/grants your admin assigned until promoted under **Users & Roles** or Access.

---

## Navigation basics

```text
Sidebar
├── Overview
│   ├── Dashboard      ← account usage & health
│   └── Systems        ← systems + Clusters entrance card
│       └── System tabs: Overview · Usage · Replicas · Access
├── JetStream
│   ├── Topology       ← stream/consumer map
│   ├── Streams        ← core JetStream work
│   ├── KV Stores
│   └── Object Stores
└── Administration
    ├── Settings
    ├── Audit Log      ← admins
    ├── Alerts         ← open/closed alert feed (bell badge)
    ├── Alert rules    ← admins: metric thresholds
    └── Console Users  ← admins
```

**Active cluster** — always check the dropdown at the top of the sidebar. All JetStream pages use that cluster.

---

## Alerts

When metrics snapshots are enabled (`METRICS_SNAPSHOT_ENABLED`), the server evaluates **alert rules** after each scrape:

1. Create or enable a rule under **Administration → Alert rules** (pick a metric, comparator, threshold, severity).
2. When the threshold is met, an **open** alert is written to the feed and the topbar **bell** shows a count.
3. Open **Administration → Alerts** to see Open / Closed lists. **Acknowledge** hides the alert from the bell for everyone; it stays open until the metric recovers, then auto-closes.
4. Seeded default rules ship **disabled** (high CPU, connections, JetStream storage) — enable them when you want coverage without creating rules from scratch.

When **SMTP** is enabled (`SMTP_ENABLED=true`), newly opened alerts are also emailed to console users who have a real email address (not `*@local`) and can access that cluster. The topbar bell and alert feed still work without SMTP.

Alerts profile the **console’s view of cluster metrics**, not a separate Prometheus stack.

### Slow consumers

Each metrics snapshot evaluates every JetStream consumer against shared thresholds (defaults match the `nats` library `WatchSlowConsumer`):

| Signal | Default |
|--------|---------|
| `NumPending` | `>= 1000` (`SLOW_CONSUMER_PENDING_THRESHOLD`) |
| Lag (`LastSeq − Delivered`) | `>= 1000` (`SLOW_CONSUMER_LAG_THRESHOLD`) |
| `NumAckPending` / `MaxAckPending` | `>= 90%` (`SLOW_CONSUMER_ACK_PENDING_RATIO`) when max ack pending is set |

Useful alert metrics:

| Metric | Meaning |
|--------|---------|
| `jetstream.slow_consumers` | Count of consumers matching thresholds |
| `jetstream.consumer_max_lag` | Highest lag across consumers |
| `jetstream.consumer_max_pending` | Highest `NumPending` |
| `jetstream.consumer_max_ack_pending` | Highest `NumAckPending` |

Example: alert when `jetstream.slow_consumers` **gte** `1`, or when `jetstream.consumer_max_lag` **gt** `5000`. Stream/consumer pages and Topology show a **Slow** badge when a durable exceeds thresholds.

### Behavior fingerprinting

Workers using the `nats` library `WatchBehaviorFingerprint` publish Normal / Current **msg/min** and **processing latency** to JetStream KV bucket `nats_consol_fingerprints` (key `{stream}/{durable}`; override with `BEHAVIOR_FINGERPRINT_KV_BUCKET`).

Consumer Detail shows those snapshots and an **Anomaly** chip when the worker reports a latency regression at stable throughput. If no snapshot exists yet, the panel stays idle (not an error).

### Hidden bottlenecks

**Docs → Hidden Bottlenecks** mines recurring **weekday × hour** patterns from compact hourly rollups (default retention `METRICS_SNAPSHOT_BOTTLENECK_RETENTION=672h`, independent of the 7-day raw metrics window). This is **not** the same as slow-consumer lag/CPU threshold alerts.

Consol derives average payload size from consecutive scrapes (`Δbytes / Δlast_seq` → `stream:{name}:avg_payload_bytes`) and folds consumer lag (plus optional fingerprint processing latency) into hour buckets. The miner looks for correlations such as “every Friday at 18:00 UTC, `billing-worker` lag rises when `ORDERS` average payload doubles.”

Findings appear as a deterministic list with optional **Ask AI** narration (Gemini). Use **Show sample** for the canned Friday 18:00 example when history is thin. Consumer Detail shows a **Hidden bottleneck** chip linking to Docs when that durable appears in the cluster snapshot.

API: `GET /api/v1/clusters/{id}/hidden-bottlenecks`, `POST …/hidden-bottlenecks/ask`.

---

## Dashboard

Your home base. Shows for the active cluster:

- JetStream memory / storage usage  
- Stream and consumer counts  
- Server info from NATS monitoring  
- **Trends** — historical charts for storage, memory, messages, and server traffic (1h–7d ranges)

Historical metrics are collected in the background and stored in PostgreSQL. New deployments show “Collecting data…” until the first samples arrive.

If numbers look stale, switch cluster away and back, or refresh the page.

---

## Clusters

Cluster registrations are configured by **DevOps** (environment variables on first boot, Helm values, or direct Postgres updates)—not from the console UI.

### View & test

1. Open **Systems** and choose the **Clusters** card  
2. Review name, NATS URL, and monitoring URL for each registered system  
3. **Check availability** — verifies the console can reach NATS + JetStream  

### Tips

- The console server must reach both URLs from **its** network (not from your laptop, unless you're on VPN).  
- The default cluster is seeded from `NATS_URL` / `NATS_MONITORING_URL` (and related creds) when the registry is empty.  
- Credentials are encrypted at rest; they are never shown again in the API after save.

### Server replicas (view-only)

Open a system → **Replicas** tab (`/systems/:clusterId/replicas`).

The page lists NATS server peers projected from monitoring (`varz`, `routez`, JetStream `meta_cluster`):

- Cluster size, online count, JetStream meta leader, and the monitored server name  
- Per-peer role (`monitored` / `route` / `meta`), online status, uptime, RTT, and route traffic  

**Note:** Full `varz` stats (CPU, memory, connections, version) apply only to the node behind `NATS_MONITORING_URL`. Other peers are derived from routes and JetStream meta.

For a local 5-node JetStream lab (supports stream `Replicas: 5`), run `make nats-cluster-up` ([`docker/nats/cluster/`](../docker/nats/cluster/); [local Docker](./local-docker.md); ports `4222–4226`, monitor `8222`). Stop the root compose `nats` service first. Point Consol at:

```bash
NATS_URL=nats://127.0.0.1:4222,nats://127.0.0.1:4223,nats://127.0.0.1:4224,nats://127.0.0.1:4225,nats://127.0.0.1:4226
NATS_MONITORING_URL=http://127.0.0.1:8222
```

For company-wide layout (many teams, many systems): see [Company-wide scaling](./company-scale.md).

---

## Streams & consumers

### Lifecycle

Operators can **view** JetStream resources. **Admins** (and root) manage the full lifecycle from the console:

| Resource | Create | Update | Delete |
|----------|--------|--------|--------|
| **Stream** | JetStream hub → Create → Stream | Stream detail → **Edit config** | Streams list → **Delete** |
| **Mirror** | JetStream hub → Create → Mirror | Same as stream (mirrors are streams with a mirror source) | Same as stream |
| **Consumer** | Stream detail → Create consumer | Consumer detail → **Edit config** | Consumer detail → **Delete** |
| **KV bucket** | JetStream hub → Create → KV | Bucket detail → **Edit config** | KV buckets list → **Delete** |
| **Object store** | JetStream hub → Create → Object | Bucket detail → **Edit config** | Object buckets list → **Delete** |

Streams also support **Purge** (clear messages, keep the stream). Consumers support **Replay** (reposition delivery) from the consumer detail page.

### Streams list

Create, edit, delete, and purge streams. Lists respect pagination — use search/filters where available.

### Mirrors

A mirror is a stream that continuously copies another stream. Create one from the JetStream hub (**Create → Mirror**), set the source stream (and optional filter / start options), then manage it like any other stream: edit config on stream detail, delete from the streams list.

### Stream detail

| Tab / action | Purpose |
|--------------|---------|
| **Overview** | Config, state, subjects; **Edit config** for streams and mirrors |
| **Consumers** | Create, inspect, then open a consumer to edit or delete |
| **Messages** | Fetch by sequence; prev/next navigation; **publish** test messages (admin+) |
| **Live** | WebSocket tail — watch messages as they arrive |
| **Purge** | Delete all messages (admin+) |

### Publish messages

On the **Messages** tab, admins can publish directly to the stream:

1. Choose a **subject** (dropdown lists stream subjects; required when the stream uses wildcards)
2. Enter payload as **JSON**, MessagePack, CBOR, or Protobuf
3. Click **Publish** — the ack shows the new sequence number

**Quick sample** publishes a default `{"hello":"world"}` JSON message in one click.

**Payload templates** — save the current subject/format/payload under a name, then reload it later from the same browser (stored in localStorage).

Useful for smoke tests without leaving the console. Operators and viewers do not see the publish form.

### Export and import messages

- **Download** on a loaded message (or live buffer) exports JSON, CSV, Excel, PDF, text, or native binary formats.
- **Import** (admin+) accepts a JSON file in the same shape as the JSON export (single object or array with `subject` + `payload`, or wire `data` as base64). Confirmed batches are published sequentially (max 100 messages per file).

### Favorite streams

Star a stream from the Streams list or stream detail header. Use **Favorites only** on the Streams tab to filter the list. Favorites are stored in the browser (localStorage) per system + stream name.

### All streams (multi-system view)

Open **All streams** from Systems (or the top nav) to see streams across every registered system in one table. Click a row to switch to that system and open the stream. JetStream detail pages remain single-system; this view is a unified index, not a merged live tail.

### Choosing a consumer type (for NATS clients)

Use the console to create the consumer your **application** will bind to. Recommendation:

| Client goal | Use |
|-------------|-----|
| Most app workers / services | **Durable pull** — leave **Deliver subject** empty; set **Filter subjects** to the stream subjects you need; pull or consume from the client |
| Server push to an inbox / shared workers | **Durable push** — set **Deliver subject**; optional **Deliver group** for queue sharing |
| Watch traffic in the console | **Live** tab — ephemeral viewer only; not for production clients |

Typical client flow:

1. Create a durable pull consumer on the stream (filter subjects matching what you want).  
2. In your NATS client, bind that durable name and pull/consume messages (ack as required by the ack policy).  
3. Do **not** treat the console **Live** WebSocket session as an application subscription.

### Live mode

1. Open a stream → **Live**  
2. Keep the tab open — messages stream in via WebSocket  
3. Publish from your app or `nats pub` to see traffic  

Live is for operators watching a stream. Application clients should use a durable pull (or push) consumer — see [Choosing a consumer type](#choosing-a-consumer-type-for-nats-clients).

Live sessions are rate-limited server-side to protect NATS.

---

## Connection Inspector

Use **Connections** to inspect live client sessions from NATS `connz` for the selected cluster/account.

- Client name, RTT, IP, TLS version, user, account
- Connected-since timestamp
- Published and received message counters
- Slow consumer indicator when NATS marks the connection as slow

Values are live monitoring snapshots (not historical series).

---

## Request / Reply (Overview)

Account **Overview** shows passive request/reply KPIs (requesters, responders, median connection RTT) derived from NATS `connz` — no active probes or pings.

---

## KV Stores

Key-Value buckets backed by JetStream.

- **List buckets** — see all KV stores on the cluster; **delete** a bucket from the list (admin+)  
- **Open a bucket** — browse keys; **Edit config** to update the bucket (admin+)  
- **Key detail** — value, revision, history  
- **Put / delete keys** — admin+  

Great for feature flags, small config, leader election metadata.

---

## Object Stores

Large blob storage on JetStream.

- Browse buckets and objects  
- **Edit bucket config** on bucket detail; **delete** a bucket from the object buckets list (admin+)  
- Upload / download / delete objects (admin+)  

Use for files, artifacts, or anything too big for KV.

---

## Topology

A visual tree of streams and their consumers — helpful when onboarding or debugging complex setups. Consumers that exceed slow-consumer thresholds show a **warning** status and a **slow** chip (same thresholds as alerts).

- Stream nodes show name and basic stats  
- Consumer nodes hang under their stream  
- Refresh to pick up changes  

Tabs on the Topology page:

- **Constellation** — interactive stream / subject / consumer map  
- **Zombie detection** — unused or idle JetStream entities  
- **Subject naming** — scans stream subjects and consumer filters for bad patterns (wrong case, camelCase without dots, `_`/`-` separators, shallow `{domain}.{action}` hierarchies, and conflicting spellings like `order.created` vs `Orders.Created`). Each finding includes a **Normalize to** suggestion (for example `orders.order.created`). Advisory only — nothing is renamed.
- **Event genome** — clusters subjects that are probably the same event despite synonym or singular/plural differences (for example `orders.created`, `orders.new`, and `order.created`). Each finding shows the peer cluster and a **Converge on** suggestion. Advisory only — nothing is renamed.

---

## Event Catalog

Swagger-style documentation for concrete NATS events (`orders.created`, …). The **Docs** tab (account nav) auto-discovers subjects from JetStream stream configs and consumer filters, then lets you attach:

- **Owner** (free-form team name)  
- **Description** (shown as Purpose in Event Wikipedia)  
- **JSON Schema** for the payload  
- **Example** payload (curated)  
- **Deprecation** status, successor subject, and note  
- **Consumers** (live JetStream consumers whose filters match the subject)

**Docs never change JetStream.** Saving or clearing catalog fields writes only to Consol’s Postgres enrichments. Stream subjects, consumer filters, and message flow are unchanged. Undocumented live subjects still appear. Subjects documented in Postgres but missing from live inventory show as **orphan**. Clearing documentation removes the enrichment only — live inventory remains.

## Event Wikipedia

Read-only auto-assembled docs under **Docs → Event Wikipedia**. Each subject page includes Purpose, History, Owner, Consumers, Examples, Schema, Related Events (from Event Genome peers), Known Incidents (links to Audit incident reconstruction for attached consumers), and Deprecation Status. Edit enrichments in Event Catalog; Wikipedia refreshes from live inventory plus those fields. Wikipedia itself has no write API.

## Live Architecture

Under **Docs → Live Architecture** (`/docs/live-architecture`), an animated painting of Consol’s architecture: services as living nodes, traffic pulses on edges, and failure ripples. Toggle **Deploy** vs **DDD layers**, and press **F** for fullscreen. Scenarios cycle Healthy → Load spike → Failure → Recovery (simulated; not live cluster metrics).

## Chaos Story Generator

Under **Docs → Chaos Story** (`/docs/chaos-story`), AI invents a realistic multi-act disaster (for example: payment cluster down during Black Friday, JetStream quorum loss, and a consumer deploy with a schema mismatch) using only stream/consumer/subject names from the selected system. **Show sample** loads a canned Black Friday story without AI. **Simulate** plays the acts as a timed narrative playbook in the browser — it does **not** inject faults, kill nodes, or change JetStream. Requires `AI_ENABLED` only for **Generate story**.

## Architecture Review

Under **Docs → Architecture Review** (`/docs/architecture-review`), ask **Is this event architecture good?** The page shows deterministic **Problems** (too many consumers, circular dependency, tight coupling, naming, large payloads) and **Suggestions** from JetStream inventory. With `AI_ENABLED`, **Ask AI** adds a Gemini narrative over those findings (never message payloads). Use **Show sample review** for a canned showcase without cluster data. Architecture Review is Docs-only (not a Topology tab); old `/admin/topology?view=review` links redirect here. On coupling-related findings, **Reduce coupling** opens Architecture Refactor seeded by that finding. The header shows a **Score N/100** chip linking to Architecture Score.

## Architecture Score

Under **Docs → Architecture Score** (`/docs/architecture-score`), see a daily **0–100** architecture health score with **+/- factors** (naming, consumer explosion, duplicate events, payload size, latency, coupling) and a **trend over months**. Scores are computed deterministically from JetStream inventory (and consumer lag) and persisted daily by the metrics collector (~180 days). With `AI_ENABLED`, **Ask AI** narrates the score card (never message payloads). Use **Show sample score** for the 92/100 showcase without cluster data.

## Architecture Refactor

Under **Docs → Architecture Refactor** (`/docs/architecture-refactor`), ask **Reduce coupling.** The page shows a deterministic **Before** / **After** graph (e.g. `A → B → C` → event fan-out) and numbered **Migration steps** from JetStream inventory. Optional query params `kind`, `stream`, and `subject` seed the plan from an Architecture Review finding. With `AI_ENABLED`, **Ask AI** polishes the migration narrative (never message payloads). Use **Show sample plan** for the classic A→B→C showcase without cluster data. Plans are documentation only — Consol does not auto-apply JetStream migrations.

## Architecture Generator

Under **Docs → Architecture Generator** (`/docs/architecture-generator`), or **Generate architecture** on **Admin → Topology**, one click scans JetStream inventory and downloads a zip with:

- C4 (PlantUML + Mermaid container view)
- Mermaid / PlantUML / Excalidraw / Draw.io diagrams
- Markdown architecture doc
- ADRs (`0001` topology, `0002` subject boundaries)

**Download sample** works without a live cluster. With `AI_ENABLED`, optionally polish ADR prose via Gemini (diagrams stay deterministic).

---

## Administration

### Settings

Theme and UI preferences (icon style, etc.). Toggle **Console Dark** / **Console Light** from the top bar, or with **Ctrl+Shift+D** (⌘+Shift+D on macOS). The shortcut is ignored while typing in an input or textarea.

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
You need the **admin** role (or root). Operators can view JetStream but cannot create, update, or delete streams, consumers, KV, or object stores.

**Why is my cluster empty?**  
Wrong cluster selected, or NATS credentials expired. Run **Check availability** on **Systems → Clusters**.

**Can the UI connect directly to NATS?**  
No — by design. All traffic goes through the Consol API so credentials and monitoring stay server-side.

**Where are messages stored?**  
In JetStream on the NATS server — the console only reads them through the API.

---

## Need the API?

Integrate automation via **REST**: see [OpenAPI](../api/openapi.yaml) or `GET /api/openapi.yaml` on your server. There is no GraphQL surface — use the REST API (or a future OpenAPI-generated CLI) for scripts and CI.

All JSON uses **camelCase** field names to match the frontend.
