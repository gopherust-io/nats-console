# syntax=docker/dockerfile:1

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist /src/web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/nats-consol ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/nats-consol /app/nats-consol
COPY migrations /app/migrations
ENV HTTP_ADDR=:8080 \
    STATIC_DIR=/app/web \
    AUTH_ENABLED=true \
    ADMIN_USERNAME=admin \
    ADMIN_PASSWORD=admin
COPY --from=web /src/web/dist /app/web
EXPOSE 8080
ENTRYPOINT ["/app/nats-consol"]
