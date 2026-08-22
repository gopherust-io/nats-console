package jetstream

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// CaptureIncidentCapsule godoc
//
// @Summary Capture Incident Capsule
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Produce json
// @Success 201 {object} api.IncidentCapsuleEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/incident-capsules [post]
func (h *Handler) CaptureIncidentCapsule(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	if err := apikit.ValidateResourceName(stream); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	var req domain.IncidentCapsuleCaptureRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := req.Validate(); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		detail, err := client.CaptureIncidentCapsule(c, stream, req)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return detail, fasthttp.StatusCreated, nil
	})
}

// CaptureIncidentCapsuleFromDLQ godoc
//
// @Summary Capture Incident Capsule From DLQ
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Param seq path string true "seq"
// @Produce json
// @Success 201 {object} api.IncidentCapsuleEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/dlq/messages/{seq}/capsule [post]
func (h *Handler) CaptureIncidentCapsuleFromDLQ(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	if err := apikit.ValidateResourceName(stream); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	seqStr := httpctx.RouteParam(ctx, "seq")
	seq, err := strconv.ParseUint(seqStr, 10, 64)
	if err != nil || seq == 0 {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("seq must be a positive integer"))
		return
	}

	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		detail, err := client.CaptureIncidentCapsuleFromDLQ(c, stream, seq)
		if err != nil {
			if errors.Is(err, domain.ErrNotDLQStream) {
				return nil, fasthttp.StatusBadRequest, err
			}
			return nil, fasthttp.StatusBadRequest, err
		}
		return detail, fasthttp.StatusCreated, nil
	})
}

// ListIncidentCapsules godoc
//
// @Summary List Incident Capsules
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Param consumer path string true "consumer"
// @Produce json
// @Success 200 {object} api.IncidentCapsuleListEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/consumers/{consumer}/incident-capsules [get]
func (h *Handler) ListIncidentCapsules(ctx *fasthttp.RequestCtx) {
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

	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		list, err := client.ListIncidentCapsules(c, stream, consumer)
		if err != nil {
			return nil, fasthttp.StatusBadGateway, err
		}
		if list == nil {
			list = []domain.IncidentCapsuleSummary{}
		}
		return list, fasthttp.StatusOK, nil
	})
}

// GetIncidentCapsule godoc
//
// @Summary Get Incident Capsule
// @Tags API
// @Param clusterId path string true "clusterId"
// @Param id path string true "id"
// @Produce json
// @Success 200 {object} api.IncidentCapsuleEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/incident-capsules/{id} [get]
func (h *Handler) GetIncidentCapsule(ctx *fasthttp.RequestCtx) {
	id := strings.TrimSpace(httpctx.RouteParam(ctx, "id"))
	if commonstrings.IsEmpty(id) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, domain.ErrCapsuleIDRequired)
		return
	}
	bucket := strings.TrimSpace(commonstrings.BytesToString(ctx.QueryArgs().Peek("bucket")))

	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		detail, err := client.LoadIncidentCapsule(c, id, bucket)
		if err != nil {
			if apikit.IsNATSNotFound(err) || errors.Is(err, domain.ErrNotFound) {
				return nil, fasthttp.StatusNotFound, err
			}
			return nil, fasthttp.StatusBadGateway, err
		}
		return detail, fasthttp.StatusOK, nil
	})
}

// ReplayIncidentCapsuleDryRun godoc
//
// @Summary Replay Incident Capsule Dry Run
// @Tags API
// @Param clusterId path string true "clusterId"
// @Param id path string true "id"
// @Produce json
// @Success 200 {object} api.IncidentCapsuleDryRunEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/incident-capsules/{id}/replay/dry-run [post]
func (h *Handler) ReplayIncidentCapsuleDryRun(ctx *fasthttp.RequestCtx) {
	id := strings.TrimSpace(httpctx.RouteParam(ctx, "id"))
	if commonstrings.IsEmpty(id) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, domain.ErrCapsuleIDRequired)
		return
	}
	bucket := strings.TrimSpace(commonstrings.BytesToString(ctx.QueryArgs().Peek("bucket")))

	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		preview, err := client.PreviewIncidentCapsule(c, id, bucket)
		if err != nil {
			if apikit.IsNATSNotFound(err) || errors.Is(err, domain.ErrNotFound) {
				return nil, fasthttp.StatusNotFound, err
			}
			return nil, fasthttp.StatusBadGateway, err
		}
		return preview, fasthttp.StatusOK, nil
	})
}
