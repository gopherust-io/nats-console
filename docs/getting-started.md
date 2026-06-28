# Getting started

This guide gets NATS Consol running on your laptop in a few minutes. No prior NATS experience required.

---

## What you'll need

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose  
- A web browser  

That's it for the quick path.

---

## Step 1 — Start everything

```bash
git clone https://github.com/gopherust-io/nats-consol.git
cd nats-consol
docker compose up --build
```

Wait until you see the console log line like `nats-consol v0.3 listening`.

This starts:

| Service | URL | Purpose |
|---------|-----|---------|
| **Console** | http://localhost:8080 | Web UI + API |
| **NATS** | `nats://localhost:4222` | JetStream server |
| **NATS monitoring** | http://localhost:8222 | Server metrics (used by console) |
| **PostgreSQL** | `localhost:5432` | Console database |
| **Keycloak** (dev IdP) | http://localhost:8180 | Optional SSO demo |

On first boot, a **default cluster** is created automatically from `NATS_URL` and `NATS_MONITORING_URL`.

---

## Step 2 — Sign in

Open **http://localhost:8080**.

### Option A — Username & password (easiest)

| Field | Value |
|-------|-------|
| Username | `admin` |
| Password | `admin` |

This is the **root** account (full access). Change the password before any real deployment.

### Option B — SSO (demo Keycloak)

1. Click **Continue with SSO**  
2. Sign in as `sso-user` / `sso-user`  
3. First login creates a **viewer** account (read-only)

Keycloak admin console (if you want to poke around): http://localhost:8180 — `admin` / `admin`

---

## Step 3 — Pick your cluster

Use the **Active cluster** dropdown in the left sidebar. The demo stack ships with one cluster named **default**.

Everything in JetStream (streams, KV, objects, live tail) is scoped to the cluster you select.

---

## Step 4 — Try a few things

### Create a stream

1. Go to **Streams** in the sidebar  
2. Click **Create stream**  
3. Name it `ORDERS`, subjects `orders.>`  
4. Save  

### Browse messages

1. Open your new stream  
2. Use **Messages** to inspect payloads (JSON or raw)  

### Live tail

1. On a stream detail page, open **Live**  
2. Publish a test message with the [NATS CLI](https://docs.nats.io/using-nats/nats-tools/nats_cli):

```bash
nats pub orders.new '{"id":1,"item":"coffee"}'
```

You should see it appear in the browser in real time.

### Optional — seed demo topology

```bash
make seed-demo
```

This creates sample streams/consumers so **Topology** and **Dashboard** look more interesting.

---

## Stopping the stack

```bash
docker compose down
```

Add `-v` to also remove Postgres/NATS volumes (fresh start next time).

---

## What's next?

- **Using the UI every day** → [User guide](./user-guide.md)  
- **Deploying for your team** → [DevOps setup guide](./devops-setup.md)  
- **Contributing code** → [Developer setup guide](./developer-setup.md)
