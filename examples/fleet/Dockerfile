# syntax=docker/dockerfile:1
# Build from the nats-consol module root:
#   docker build -f examples/fleet/Dockerfile -t nats-fleet:local .
FROM golang:1.27.0-bookworm@sha256:484ef6066fa69acb059fdfeda7ba2b8f7391f2ef6abc6f9b8411e669ebd56466 AS build
WORKDIR /src
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /fleet ./examples/fleet

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /fleet /fleet
USER nonroot:nonroot
ENTRYPOINT ["/fleet"]
ENV SERVICE=all
ENV TEL_ENABLE=false
