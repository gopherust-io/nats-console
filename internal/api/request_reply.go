package api

import (
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type RequestReplyHandler struct {
	svc *app.Services
	hub *snapshot.Hub
}

func NewRequestReplyHandler(svc *app.Services, hub *snapshot.Hub) *RequestReplyHandler {
	return &RequestReplyHandler{svc: svc, hub: hub}
}

func (h *RequestReplyHandler) Snapshot(ctx *fasthttp.RequestCtx) {
	cluster := clusterID(ctx)
	fresh := strings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"

	var connz []byte
	var capturedAt time.Time
	var probeResults []domain.RequestReplyProbeResult

	if !fresh && h.hub != nil {
		if data, captured, ok := h.hub.MonitoringPayload(cluster, snapshot.RequestReplyConnzPath); ok {
			connz = data
			capturedAt = captured
		}
		if probes, _, ok := h.hub.ProbeResultsOverlay(cluster); ok {
			probeResults = probes
		}
	}

	if len(connz) == 0 {
		c := httpctx.FromRequest(ctx)
		client, err := h.svc.JetStream.GetExecutor(c, cluster)
		if err != nil {
			writeAPIError(ctx, err)
			return
		}
		data, err := client.Monitoring(c, natsclient.RequestReplyConnzPath)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
			return
		}
		connz = data
		if capturedAt.IsZero() {
			capturedAt = time.Now().UTC()
		}
		// Prefer cached probe overlay even on fresh connz fetch.
		if h.hub != nil && len(probeResults) == 0 {
			if probes, _, ok := h.hub.ProbeResultsOverlay(cluster); ok {
				probeResults = probes
			}
		}
	}

	snap := natsclient.BuildRequestReplySnapshot(connz, probeResults)
	if !capturedAt.IsZero() {
		snap.CapturedAt = capturedAt
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, snap)
}
