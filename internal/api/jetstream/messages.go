package jetstream

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// GetMessage godoc
//
// @Summary Get Message
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Produce json
// @Success 200 {object} api.StreamMessageEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/messages [get]
func (h *Handler) GetMessage(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	seqStr := commonstrings.BytesToString(ctx.QueryArgs().Peek("seq"))
	if commonstrings.IsEmpty(seqStr) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, apikit.ErrMissing("seq"))
		return
	}
	seq, err := strconv.ParseUint(seqStr, 10, 64)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	direction := commonstrings.BytesToString(ctx.QueryArgs().Peek("direction"))

	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		result, err := client.GetMessageNav(c, stream, seq, direction)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return result, fasthttp.StatusOK, nil
	})
}

// GetMessageRange godoc
//
// @Summary Get Message Range
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Produce json
// @Success 200 {object} api.MessageRangeEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/messages/range [get]
func (h *Handler) GetMessageRange(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	if err := apikit.ValidateResourceName(stream); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	args := ctx.QueryArgs()
	startSeqStr := commonstrings.BytesToString(args.Peek("startSeq"))
	endSeqStr := commonstrings.BytesToString(args.Peek("endSeq"))
	startTimeStr := commonstrings.BytesToString(args.Peek("startTime"))
	endTimeStr := commonstrings.BytesToString(args.Peek("endTime"))
	limitStr := commonstrings.BytesToString(args.Peek("limit"))

	limit := 0
	if !commonstrings.IsEmpty(limitStr) {
		n, err := strconv.Atoi(limitStr)
		if err != nil || n < 0 {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("limit must be a non-negative integer"))
			return
		}
		limit = n
	}
	if limit == 0 || limit > domain.DefaultMsgRangeMax {
		limit = domain.DefaultMsgRangeMax
	}

	bySeq := !commonstrings.IsEmpty(startSeqStr) || !commonstrings.IsEmpty(endSeqStr)
	byTime := !commonstrings.IsEmpty(startTimeStr) || !commonstrings.IsEmpty(endTimeStr)
	if bySeq == byTime {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("provide either startSeq+endSeq or startTime+endTime"))
		return
	}

	if bySeq {
		if commonstrings.IsEmpty(startSeqStr) || commonstrings.IsEmpty(endSeqStr) {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("startSeq and endSeq are required together"))
			return
		}
		startSeq, err := strconv.ParseUint(startSeqStr, 10, 64)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		endSeq, err := strconv.ParseUint(endSeqStr, 10, 64)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
			result, err := client.GetMessageRange(c, stream, startSeq, endSeq, limit)
			if err != nil {
				return nil, fasthttp.StatusBadRequest, err
			}
			return result, fasthttp.StatusOK, nil
		})
		return
	}

	startTime, err := parseRangeTime(startTimeStr)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, fmt.Errorf("startTime must be RFC3339: %w", err))
		return
	}
	endTime, err := parseRangeTime(endTimeStr)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, fmt.Errorf("endTime must be RFC3339: %w", err))
		return
	}

	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		result, err := client.GetMessageRangeByTime(c, stream, startTime, endTime, limit)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return result, fasthttp.StatusOK, nil
	})
}

// PublishMessage godoc
//
// @Summary Publish Message
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Produce json
// @Success 201 {object} api.PublishMessageEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/messages [post]
func (h *Handler) PublishMessage(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	if err := apikit.ValidateResourceName(stream); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	var req domain.PublishMessageRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.Data) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, apikit.ErrMissing("data"))
		return
	}

	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		result, err := client.PublishStreamMessage(c, stream, req)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return result, fasthttp.StatusCreated, nil
	})
}

func parseRangeTime(raw string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Parse(time.RFC3339, raw)
	}
	return t, nil
}
