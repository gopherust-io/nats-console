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
| **Admin** | Create/edit/delete streams, consumers, KV, object stores; users, audit, clusters (may be scoped) |
| **Root** | Full access; creates other admins with optional limits |

New invited users start with the roles/grants your admin assigned until promoted under **Users & Roles** or Access.

---

## Navigation basics

```text
Sidebar
├── Overview
│   ├── Dashboard      ← account usage & health
│   └── Clusters       ← register / test NATS endpoints
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

1. Open **Clusters** (or **Systems → Manage Systems**)  
2. Review name, NATS URL, and monitoring URL for each registered system  
3. **Test** — verifies the console can reach NATS + JetStream  

### Tips

- The console server must reach both URLs from **its** network (not from your laptop, unless you're on VPN).  
- The default cluster is seeded from `NATS_URL` / `NATS_MONITORING_URL` (and related creds) when the registry is empty.  
- Credentials are encrypted at rest; they are never shown again in the API after save.

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
2. Enter payload as **JSON** or **raw text** (sent as base64 to the API)
3. Click **Publish** — the ack shows the new sequence number

Useful for smoke tests without leaving the console. Operators and viewers do not see the publish form.

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

When the console is served over HTTP/3 (via Caddy or Ingress), live tail still uses WebSocket over HTTP/1.1 or HTTP/2 upgrade — browsers negotiate this automatically.

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

A visual tree of streams and their consumers — helpful when onboarding or debugging complex setups.

- Stream nodes show name and basic stats  
- Consumer nodes hang under their stream  
- Refresh to pick up changes  

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
Wrong cluster selected, or NATS credentials expired. Run **Test** on the Clusters page.

**Can the UI connect directly to NATS?**  
No — by design. All traffic goes through the Consol API so credentials and monitoring stay server-side.

**Where are messages stored?**  
In JetStream on the NATS server — the console only reads them through the API.

---

## Need the API?

Integrate automation via REST: see [OpenAPI](../api/openapi.yaml) or `GET /api/openapi.yaml` on your server.

All JSON uses **camelCase** field names to match the frontend.
