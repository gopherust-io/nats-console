# Manual test checklist

Use after a deploy or upgrade. For automated API coverage against a running stack, prefer `make test-smoke` (CI also runs this on every PR).

## Prerequisites

- Console reachable (compose, Helm, or local `make run`)
- Admin credentials (default local: `admin` / `admin`)

## UI smoke (5–10 minutes)

1. **Login** — Sign in; land on Systems; brand and shell chrome visible.
2. **Systems** — Open the default system; open account **Default**.
3. **JetStream** — Open JetStream hub; create a short-lived stream (or open an existing one).
4. **Publish** — On stream detail, publish a small JSON message; confirm it appears in the message browser (or state updates).
5. **Live tail** — Open Live Tail; confirm the page connects (badge / waiting state is acceptable if idle).
6. **Admin alerts** — Open Alerts from the user menu; list loads (empty is OK).
7. **Sign out** — Sign out returns to Sign In.

## API smoke (optional)

```bash
docker compose up --build -d   # if local
make test-smoke                # health, login, clusters, streams, live WS, OpenAPI
```

## Load / stress (optional, needs [vegeta](https://github.com/tsenart/vegeta))

```bash
make test-performance   # ~20 RPS, 10s
make test-stress        # ~100 RPS, 30s
```
