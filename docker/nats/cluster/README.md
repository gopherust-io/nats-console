# 5-node NATS JetStream cluster lab

Local insecure lab for stream `Replicas: 5` and the Consol **Replicas** page.

```bash
make nats-cluster-up
# or:
docker compose -f docker/nats/cluster/docker-compose.yml up -d
```

| | |
|--|--|
| JetStream domain | `hub` |
| Client ports | `4222–4226` |
| Monitoring | `8222–8226` |
| Image | `nats:2.14` |

**Stop** the root compose `nats` service first (`docker compose stop nats`) — it binds the same host ports.

Point Consol at:

```bash
NATS_URL=nats://127.0.0.1:4222,nats://127.0.0.1:4223,nats://127.0.0.1:4224,nats://127.0.0.1:4225,nats://127.0.0.1:4226
NATS_MONITORING_URL=http://127.0.0.1:8222
```

From Docker (console container → host monitors): use `host.docker.internal` instead of `127.0.0.1`.

Tear down (wipes JetStream volumes):

```bash
make nats-cluster-down
```

Not for production (no TLS / auth). See also [docs/local-docker.md](../../../docs/local-docker.md).
