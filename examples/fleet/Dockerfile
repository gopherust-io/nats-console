# syntax=docker/dockerfile:1
# Build from the nats-consol module root (sibling gopherust-io/nats for go.mod replace):
#   docker build -f examples/fleet/Dockerfile --build-context nats=../nats -t nats-fleet:local .
FROM golang:1.26.5-bookworm AS build
WORKDIR /src
ENV GOTOOLCHAIN=auto
COPY --from=nats / /nats
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /fleet ./examples/fleet

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /fleet /fleet
USER nonroot:nonroot
ENTRYPOINT ["/fleet"]
ENV SERVICE=all
ENV TEL_ENABLE=false
