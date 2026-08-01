package api

import (
	"errors"
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

var errArchitectureScoreUnavailable = errors.New("architecture score service unavailable")

// ArchitectureScoreDemo returns a sample score card (no cluster required).
func (h *Handler) ArchitectureScoreDemo(ctx *fasthttp.RequestCtx) {
	httpstatus.WriteData(ctx, fasthttp.StatusOK, domain.DemoArchitectureScoreSnapshot())
}

// ArchitectureScore returns a live 0–100 architecture score with factors and trend.
func (h *Handler) ArchitectureScore(ctx *fasthttp.RequestCtx) {
	snap, err := h.loadArchitectureScore(ctx)
	if err != nil {
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, snap)
}

type architectureScoreAskRequest struct {
	Message string `json:"message"`
}

type architectureScoreAskResponse struct {
	Reply    string                          `json:"reply"`
	Snapshot domain.ArchitectureScoreSnapshot `json:"snapshot"`
}

// ArchitectureScoreAsk uses Gemini to narrate a precomputed architecture score.
func (h *Handler) ArchitectureScoreAsk(ctx *fasthttp.RequestCtx) {
	if h.svc == nil || h.svc.Assistant == nil || !h.svc.Assistant.Enabled() {
		writeAssistantError(ctx, assistant.WrapError(assistant.ErrNotEnabled))
		return
	}
	snap, err := h.loadArchitectureScore(ctx)
	if err != nil {
		return
	}
	var req architectureScoreAskRequest
	if len(ctx.PostBody()) > 0 {
		if uerr := serializer.Unmarshal(ctx.PostBody(), &req); uerr != nil {
			writeAssistantError(ctx, assistant.WrapError(uerr))
			return
		}
	}
	reply, aerr := h.svc.Assistant.ArchitectureScore(httpctx.FromRequest(ctx), snap, req.Message)
	if aerr != nil {
		writeAssistantError(ctx, assistant.WrapError(aerr))
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, architectureScoreAskResponse{Reply: reply, Snapshot: snap})
}

func (h *Handler) loadArchitectureScore(ctx *fasthttp.RequestCtx) (domain.ArchitectureScoreSnapshot, error) {
	demo := strings.BytesToString(ctx.QueryArgs().Peek("demo")) == "1"
	if demo {
		return domain.DemoArchitectureScoreSnapshot(), nil
	}

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
			return domain.ArchitectureScoreSnapshot{}, err
		}
		data, err := client.Monitoring(c, snapshot.TopologyJSZPath)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
			return domain.ArchitectureScoreSnapshot{}, err
		}
		raw = data
		capturedAt = time.Now().UTC()
		if int64(len(raw)) > h.cfg.MaxMonitoringBodyBytes {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, errMonitoringTooLarge)
			return domain.ArchitectureScoreSnapshot{}, errMonitoringTooLarge
		}
	}

	projected := projectJSZForTopology(raw)
	inputs, err := natsclient.ExtractEventArchitectureInputs(projected)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return domain.ArchitectureScoreSnapshot{}, err
	}

	hints := domain.ArchitectureScoreHints{}
	if samples, serr := natsclient.ExtractIncidentConsumerSamples(projected); serr == nil {
		if avg, ok := domain.AverageConsumerLag(samples); ok {
			hints.AvgLag = avg
			hints.HasLag = true
		}
	}
	if h.svc != nil && h.svc.ArchScore != nil {
		if prior, ok, perr := h.svc.ArchScore.PriorDay(httpctx.FromRequest(ctx), clusterID, time.Now().UTC()); perr == nil && ok {
			hints.Prior = &prior
		}
	}

	snap := domain.ComputeArchitectureScore(inputs, hints)
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}
	snap.CapturedAt = capturedAt

	if h.svc != nil && h.svc.ArchScore != nil {
		rows, lerr := h.svc.ArchScore.ListHistory(httpctx.FromRequest(ctx), clusterID)
		if lerr != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusInternalServerError, lerr)
			return domain.ArchitectureScoreSnapshot{}, lerr
		}
		snap = domain.AttachArchitectureScoreTrend(snap, rows)
		_ = h.svc.ArchScore.UpsertToday(httpctx.FromRequest(ctx), domain.ArchitectureScoreDailyRow{
			ClusterID:  clusterID,
			ScoreDay:   capturedAt,
			Score:      snap.Score,
			Factors:    snap.Factors,
			AvgLag:     hints.AvgLag,
			CapturedAt: capturedAt,
		})
	} else if h.svc == nil {
		httpstatus.WriteError(ctx, fasthttp.StatusServiceUnavailable, errArchitectureScoreUnavailable)
		return domain.ArchitectureScoreSnapshot{}, errArchitectureScoreUnavailable
	}

	if snap.Factors == nil {
		snap.Factors = []domain.ArchitectureScoreFactor{}
	}
	if snap.Trend == nil {
		snap.Trend = []domain.ArchitectureScoreTrendPoint{}
	}
	return snap, nil
}
