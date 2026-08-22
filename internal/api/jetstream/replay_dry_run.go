package jetstream

import (
	"context"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
)

// ReplayConsumerDryRun godoc
//
// @Summary Replay Consumer Dry Run
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Param consumer path string true "consumer"
// @Produce json
// @Success 200 {object} api.ReplayDryRunEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/consumers/{consumer}/replay/dry-run [post]
func (h *Handler) ReplayConsumerDryRun(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	consumer := httpctx.RouteParam(ctx, "consumer")
	if err := apikit.ValidateResourceName(stream); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := apikit.ValidateResourceName(consumer); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	var req domain.ReplayConsumerRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := req.Validate(); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		streamInfo, err := client.StreamInfo(c, stream)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		consumerInfo, err := client.ConsumerInfo(c, stream, consumer)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		out, err := domain.ComputeReplayDryRun(
			req,
			domain.StreamInfoFromNATS(streamInfo),
			domain.ConsumerInfoFromNATS(consumerInfo),
		)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return out, fasthttp.StatusOK, nil
	})
}
