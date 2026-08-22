# 5-node NATS JetStream cluster lab

Local insecure lab for stream `Replicas: 5` and the Consol **Replicas** page.

Included by the root [`docker-compose.yml`](../../../docker-compose.yml) under Compose
profile **`cluster`** (same Docker Desktop project: `nats-consol`). Nodes also join the
project default network so console/fleet can reach `nats-cluster-1`, …

```bash
make nats-cluster-up
# equivalent:
#   docker compose stop nats
#   docker compose --profile cluster up -d nats-1 nats-2 nats-3 nats-4 nats-5
```

| | |
|--|--|
| JetStream domain | `hub` |
| Client ports | `4222–4226` |
| Monitoring | `8222–8226` |
| Image | `nats:2.14` |

**Stop** the root compose `nats` service first — `make nats-cluster-up` does this
automatically (`4222`/`8222` clash otherwise).

Point Consol at:

```bash
NATS_URL=nats://127.0.0.1:4222,nats://127.0.0.1:4223,nats://127.0.0.1:4224,nats://127.0.0.1:4225,nats://127.0.0.1:4226
NATS_MONITORING_URL=http://127.0.0.1:8222
```

From the console container on the compose network, prefer `nats://nats-cluster-1:4222,…`
(or `host.docker.internal` for host-published monitors).

Fleet clients default to the same multi-URL list (`make fleet-up`) so connections
spread across replicas and reconnect to survivors when a node drops.

Tear down cluster nodes only (wipes JetStream volumes); then restart single `nats` if needed:

```bash
make nats-cluster-down
docker compose up -d nats
```

Not for production (no TLS / auth). See also [docs/local-docker.md](../../../docs/local-docker.md).
