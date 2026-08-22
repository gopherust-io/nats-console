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
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ListStreams godoc
//
// @Summary List Streams
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.StreamListEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams [get]
func (h *Handler) ListStreams(ctx *fasthttp.RequestCtx) {
	offset, limit := apikit.ParsePaginationParams(ctx, h.Cfg)
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		streams, total, err := client.ListStreams(c, offset, limit)
		if err != nil {
			return nil, 0, err
		}
		return apikit.StreamsPage(streams, total, offset, limit), fasthttp.StatusOK, nil
	})
}

// GetStream godoc
//
// @Summary Get Stream
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Produce json
// @Success 200 {object} api.StreamInfoEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name} [get]
func (h *Handler) GetStream(ctx *fasthttp.RequestCtx) {
	name := httpctx.RouteParam(ctx, "name")
	if err := apikit.ValidateResourceName(name); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.StreamInfo(c, name)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return domain.StreamInfoFromNATS(info), fasthttp.StatusOK, nil
	})
}

// CreateStream godoc
//
// @Summary Create Stream
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 201 {object} api.StreamInfoEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams [post]
func (h *Handler) CreateStream(ctx *fasthttp.RequestCtx) {
	var req streamConfigRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.Name) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, apikit.ErrMissing("name"))
		return
	}
	if err := apikit.ValidateResourceName(req.Name); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	cfg, err := req.toNATS()
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.AddStream(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		h.InvalidateSnapshot(ctx)
		return domain.StreamInfoFromNATS(info), fasthttp.StatusCreated, nil
	})
}

// UpdateStream godoc
//
// @Summary Update Stream
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Produce json
// @Success 200 {object} api.StreamInfoEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name} [put]
func (h *Handler) UpdateStream(ctx *fasthttp.RequestCtx) {
	var req streamConfigRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.Name) {
		req.Name = httpctx.RouteParam(ctx, "name")
	}
	cfg, err := req.toNATS()
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.UpdateStream(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		h.InvalidateSnapshot(ctx)
		return domain.StreamInfoFromNATS(info), fasthttp.StatusOK, nil
	})
}

// DeleteStream godoc
//
// @Summary Delete Stream
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name} [delete]
func (h *Handler) DeleteStream(ctx *fasthttp.RequestCtx) {
	h.Void(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		if err := client.DeleteStream(c, httpctx.RouteParam(ctx, "name")); err != nil {
			return err
		}
		h.InvalidateSnapshot(ctx)
		return nil
	}, fasthttp.StatusBadRequest)
}

// PurgeStream godoc
//
// @Summary Purge Stream
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/purge [post]
func (h *Handler) PurgeStream(ctx *fasthttp.RequestCtx) {
	h.Void(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		if err := client.PurgeStream(c, httpctx.RouteParam(ctx, "name")); err != nil {
			return err
		}
		h.InvalidateSnapshot(ctx)
		return nil
	}, fasthttp.StatusBadRequest)
}
