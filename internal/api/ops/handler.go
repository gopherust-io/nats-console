// Package ops hosts the operator-facing endpoints: cluster CRUD, the
// connection-status SSE feed, the health/OpenAPI surface, and the pprof
// passthrough.
package ops

import (
	"github.com/valyala/fasthttp"

	openapispec "github.com/gopherust-io/nats-consol/api"
	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
)

type Handler struct {
	*apikit.Core
}

func NewHandler(svc *app.Services, cfg config.Config, hub *snapshot.Hub) *Handler {
	return &Handler{Core: apikit.NewCore(svc, cfg, hub)}
}

// Health godoc
//
// @Summary Health
// @Tags System
// @Produce json
// @Success 200 {object} api.HealthEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Router /api/health [get]
func (h *Handler) Health(ctx *fasthttp.RequestCtx) {
	status, code := h.Svc.Health.Check(httpctx.FromRequest(ctx))
	httpstatus.WriteData(ctx, code, status)
}

// OpenAPI godoc
//
// @Summary Open API
// @Tags System
// @Produce application/yaml
// @Success 200 {string} string "YAML OpenAPI document"
// @Router /api/openapi.yaml [get]
func (h *Handler) OpenAPI(ctx *fasthttp.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/yaml")
	ctx.SetBody(openapispec.OpenAPISpec)
}
