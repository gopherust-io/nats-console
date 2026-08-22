package insights

import (
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// EventWikipediaHandler serves auto-assembled event Wikipedia pages.
// Read-only: assembles pages from jsz + Postgres docs; never mutates JetStream.
type EventWikipediaHandler struct {
	*apikit.Core
}

// NewEventWikipediaHandler wires catalog + genome assembly from JSZ.
func NewEventWikipediaHandler(svc *app.Services, cfg config.Config, hub *snapshot.Hub) *EventWikipediaHandler {
	return &EventWikipediaHandler{Core: apikit.NewCore(svc, cfg, hub)}
}

// List godoc
//
// @Summary List
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.EventWikipediaEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/event-wikipedia [get]
func (h *EventWikipediaHandler) List(ctx *fasthttp.RequestCtx) {
	clusterID := apikit.ClusterID(ctx)
	fresh := commonstrings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"
	subjectFilter := commonstrings.BytesToString(ctx.QueryArgs().Peek("subject"))

	c := httpctx.FromRequest(ctx)
	raw, capturedAt, err := h.Svc.Monitoring.FetchJSZ(c, clusterID, fresh)
	if err != nil {
		apikit.WriteJSZFetchError(ctx, err)
		return
	}

	live, err := h.Svc.Monitoring.EventCatalogLiveFromJSZ(raw)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	genomeInputs, err := h.Svc.Monitoring.EventGenomeInputsFromJSZ(raw)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	docs, err := h.Svc.EventCatalog.ListDocs(httpctx.FromRequest(ctx), clusterID)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}

	catalog := domain.BuildEventCatalog(live, docs)
	if !capturedAt.IsZero() {
		catalog.CapturedAt = capturedAt
	} else {
		catalog.CapturedAt = time.Now().UTC()
	}
	genome := domain.AnalyzeEventGenome(genomeInputs)
	genome.CapturedAt = catalog.CapturedAt

	pages := domain.BuildEventWikipedia(catalog, genome)
	pages = domain.FilterEventWikipediaPages(pages, subjectFilter)
	if pages.Pages == nil {
		pages.Pages = []domain.EventWikipediaPage{}
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, pages)
}
