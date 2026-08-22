package insights

import (
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

// ArchitectureReview godoc
//
// @Summary Architecture Review
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.EventArchitectureEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/architecture-review [get]
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
	Reply    string                           `json:"reply"`
	Snapshot domain.EventArchitectureSnapshot `json:"snapshot"`
}

// ArchitectureReviewAsk godoc
//
// @Summary Architecture Review Ask
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
// @Router /api/v1/clusters/{clusterId}/architecture-review/ask [post]
func (h *Handler) ArchitectureReviewAsk(ctx *fasthttp.RequestCtx) {
	if h.Svc == nil || h.Svc.Assistant == nil || !h.Svc.Assistant.Enabled() {
		apikit.WriteAssistantError(ctx, assistant.WrapError(assistant.ErrNotEnabled))
		return
	}

	snap, err := h.loadArchitectureReview(ctx)
	if err != nil {
		return
	}

	var req architectureReviewAskRequest
	if len(ctx.PostBody()) > 0 {
		if uerr := serializer.Unmarshal(ctx.PostBody(), &req); uerr != nil {
			apikit.WriteAssistantError(ctx, assistant.WrapError(uerr))
			return
		}
	}

	reply, aerr := h.Svc.Assistant.ArchitectureReview(
		httpctx.FromRequest(ctx),
		snap,
		req.Message,
	)
	if aerr != nil {
		apikit.WriteAssistantError(ctx, assistant.WrapError(aerr))
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, architectureReviewAskResponse{
		Reply:    reply,
		Snapshot: snap,
	})
}

func (h *Handler) loadArchitectureReview(ctx *fasthttp.RequestCtx) (domain.EventArchitectureSnapshot, error) {
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
			return domain.EventArchitectureSnapshot{}, err
		}
		data, err := client.Monitoring(c, snapshot.TopologyJSZPath)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
			return domain.EventArchitectureSnapshot{}, err
		}
		raw = data
		capturedAt = time.Now().UTC()
		if int64(len(raw)) > h.Cfg.MaxMonitoringBodyBytes {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, apikit.ErrMonitoringTooLarge)
			return domain.EventArchitectureSnapshot{}, apikit.ErrMonitoringTooLarge
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
