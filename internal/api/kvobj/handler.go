// Package kvobj serves the JetStream Key/Value and Object Store bounded
// context: bucket lifecycle, entries, and object payloads.
package kvobj

import (
	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
)

type Handler struct {
	*apikit.Core
}

func NewHandler(svc *app.Services, cfg config.Config, hub *snapshot.Hub) *Handler {
	return &Handler{Core: apikit.NewCore(svc, cfg, hub)}
}
