# Contributing

## Prerequisites

- Go version from [`go.mod`](go.mod)
- Node.js 22+ for `web/`
- `make` helpers (optional)

## Development

```bash
make test-unit        # Go unit tests
make openapi          # Regenerate api/swagger.yaml from swag annotations
make lint-go          # golangci-lint + goalign
make lint-web         # web lint + typecheck + vitest
make test-web         # web vitest + Playwright e2e
make ci               # lint-go + lint-web + unit + regression
```

## Pull requests

1. Keep changes focused; match existing package and UI style.
2. Run `make lint-go` and `make test-unit` before opening a PR; run web lint when touching `web/`.
3. When changing public APIs: add/update swag annotations on handlers, run `make openapi`, and commit `api/swagger.yaml`.
4. Do not commit secrets.

CI on PRs and `main` runs Go lint/tests (including OpenAPI drift check), web build, and regression suites.

Security reports: see [SECURITY.md](SECURITY.md). Do not open public issues for vulnerabilities.
