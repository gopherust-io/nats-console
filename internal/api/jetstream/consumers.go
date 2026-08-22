package jetstream

import (
	"context"
	"errors"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ListConsumers godoc
//
// @Summary List Consumers
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Produce json
// @Success 200 {object} api.ConsumerListEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/consumers [get]
func (h *Handler) ListConsumers(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	offset, limit := apikit.ParsePaginationParams(ctx, h.Cfg)
	thr := h.slowConsumerThresholds()
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		consumers, total, err := client.ListConsumers(c, stream, offset, limit)
		if err != nil {
			return nil, 0, err
		}
		return apikit.ConsumersPage(consumers, total, offset, limit, streamLastSeq(c, client, stream), thr), fasthttp.StatusOK, nil
	})
}

// GetConsumer godoc
//
// @Summary Get Consumer
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Param consumer path string true "consumer"
// @Produce json
// @Success 200 {object} api.ConsumerInfoEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/consumers/{consumer} [get]
func (h *Handler) GetConsumer(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	consumer := httpctx.RouteParam(ctx, "consumer")
	thr := h.slowConsumerThresholds()
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.ConsumerInfo(c, stream, consumer)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		out := domain.ConsumerInfoFromNATS(info)
		domain.ApplySlowConsumerFlags(&out, streamLastSeq(c, client, stream), thr)
		return out, fasthttp.StatusOK, nil
	})
}

// CreateConsumer godoc
//
// @Summary Create Consumer
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Produce json
// @Success 201 {object} api.ConsumerInfoEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/consumers [post]
func (h *Handler) CreateConsumer(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	var req consumerConfigRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	cfg, err := req.toNATS()
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	thr := h.slowConsumerThresholds()
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.AddConsumer(c, stream, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		h.InvalidateSnapshot(ctx)
		out := domain.ConsumerInfoFromNATS(info)
		domain.ApplySlowConsumerFlags(&out, streamLastSeq(c, client, stream), thr)
		return out, fasthttp.StatusCreated, nil
	})
}

// UpdateConsumer godoc
//
// @Summary Update Consumer
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Param consumer path string true "consumer"
// @Produce json
// @Success 200 {object} api.ConsumerInfoEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/consumers/{consumer} [put]
func (h *Handler) UpdateConsumer(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	consumer := httpctx.RouteParam(ctx, "consumer")
	var req consumerConfigRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.DurableName) {
		req.DurableName = consumer
	}
	if req.DurableName != consumer {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("consumer name cannot be changed"))
		return
	}
	cfg, err := req.toNATS()
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	thr := h.slowConsumerThresholds()
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.UpdateConsumer(c, stream, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		h.InvalidateSnapshot(ctx)
		out := domain.ConsumerInfoFromNATS(info)
		domain.ApplySlowConsumerFlags(&out, streamLastSeq(c, client, stream), thr)
		return out, fasthttp.StatusOK, nil
	})
}

// DeleteConsumer godoc
//
// @Summary Delete Consumer
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Param consumer path string true "consumer"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/consumers/{consumer} [delete]
func (h *Handler) DeleteConsumer(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	consumer := httpctx.RouteParam(ctx, "consumer")
	h.Void(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		if err := client.DeleteConsumer(c, stream, consumer); err != nil {
			return err
		}
		h.InvalidateSnapshot(ctx)
		return nil
	}, fasthttp.StatusBadRequest)
}

// ReplayConsumer godoc
//
// @Summary Replay Consumer
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Param consumer path string true "consumer"
// @Produce json
// @Success 200 {object} api.ReplayEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/consumers/{consumer}/replay [post]
func (h *Handler) ReplayConsumer(ctx *fasthttp.RequestCtx) {
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
		result, err := client.ReplayConsumer(c, stream, consumer, req)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return result, fasthttp.StatusOK, nil
	})
}

// streamLastSeq reports the stream's last sequence for slow-consumer lag math,
// returning 0 when the stream cannot be inspected.
func streamLastSeq(c context.Context, client port.JetStreamExecutor, stream string) uint64 {
	si, err := client.StreamInfo(c, stream)
	if err != nil || si == nil {
		return 0
	}
	return si.State.LastSeq
}
