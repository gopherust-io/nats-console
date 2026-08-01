package api

import (
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// EventWikipediaHandler serves auto-assembled event Wikipedia pages.
// Read-only: assembles pages from jsz + Postgres docs; never mutates JetStream.
type EventWikipediaHandler struct {
	svc                   *app.Services
	hub                   *snapshot.Hub
	cfgMaxMonitoringBytes int64
}

// NewEventWikipediaHandler wires catalog + genome assembly from JSZ.
func NewEventWikipediaHandler(svc *app.Services, hub *snapshot.Hub, maxMonitoringBytes int64) *EventWikipediaHandler {
	return &EventWikipediaHandler{svc: svc, hub: hub, cfgMaxMonitoringBytes: maxMonitoringBytes}
}

// List returns auto-assembled Wikipedia pages for concrete subjects.
func (h *EventWikipediaHandler) List(ctx *fasthttp.RequestCtx) {
	clusterID := clusterID(ctx)
	fresh := commonstrings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"
	subjectFilter := commonstrings.BytesToString(ctx.QueryArgs().Peek("subject"))

	var raw []byte
	var capturedAt time.Time
	if !fresh && h.hub != nil {
		if data, at, ok := h.hub.MonitoringPayload(clusterID, snapshot.TopologyJSZPath); ok {
			raw = data
			capturedAt = at
		}
	}
	if raw == nil {
		c := httpctx.FromRequest(ctx)
		client, err := h.svc.JetStream.GetExecutor(c, clusterID)
		if err != nil {
			writeAPIError(ctx, err)
			return
		}
		data, err := client.Monitoring(c, snapshot.TopologyJSZPath)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
			return
		}
		raw = data
		capturedAt = time.Now().UTC()
		if h.cfgMaxMonitoringBytes > 0 && int64(len(raw)) > h.cfgMaxMonitoringBytes {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, errMonitoringTooLarge)
			return
		}
	}

	projected := projectJSZForTopology(raw)
	live, err := eventCatalogLiveFromJSZ(projected)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	genomeInputs, err := eventGenomeInputsFromJSZ(projected)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	docs, err := h.svc.EventCatalog.ListDocs(httpctx.FromRequest(ctx), clusterID)
	if err != nil {
		writeAPIError(ctx, err)
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

	snap := domain.BuildEventWikipedia(catalog, genome)
	snap = domain.FilterEventWikipediaPages(snap, subjectFilter)
	if snap.Pages == nil {
		snap.Pages = []domain.EventWikipediaPage{}
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, snap)
}
