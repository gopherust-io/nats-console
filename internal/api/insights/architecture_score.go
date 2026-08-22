package insights

import (
	"errors"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
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

// ArchitectureScoreDemo godoc
//
// @Summary Architecture Score Demo
// @Tags Docs
// @Produce json
// @Success 200 {object} api.ArchitectureScoreEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Router /api/v1/architecture-score/demo [get]
func (h *Handler) ArchitectureScoreDemo(ctx *fasthttp.RequestCtx) {
	httpstatus.WriteData(ctx, fasthttp.StatusOK, domain.DemoArchitectureScoreSnapshot())
}

// ArchitectureScore godoc
//
// @Summary Architecture Score
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.ArchitectureScoreEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/architecture-score [get]
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
	Reply    string                           `json:"reply"`
	Snapshot domain.ArchitectureScoreSnapshot `json:"snapshot"`
}

// ArchitectureScoreAsk godoc
//
// @Summary Architecture Score Ask
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.DataMetaEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/architecture-score/ask [post]
func (h *Handler) ArchitectureScoreAsk(ctx *fasthttp.RequestCtx) {
	if h.Svc == nil || h.Svc.Assistant == nil || !h.Svc.Assistant.Enabled() {
		apikit.WriteAssistantError(ctx, assistant.WrapError(assistant.ErrNotEnabled))
		return
	}
	snap, err := h.loadArchitectureScore(ctx)
	if err != nil {
		return
	}
	var req architectureScoreAskRequest
	if len(ctx.PostBody()) > 0 {
		if uerr := serializer.Unmarshal(ctx.PostBody(), &req); uerr != nil {
			apikit.WriteAssistantError(ctx, assistant.WrapError(uerr))
			return
		}
	}
	reply, aerr := h.Svc.Assistant.ArchitectureScore(httpctx.FromRequest(ctx), snap, req.Message)
	if aerr != nil {
		apikit.WriteAssistantError(ctx, assistant.WrapError(aerr))
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, architectureScoreAskResponse{Reply: reply, Snapshot: snap})
}

func (h *Handler) loadArchitectureScore(ctx *fasthttp.RequestCtx) (domain.ArchitectureScoreSnapshot, error) {
	demo := strings.BytesToString(ctx.QueryArgs().Peek("demo")) == "1"
	if demo {
		return domain.DemoArchitectureScoreSnapshot(), nil
	}

	clusterID := apikit.ClusterID(ctx)
	fresh := strings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"

	var raw []byte
	var capturedAt time.Time
	if !fresh && h.Hub != nil {
		if data, at, ok := h.Hub.MonitoringPayload(clusterID, snapshot.TopologyJSZPath); ok {
			raw = data
			capturedAt = at
		}
	}
	if raw == nil {
		c := httpctx.FromRequest(ctx)
		client, err := h.Svc.JetStream.GetExecutor(c, clusterID)
		if err != nil {
			apikit.WriteAPIError(ctx, err)
			return domain.ArchitectureScoreSnapshot{}, err
		}
		data, err := client.Monitoring(c, snapshot.TopologyJSZPath)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
			return domain.ArchitectureScoreSnapshot{}, err
		}
		raw = data
		capturedAt = time.Now().UTC()
		if int64(len(raw)) > h.Cfg.MaxMonitoringBodyBytes {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, apikit.ErrMonitoringTooLarge)
			return domain.ArchitectureScoreSnapshot{}, apikit.ErrMonitoringTooLarge
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
	if h.Svc != nil && h.Svc.ArchScore != nil {
		if prior, ok, perr := h.Svc.ArchScore.PriorDay(httpctx.FromRequest(ctx), clusterID, time.Now().UTC()); perr == nil && ok {
			hints.Prior = &prior
		}
	}

	snap := domain.ComputeArchitectureScore(inputs, hints)
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}
	snap.CapturedAt = capturedAt

	if h.Svc != nil && h.Svc.ArchScore != nil {
		rows, lerr := h.Svc.ArchScore.ListHistory(httpctx.FromRequest(ctx), clusterID)
		if lerr != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusInternalServerError, lerr)
			return domain.ArchitectureScoreSnapshot{}, lerr
		}
		snap = domain.AttachArchitectureScoreTrend(snap, rows)
		_ = h.Svc.ArchScore.UpsertToday(httpctx.FromRequest(ctx), domain.ArchitectureScoreDailyRow{
			ClusterID:  clusterID,
			ScoreDay:   capturedAt,
			Score:      snap.Score,
			Factors:    snap.Factors,
			AvgLag:     hints.AvgLag,
			CapturedAt: capturedAt,
		})
	} else if h.Svc == nil {
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
