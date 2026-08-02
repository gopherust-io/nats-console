.PHONY: dev dev-web dev-web-docker reload-front build run docker-up docker-down \
	nats-up nats-down nats-auth-up nats-cluster-up nats-cluster-down nats-down-all tidy generate \
	fleet-up fleet-down \
	test test-unit test-integration test-contract test-security test-regression \
	test-e2e test-smoke test-performance test-stress test-web ci lint lint-go lint-go-fix lint-web lint-web-docker lint-web-local align reload-api

NATS_COMPOSE := docker/nats/single/docker-compose.yml
NATS_CLUSTER_COMPOSE := docker/nats/cluster/docker-compose.yml
NATS_AUTH_COMPOSE := docker/nats/auth/docker-compose.yml
NATS_SUPERCLUSTER_COMPOSE := docker/nats/supercluster/docker-compose.yml
NATS_HEALTHZ := http://127.0.0.1:8222/healthz
CLUSTER_SERVICES := nats-1 nats-2 nats-3 nats-4 nats-5

NODE_IMAGE ?= node:22-alpine
WEB_DIR := web
GOALIGN_VERSION := v1.4.0
GOALIGN_BIN := $(HOME)/go/bin/goalign
GOALIGN_FLAGS := analyze -r --arch=amd64 --fail-on-findings --min-waste=1 -e web/,bin/,node_modules/ .

$(GOALIGN_BIN):
	go install github.com/gopherust-io/goalign@$(GOALIGN_VERSION)

align: $(GOALIGN_BIN)
	$(GOALIGN_BIN) $(GOALIGN_FLAGS)

# Packages for unit tests (exclude tagged integration suites and vendored paths).
UNIT_PKGS := $(shell go list ./... | grep -v '/tests/integration' | grep -v '/tests/contract' | grep -v '/tests/security' | grep -v '/web/node_modules')

generate:
	go generate ./...

dev:
	go run ./cmd

dev-web:
	cd $(WEB_DIR) && npm install && npm run dev

dev-web-docker:
	-docker rm -f nats-consol-web-dev 2>/dev/null || true
	CONSOLE_PORT=8081 NODE_IMAGE=$(NODE_IMAGE) docker compose --profile web up -d --build --force-recreate console web-dev

reload-front: dev-web-docker

# Rebuild the Linux API binary and hot-swap it into the running compose console
# (useful when Dockerfile build can't resolve the local nats replace).
# Migrations are copied too: the server reads them from disk at startup, so a
# binary with new endpoints would otherwise run against a stale schema.
reload-api:
	CGO_ENABLED=0 GOOS=linux GOARCH=$$(docker exec nats-consol-console-1 uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') \
		go build -o bin/nats-consol-linux ./cmd
	docker cp bin/nats-consol-linux nats-consol-console-1:/app/nats-consol
	docker exec nats-consol-console-1 sh -c 'rm -f /app/migrations/*.sql'
	docker cp migrations/. nats-consol-console-1:/app/migrations/
	docker restart nats-consol-console-1

build:
	cd $(WEB_DIR) && npm install && npm run build
	go build -o bin/nats-consol ./cmd

run: build
	STATIC_DIR=web/dist DATABASE_URL=postgres://natsconsol:natsconsol@localhost:5432/natsconsol?sslmode=disable \
		NATS_URL=nats://localhost:4222 NATS_MONITORING_URL=http://localhost:8222 ./bin/nats-consol

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

nats-up:
	docker compose -f $(NATS_COMPOSE) up -d
	@echo "Waiting for JetStream healthz..."
	@i=0; \
	while [ $$i -lt 30 ]; do \
		if curl -sf $(NATS_HEALTHZ) >/dev/null 2>&1; then \
			echo "NATS ready: nats://127.0.0.1:4222  monitor $(NATS_HEALTHZ)"; \
			exit 0; \
		fi; \
		i=$$((i+1)); \
		sleep 1; \
	done; \
	echo "Timed out waiting for $(NATS_HEALTHZ)"; \
	exit 1

nats-down:
	docker compose -f $(NATS_COMPOSE) down

nats-auth-up:
	docker compose -f $(NATS_AUTH_COMPOSE) up -d
	@echo "Auth lab up. Users: orders-pub/pubpass, orders-worker/workerpass, js-admin/adminpass"

nats-cluster-up:
	docker compose stop nats
	docker compose --profile cluster up -d $(CLUSTER_SERVICES)
	@echo "Cluster lab up (ports 4222-4226 / 8222-8226) in project nats-consol. See docker/nats/cluster/README.md"
	@echo "Restart single broker later with: docker compose up -d nats"

nats-cluster-down:
	docker compose -p nats-consol -f $(NATS_CLUSTER_COMPOSE) --profile cluster down -v
	@echo "Cluster nodes removed. Restart single broker with: docker compose up -d nats"

nats-down-all:
	-docker compose -f $(NATS_COMPOSE) down -v
	-docker compose -p nats-consol -f $(NATS_CLUSTER_COMPOSE) --profile cluster down -v
	-docker compose -f $(NATS_AUTH_COMPOSE) down -v
	-docker compose -f $(NATS_SUPERCLUSTER_COMPOSE) down -v

seed-demo:
	chmod +x scripts/seed-demo-topology.sh
	./scripts/seed-demo-topology.sh

# Fleet profile on root compose (uses published gopherust-io/nats; optional sibling for docker build-context).
FLEET_COMPOSE ?= examples/fleet/docker-compose.yml
FLEET_NATS_URL ?= nats://nats:4222

fleet-up:
	@test -d ../nats || (echo "missing sibling ../nats (docker build-context for examples/fleet image)" >&2; exit 1)
	NATS_URL=$(FLEET_NATS_URL) docker compose -p nats-consol -f $(FLEET_COMPOSE) --profile fleet up -d --build

fleet-down:
	docker compose -p nats-consol -f $(FLEET_COMPOSE) --profile fleet down

tidy:
	go mod tidy

test: test-unit

test-unit:
	go test $(UNIT_PKGS) -count=1 -p $(shell nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)

test-integration:
	go test -tags=integration ./tests/integration/... -count=1

test-contract:
	go test -tags=integration ./tests/contract/... -count=1

test-security:
	go test -tags=integration ./tests/security/... -count=1

test-regression: test-integration test-contract test-security

test-e2e test-smoke:
	./tests/e2e/smoke.sh

test-web:
	cd $(WEB_DIR) && npm ci && npm test && npm run build && npx playwright install --with-deps chromium && npm run test:e2e

test-performance:
	./tests/performance/load.sh

test-stress:
	./tests/performance/stress.sh

# Targets run on every pull request in GitHub Actions (.github/workflows/test.yml).
# Smoke needs a running compose stack (CI starts it); run `make test-smoke` separately locally.
# HTTPS/HTTP/3: BASE_URL=https://localhost HTTP3_INSECURE=1 make test-smoke
ci: lint-go lint-web test-unit test-regression

lint: lint-go lint-web

lint-go: align
	golangci-lint run ./...

lint-go-fix: align
	golangci-lint run ./... --fix
	@if command -v fieldalignment >/dev/null 2>&1; then \
		fieldalignment -fix ./...; \
	elif [ -x "$(HOME)/go/bin/fieldalignment" ]; then \
		"$(HOME)/go/bin/fieldalignment" -fix ./...; \
	fi

lint-web:
	@if command -v npm >/dev/null 2>&1; then \
		cd $(WEB_DIR) && npm install && npm run lint && npm run typecheck && npm test; \
	else \
		$(MAKE) lint-web-docker; \
	fi

lint-web-docker:
	docker run --rm -v "$(CURDIR)/$(WEB_DIR):/web" -w /web $(NODE_IMAGE) sh -c "npm install && npm run lint && npm run typecheck && npm test"

lint-web-local:
	cd $(WEB_DIR) && npm install && npm run lint && npm run typecheck && npm test
