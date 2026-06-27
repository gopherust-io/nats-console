.PHONY: dev dev-web build run docker-up docker-down tidy generate test test-integration lint

generate:
	go generate ./...

dev:
	go run ./cmd/server

dev-web:
	cd web && npm install && npm run dev

build:
	cd web && npm install && npm run build
	go build -o bin/nats-consol ./cmd/server

run: build
	STATIC_DIR=web/dist DATABASE_URL=postgres://natsconsol:natsconsol@localhost:5432/natsconsol?sslmode=disable \
		NATS_URL=nats://localhost:4222 NATS_MONITORING_URL=http://localhost:8222 ./bin/nats-consol

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

tidy:
	go mod tidy

test:
	go test ./...

test-integration:
	go test ./internal/api/... -count=1 -v

lint:
	golangci-lint run ./...
