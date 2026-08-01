# Local NATS (Docker Compose)

Ready-to-run JetStream topologies for developing against NATS Consol and local brokers.

| Topology | Compose | Client `Address` |
|----------|---------|------------------|
| Single | [`single/`](single/) | `nats://127.0.0.1:4222` |
| Cluster (5 nodes) | [`cluster/`](cluster/) | `nats://127.0.0.1:4222,…,nats://127.0.0.1:4226` |
| Supercluster (2×2) | [`supercluster/`](supercluster/) | East: `…4222,4223` · West: `…4225,4226` |
| Auth (users + permissions) | [`auth/`](auth/) | `nats://127.0.0.1:4222` + User/Password |

Full guide: [docs/local-docker.md](../../docs/local-docker.md).

```bash
# Everyday local (or: make nats-up)
docker compose -f docker/nats/single/docker-compose.yml up -d

# Tear down (wipe JetStream volumes)
docker compose -f docker/nats/single/docker-compose.yml down -v
```

**Do not** run `cluster` and `supercluster` at the same time (shared host ports `4222`–`4226`). Stop the root compose `nats` service before starting these labs (same ports).

These confs have **no TLS** (auth lab has plaintext passwords) — local development only.
