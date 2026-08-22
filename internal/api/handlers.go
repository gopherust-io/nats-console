package api

import (
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
)

// Handler serves the endpoints that are not tied to a NATS bounded context:
// auth, users, access, admin, assistant, alerts, audit, metrics history, and
// request/reply. The JetStream, KV/object, NATS account, insight, and ops
// endpoints live in the sibling packages of the same name.
type Handler struct {
	*apikit.Core
}

func NewHandler(svc *app.Services, cfg config.Config, hub *snapshot.Hub) *Handler {
	return &Handler{Core: apikit.NewCore(svc, cfg, hub)}
}

// OpenAPIModels godoc
//
// @Summary Schema catalog
// @Description Returns an empty catalog whose fields seed the Swagger Models section with shared DTOs.
// @Tags System
// @Produce json
// @Success 200 {object} ModelCatalog
// @Router /api/v1/schemas [get]
func (h *Handler) OpenAPIModels(ctx *fasthttp.RequestCtx) {
	httpstatus.WriteData(ctx, fasthttp.StatusOK, ModelCatalog{})
}
