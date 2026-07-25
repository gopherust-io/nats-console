# syntax=docker/dockerfile:1

FROM node:22-alpine@sha256:16e22a550f3863206a3f701448c45f7912c6896a62de43add43bb9c86130c3e2 AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist /src/web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/nats-consol ./cmd/server

FROM alpine:3.21@sha256:48b0309ca019d89d40f670aa1bc06e426dc0931948452e8491e3d65087abc07d
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
