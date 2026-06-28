.PHONY: dev dev-web build run docker-up docker-down tidy generate \
	test test-unit test-integration test-contract test-security test-regression \
	test-e2e test-smoke test-performance lint lint-go lint-go-fix lint-web lint-web-docker lint-web-local

NODE_IMAGE ?= node:22-alpine
WEB_DIR := web

# Packages for unit tests (exclude tagged integration suites and vendored paths).
UNIT_PKGS := $(shell go list ./... | grep -v '/tests/integration' | grep -v '/tests/contract' | grep -v '/tests/security' | grep -v '/web/node_modules')

generate:
	go generate ./...

dev:
	go run ./cmd/server

dev-web:
	cd $(WEB_DIR) && npm install && npm run dev

build:
	cd $(WEB_DIR) && npm install && npm run build
	go build -o bin/nats-consol ./cmd/server

run: build
	STATIC_DIR=web/dist DATABASE_URL=postgres://natsconsol:natsconsol@localhost:5432/natsconsol?sslmode=disable \
		NATS_URL=nats://localhost:4222 NATS_MONITORING_URL=http://localhost:8222 ./bin/nats-consol

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

seed-demo:
	chmod +x scripts/seed-demo-topology.sh
	./scripts/seed-demo-topology.sh

tidy:
	go mod tidy

test: test-unit

test-unit:
	go test $(UNIT_PKGS) -count=1

test-integration:
	go test -tags=integration ./tests/integration/... -count=1 -v

test-contract:
	go test -tags=integration ./tests/contract/... -count=1 -v

test-security:
	go test -tags=integration ./tests/security/... -count=1 -v

test-regression: test-integration test-contract test-security

test-e2e test-smoke:
	./tests/e2e/smoke.sh

test-performance:
	./tests/performance/load.sh

lint: lint-go lint-web

lint-go:
	golangci-lint run ./...

lint-go-fix:
	golangci-lint run ./... --fix
	@if command -v fieldalignment >/dev/null 2>&1; then \
		fieldalignment -fix ./...; \
	elif [ -x "$(HOME)/go/bin/fieldalignment" ]; then \
		"$(HOME)/go/bin/fieldalignment" -fix ./...; \
	fi

lint-web:
	@if command -v npm >/dev/null 2>&1; then \
		cd $(WEB_DIR) && npm install && npm run lint && npm run typecheck; \
	else \
		$(MAKE) lint-web-docker; \
	fi

lint-web-docker:
	docker run --rm -v "$(CURDIR)/$(WEB_DIR):/web" -w /web $(NODE_IMAGE) sh -c "npm install && npm run lint && npm run typecheck"

lint-web-local:
	cd $(WEB_DIR) && npm install && npm run lint && npm run typecheck
