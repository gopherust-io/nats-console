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

On first boot, a **default cluster** is created automatically from `NATS_URL` and `NATS_MONITORING_URL`.

---

## Step 2 — Sign in

Open **http://localhost:8080**.

### Username & password (default)

| Field | Value |
|-------|-------|
| Username | `admin` |
| Password | `admin` |

This is the **root** account (full access). Change the password before any real deployment.

### Invite a person

1. Sign in as admin → **Admin → People**
2. Use **Invite person** to create a pending user and copy the one-time invite URL
3. Open `/invite/<token>`, set a password, and sign in
4. Assign **System** / **Account Access** from the system or account **Access** tab

Access remains invite-only — there is no public sign-up.

---

## Step 3 — Pick your cluster

Use the **Active cluster** dropdown in the left sidebar. The demo stack ships with one cluster named **default**.

Everything in JetStream (streams, KV, objects, live tail) is scoped to the cluster you select.

Open the system → **Replicas** to see NATS server peers (view-only). Against a multi-node cluster lab, you should see all route peers listed.

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

This creates sample streams/consumers (aligned with the fleet demo) so **Topology** and **Dashboard** look more interesting.

For **live load and Account → Connections** (one NATS client per service), enable the root compose **`fleet`** profile ([`examples/fleet`](../examples/fleet); image build needs a sibling [`gopherust-io/nats`](https://github.com/gopherust-io/nats) checkout for `go.mod` replace). Fleet containers join the same `nats-consol` Docker Desktop project:

```bash
make fleet-up
# or: docker compose -p nats-consol -f examples/fleet/docker-compose.yml --profile fleet up -d --build
```

Each container connects as `fleet-<service>` (visible under **Connections**). Single-process local run:

```bash
TEL_ENABLE=false NATS_URL=nats://127.0.0.1:4222 go run ./examples/fleet
```

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
