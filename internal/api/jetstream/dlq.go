package jetstream

import (
	"context"
	"errors"
	"strconv"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ListDLQMessages godoc
//
// @Summary List DLQMessages
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Produce json
// @Success 200 {object} api.DLQListEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/dlq/messages [get]
func (h *Handler) ListDLQMessages(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	if err := apikit.ValidateResourceName(stream); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	args := ctx.QueryArgs()
	var startSeq uint64
	if startSeqStr := commonstrings.BytesToString(args.Peek("startSeq")); !commonstrings.IsEmpty(startSeqStr) {
		n, err := strconv.ParseUint(startSeqStr, 10, 64)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("startSeq must be a non-negative integer"))
			return
		}
		startSeq = n
	}
	limit := 0
	if limitStr := commonstrings.BytesToString(args.Peek("limit")); !commonstrings.IsEmpty(limitStr) {
		n, err := strconv.Atoi(limitStr)
		if err != nil || n < 0 {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("limit must be a non-negative integer"))
			return
		}
		limit = n
	}

	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		result, err := client.ListDLQMessages(c, stream, startSeq, limit)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return result, fasthttp.StatusOK, nil
	})
}

// RetryDLQMessages godoc
//
// @Summary Retry DLQMessages
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Produce json
// @Success 200 {object} api.DLQRetryEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/dlq/retry [post]
func (h *Handler) RetryDLQMessages(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	if err := apikit.ValidateResourceName(stream); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	var req domain.DLQRetryRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := req.Validate(); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		result, err := client.RetryDLQMessages(c, stream, req)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return result, fasthttp.StatusOK, nil
	})
}
