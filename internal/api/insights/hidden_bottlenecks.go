package insights

import (
	"errors"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

var errBottlenecksUnavailable = errors.New("hidden bottlenecks service unavailable")

// HiddenBottlenecks godoc
//
// @Summary Hidden Bottlenecks
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.HiddenBottlenecksEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/hidden-bottlenecks [get]
func (h *Handler) HiddenBottlenecks(ctx *fasthttp.RequestCtx) {
	snap, err := h.loadHiddenBottlenecks(ctx)
	if err != nil {
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, snap)
}

type hiddenBottlenecksAskRequest struct {
	Message string `json:"message"`
}

type hiddenBottlenecksAskResponse struct {
	Reply    string                          `json:"reply"`
	Snapshot domain.HiddenBottleneckSnapshot `json:"snapshot"`
}

// HiddenBottlenecksAsk godoc
//
// @Summary Hidden Bottlenecks Ask
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
// @Router /api/v1/clusters/{clusterId}/hidden-bottlenecks/ask [post]
func (h *Handler) HiddenBottlenecksAsk(ctx *fasthttp.RequestCtx) {
	if h.Svc == nil || h.Svc.Assistant == nil || !h.Svc.Assistant.Enabled() {
		apikit.WriteAssistantError(ctx, assistant.WrapError(assistant.ErrNotEnabled))
		return
	}

	snap, err := h.loadHiddenBottlenecks(ctx)
	if err != nil {
		return
	}

	var req hiddenBottlenecksAskRequest
	if len(ctx.PostBody()) > 0 {
		if uerr := serializer.Unmarshal(ctx.PostBody(), &req); uerr != nil {
			apikit.WriteAssistantError(ctx, assistant.WrapError(uerr))
			return
		}
	}

	reply, aerr := h.Svc.Assistant.HiddenBottlenecks(
		httpctx.FromRequest(ctx),
		snap,
		req.Message,
	)
	if aerr != nil {
		apikit.WriteAssistantError(ctx, assistant.WrapError(aerr))
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, hiddenBottlenecksAskResponse{
		Reply:    reply,
		Snapshot: snap,
	})
}

func (h *Handler) loadHiddenBottlenecks(ctx *fasthttp.RequestCtx) (domain.HiddenBottleneckSnapshot, error) {
	demo := commonstrings.BytesToString(ctx.QueryArgs().Peek("demo")) == "1"
	if demo {
		return domain.DemoHiddenBottlenecksSnapshot(), nil
	}
	if h.Svc == nil || h.Svc.Bottlenecks == nil {
		httpstatus.WriteError(ctx, fasthttp.StatusServiceUnavailable, errBottlenecksUnavailable)
		return domain.HiddenBottleneckSnapshot{}, errBottlenecksUnavailable
	}
	snap, err := h.Svc.Bottlenecks.Discover(httpctx.FromRequest(ctx), apikit.ClusterID(ctx))
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return domain.HiddenBottleneckSnapshot{}, err
	}
	return snap, nil
}
