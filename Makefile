.PHONY: align generate openapi tidy \
	dev dev-web dev-web-docker reload-api build run \
	docker-up docker-down \
	nats-up nats-down nats-auth-up nats-cluster-up nats-cluster-down nats-down-all \
	fleet-up fleet-down seed-demo \
	test test-unit test-integration test-contract test-security test-regression \
	test-e2e test-smoke test-performance test-stress test-web ci \
	lint lint-go lint-go-fix lint-web lint-web-docker

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
SWAG_VERSION := v1.16.4
SWAG_BIN := $(HOME)/go/bin/swag
UNIT_PKGS := $(shell go list ./... | grep -v '/tests/integration\|/tests/contract\|/tests/security\|/web/node_modules')
FLEET_COMPOSE ?= examples/fleet/docker-compose.yml
FLEET_NATS_URL ?= nats://nats-cluster-1:4222,nats://nats-cluster-2:4222,nats://nats-cluster-3:4222,nats://nats-cluster-4:4222,nats://nats-cluster-5:4222
FLEET_STREAM_REPLICAS ?= 1

$(GOALIGN_BIN):
	go install github.com/gopherust-io/goalign@$(GOALIGN_VERSION)

$(SWAG_BIN):
	go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)

align: $(GOALIGN_BIN)
	$(GOALIGN_BIN) $(GOALIGN_FLAGS)

# Generate api/swagger.yaml from swag annotations (cmd + internal/api handlers).
# -g is relative to the first -d directory (./cmd).
openapi: $(SWAG_BIN)
	$(SWAG_BIN) init \
		-g main.go \
		-d ./cmd,./internal/api,./internal/app,./internal/domain,./internal/live \
		--parseInternal \
		-o ./api \
		--ot yaml

generate:
	go generate ./...
	$(MAKE) openapi

tidy:
	go mod tidy

dev:
	go run ./cmd

dev-web:
	cd $(WEB_DIR) && npm install && npm run dev

dev-web-docker:
	-docker rm -f nats-consol-web-dev 2>/dev/null || true
	CONSOLE_PORT=8081 NODE_IMAGE=$(NODE_IMAGE) docker compose --profile web up -d --build --force-recreate console web-dev

reload-api:
	./scripts/reload-api.sh

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
	curl --retry 30 --retry-delay 1 --retry-connrefused -sf $(NATS_HEALTHZ) >/dev/null
	@echo "NATS ready: nats://127.0.0.1:4222  monitor $(NATS_HEALTHZ)"

nats-down:
	docker compose -f $(NATS_COMPOSE) down

nats-auth-up:
	docker compose -f $(NATS_AUTH_COMPOSE) up -d
	@echo "Auth lab up. Users: orders-pub/pubpass, orders-worker/workerpass, js-admin/adminpass"

nats-cluster-up:
	docker compose stop nats
	docker compose --profile cluster up -d $(CLUSTER_SERVICES)
	@echo "Cluster lab up (4222-4226 / 8222-8226). Restore single: docker compose up -d nats"

nats-cluster-down:
	docker compose -p nats-consol -f $(NATS_CLUSTER_COMPOSE) --profile cluster down -v
	@echo "Cluster removed. Restore single: docker compose up -d nats"

nats-down-all:
	-docker compose -f $(NATS_COMPOSE) down -v
	-docker compose -p nats-consol -f $(NATS_CLUSTER_COMPOSE) --profile cluster down -v
	-docker compose -f $(NATS_AUTH_COMPOSE) down -v
	-docker compose -f $(NATS_SUPERCLUSTER_COMPOSE) down -v

fleet-up:
	NATS_URL=$(FLEET_NATS_URL) STREAM_REPLICAS=$(FLEET_STREAM_REPLICAS) \
		docker compose -p nats-consol -f $(FLEET_COMPOSE) --profile fleet up -d --build --force-recreate

fleet-down:
	docker compose -p nats-consol -f $(FLEET_COMPOSE) --profile fleet down

seed-demo:
	./scripts/seed-demo-topology.sh

test: test-unit

test-unit:
	go test $(UNIT_PKGS) -count=1 -p $(shell nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)

test-integration test-contract test-security:
	go test -tags=integration ./tests/$(patsubst test-%,%,$@)/... -count=1

test-regression: test-integration test-contract test-security

test-e2e test-smoke:
	./tests/e2e/smoke.sh

test-performance:
	./tests/performance/load.sh

test-stress:
	./tests/performance/stress.sh

test-web:
	cd $(WEB_DIR) && npm ci && npm test && npm run build && npx playwright install --with-deps chromium && npm run test:e2e

ci: lint-go lint-web test-unit test-regression

lint: lint-go lint-web

lint-go: align
	golangci-lint run ./...

lint-go-fix: align
	golangci-lint run ./... --fix
	@fieldalignment -fix ./... 2>/dev/null || "$(HOME)/go/bin/fieldalignment" -fix ./... 2>/dev/null || true

lint-web:
	@if command -v npm >/dev/null 2>&1; then \
		cd $(WEB_DIR) && npm install && npm run lint && npm run typecheck && npm test; \
	else \
		$(MAKE) lint-web-docker; \
	fi

lint-web-docker:
	docker run --rm -v "$(CURDIR)/$(WEB_DIR):/web" -w /web $(NODE_IMAGE) \
		sh -c "npm install && npm run lint && npm run typecheck && npm test"
