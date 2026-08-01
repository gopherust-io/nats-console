# Auth-enabled NATS (local lab)

Minimal JetStream with **username/password** and subject permissions for practicing AuthZ locally.

**Not production TLS** — passwords are plaintext in conf for local drills only.

| User | Password | Role |
|------|----------|------|
| `orders-pub` | `pubpass` | publish `orders.>`, subscribe `_INBOX.>` |
| `orders-worker` | `workerpass` | consume + scoped `$JS.*` |
| `js-admin` | `adminpass` | stream/consumer admin |

```bash
make nats-auth-up
# or:
docker compose -f docker/nats/auth/docker-compose.yml up -d
# client:
#   Address: nats://127.0.0.1:4222
#   User/Password: orders-pub / pubpass  (or worker / admin)
```

Tear down: `docker compose -f docker/nats/auth/docker-compose.yml down -v`

Full guide: [docs/local-docker.md](../../../docs/local-docker.md).
