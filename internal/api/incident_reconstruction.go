package api

import (
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// IncidentReconstructionHandler serves deploy annotations and consumer timelines.
type IncidentReconstructionHandler struct {
	incidents *app.IncidentService
}

func NewIncidentReconstructionHandler(incidents *app.IncidentService) *IncidentReconstructionHandler {
	return &IncidentReconstructionHandler{incidents: incidents}
}

// CreateAnnotation handles POST /api/v1/clusters/{clusterId}/incident-annotations.
func (h *IncidentReconstructionHandler) CreateAnnotation(ctx *fasthttp.RequestCtx) {
	clusterID := clusterID(ctx)
	if err := validateUUID(clusterID); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	var req domain.IncidentAnnotationCreate
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := req.Validate(); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	ann, err := h.incidents.CreateAnnotation(httpctx.FromRequest(ctx), clusterID, req)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusCreated, ann)
}

// GetReconstruction handles GET .../consumers/{consumer}/incident-reconstruction.
func (h *IncidentReconstructionHandler) GetReconstruction(ctx *fasthttp.RequestCtx) {
	clusterID := clusterID(ctx)
	stream := httpctx.RouteParam(ctx, "name")
	consumer := httpctx.RouteParam(ctx, "consumer")
	if err := validateUUID(clusterID); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := validateResourceName(stream); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := validateResourceName(consumer); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	to := time.Now().UTC()
	from := to.Add(-24 * time.Hour)
	if raw := strings.BytesToString(ctx.QueryArgs().Peek("from")); !strings.IsEmpty(raw) {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		from = parsed.UTC()
	}
	if raw := strings.BytesToString(ctx.QueryArgs().Peek("to")); !strings.IsEmpty(raw) {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		to = parsed.UTC()
	}
	if !from.Before(to) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, domain.ErrInvalidRange)
		return
	}

	out, err := h.incidents.Reconstruction(httpctx.FromRequest(ctx), clusterID, stream, consumer, from, to)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, out)
}
