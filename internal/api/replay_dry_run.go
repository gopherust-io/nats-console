package api

import (
	"context"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
)

// ReplayConsumerDryRun returns a preview of replay impact without mutating JetStream.
func (h *Handler) ReplayConsumerDryRun(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	consumer := httpctx.RouteParam(ctx, "consumer")
	if err := validateResourceName(stream); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := validateResourceName(consumer); err != nil {
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

	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
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
