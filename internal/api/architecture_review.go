package api

import (
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ArchitectureReview returns deterministic event-architecture findings from topology jsz.
func (h *Handler) ArchitectureReview(ctx *fasthttp.RequestCtx) {
	snap, err := h.loadArchitectureReview(ctx)
	if err != nil {
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, snap)
}

type architectureReviewAskRequest struct {
	Message string `json:"message"`
}

type architectureReviewAskResponse struct {
	Reply    string                             `json:"reply"`
	Snapshot domain.EventArchitectureSnapshot   `json:"snapshot"`
}

// ArchitectureReviewAsk uses Gemini to narrate a precomputed architecture review.
func (h *Handler) ArchitectureReviewAsk(ctx *fasthttp.RequestCtx) {
	if h.svc == nil || h.svc.Assistant == nil || !h.svc.Assistant.Enabled() {
		writeAssistantError(ctx, assistant.WrapError(assistant.ErrNotEnabled))
		return
	}

	snap, err := h.loadArchitectureReview(ctx)
	if err != nil {
		return
	}

	var req architectureReviewAskRequest
	if len(ctx.PostBody()) > 0 {
		if uerr := serializer.Unmarshal(ctx.PostBody(), &req); uerr != nil {
			writeAssistantError(ctx, assistant.WrapError(uerr))
			return
		}
	}

	reply, aerr := h.svc.Assistant.ArchitectureReview(
		httpctx.FromRequest(ctx),
		snap,
		req.Message,
	)
	if aerr != nil {
		writeAssistantError(ctx, assistant.WrapError(aerr))
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, architectureReviewAskResponse{
		Reply:    reply,
		Snapshot: snap,
	})
}

func (h *Handler) loadArchitectureReview(ctx *fasthttp.RequestCtx) (domain.EventArchitectureSnapshot, error) {
	clusterID := clusterID(ctx)
	fresh := strings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"

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
			return domain.EventArchitectureSnapshot{}, err
		}
		data, err := client.Monitoring(c, snapshot.TopologyJSZPath)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
			return domain.EventArchitectureSnapshot{}, err
		}
		raw = data
		capturedAt = time.Now().UTC()
		if int64(len(raw)) > h.cfg.MaxMonitoringBodyBytes {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, errMonitoringTooLarge)
			return domain.EventArchitectureSnapshot{}, errMonitoringTooLarge
		}
	}

	projected := projectJSZForTopology(raw)
	inputs, err := architectureReviewInputsFromJSZ(projected)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return domain.EventArchitectureSnapshot{}, err
	}
	snap := domain.AnalyzeEventArchitecture(inputs)
	if !capturedAt.IsZero() {
		snap.CapturedAt = capturedAt
	} else {
		snap.CapturedAt = time.Now().UTC()
	}
	return snap, nil
}

func architectureReviewInputsFromJSZ(raw []byte) ([]domain.EventArchitectureInput, error) {
	return natsclient.ExtractEventArchitectureInputs(raw)
}
