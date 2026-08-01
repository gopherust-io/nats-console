package api

import (
	"errors"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

var errBottlenecksUnavailable = errors.New("hidden bottlenecks service unavailable")

// HiddenBottlenecks returns deterministic schedule/correlation findings from hourly rollups.
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

// HiddenBottlenecksAsk uses Gemini to narrate a precomputed bottleneck snapshot.
func (h *Handler) HiddenBottlenecksAsk(ctx *fasthttp.RequestCtx) {
	if h.svc == nil || h.svc.Assistant == nil || !h.svc.Assistant.Enabled() {
		writeAssistantError(ctx, assistant.WrapError(assistant.ErrNotEnabled))
		return
	}

	snap, err := h.loadHiddenBottlenecks(ctx)
	if err != nil {
		return
	}

	var req hiddenBottlenecksAskRequest
	if len(ctx.PostBody()) > 0 {
		if uerr := serializer.Unmarshal(ctx.PostBody(), &req); uerr != nil {
			writeAssistantError(ctx, assistant.WrapError(uerr))
			return
		}
	}

	reply, aerr := h.svc.Assistant.HiddenBottlenecks(
		httpctx.FromRequest(ctx),
		snap,
		req.Message,
	)
	if aerr != nil {
		writeAssistantError(ctx, assistant.WrapError(aerr))
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
	if h.svc == nil || h.svc.Bottlenecks == nil {
		httpstatus.WriteError(ctx, fasthttp.StatusServiceUnavailable, errBottlenecksUnavailable)
		return domain.HiddenBottleneckSnapshot{}, errBottlenecksUnavailable
	}
	snap, err := h.svc.Bottlenecks.Discover(httpctx.FromRequest(ctx), clusterID(ctx))
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return domain.HiddenBottleneckSnapshot{}, err
	}
	return snap, nil
}
