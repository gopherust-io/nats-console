package kvobj

import (
	"context"
	"encoding/base64"
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

// ListKVBuckets godoc
//
// @Summary List KVBuckets
// @Tags KV
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.KVBucketListEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/kv/buckets [get]
func (h *Handler) ListKVBuckets(ctx *fasthttp.RequestCtx) {
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		buckets, err := client.ListKVBuckets(c)
		if err != nil {
			return nil, 0, err
		}
		return apikit.DataMeta{Data: apikit.NonNilSlice(buckets), Meta: apikit.TotalMeta(len(buckets))}, fasthttp.StatusOK, nil
	})
}

// CreateKVBucket godoc
//
// @Summary Create KVBucket
// @Tags KV
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 201 {object} api.KVBucketEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/kv/buckets [post]
func (h *Handler) CreateKVBucket(ctx *fasthttp.RequestCtx) {
	var req kvBucketConfigRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.Bucket) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, apikit.ErrMissing("bucket"))
		return
	}
	cfg, err := req.toKVConfig()
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	opts := domain.KVBucketWriteOpts{
		LimitMarkerTTLNs: req.LimitMarkerTTLNs,
		Metadata:         req.Metadata,
	}
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.CreateKVBucket(c, &cfg, opts)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusCreated, nil
	})
}

// UpdateKVBucket godoc
//
// @Summary Update KVBucket
// @Tags KV
// @Param clusterId path string true "clusterId"
// @Param bucket path string true "bucket"
// @Produce json
// @Success 200 {object} api.KVBucketEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/kv/buckets/{bucket} [put]
func (h *Handler) UpdateKVBucket(ctx *fasthttp.RequestCtx) {
	var req kvBucketConfigRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	bucket := httpctx.RouteParam(ctx, "bucket")
	if commonstrings.IsEmpty(req.Bucket) {
		req.Bucket = bucket
	}
	if req.Bucket != bucket {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("bucket name cannot be changed"))
		return
	}
	cfg, err := req.toKVConfig()
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	opts := domain.KVBucketWriteOpts{
		LimitMarkerTTLNs: req.LimitMarkerTTLNs,
		Metadata:         req.Metadata,
	}
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.UpdateKVBucket(c, &cfg, opts)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

// GetKVBucket godoc
//
// @Summary Get KVBucket
// @Tags KV
// @Param clusterId path string true "clusterId"
// @Param bucket path string true "bucket"
// @Produce json
// @Success 200 {object} api.KVBucketEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/kv/buckets/{bucket} [get]
func (h *Handler) GetKVBucket(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.GetKVBucket(c, bucket)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

// DeleteKVBucket godoc
//
// @Summary Delete KVBucket
// @Tags KV
// @Param clusterId path string true "clusterId"
// @Param bucket path string true "bucket"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/kv/buckets/{bucket} [delete]
func (h *Handler) DeleteKVBucket(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	h.Void(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.DeleteKVBucket(c, bucket)
	}, fasthttp.StatusBadRequest)
}

// ListKVKeys godoc
//
// @Summary List KVKeys
// @Tags KV
// @Param clusterId path string true "clusterId"
// @Param bucket path string true "bucket"
// @Produce json
// @Success 200 {object} api.DataMetaEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/kv/buckets/{bucket}/keys [get]
func (h *Handler) ListKVKeys(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	offset, limit := apikit.ParsePaginationParams(ctx, h.Cfg)
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		keys, total, err := client.ListKVKeys(c, bucket, offset, limit)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return apikit.KeysPage(keys, total, offset, limit), fasthttp.StatusOK, nil
	})
}

// GetKVEntry godoc
//
// @Summary Get KVEntry
// @Tags KV
// @Param clusterId path string true "clusterId"
// @Param bucket path string true "bucket"
// @Param key path string true "key"
// @Produce json
// @Success 200 {object} api.KVEntryEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/kv/buckets/{bucket}/keys/{key} [get]
func (h *Handler) GetKVEntry(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	key := httpctx.RouteParam(ctx, "key")
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		entry, err := client.GetKVEntry(c, bucket, key)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return entry, fasthttp.StatusOK, nil
	})
}

type kvPutRequest struct {
	Value string `json:"value"`
}

// PutKVEntry godoc
//
// @Summary Put KVEntry
// @Tags KV
// @Param clusterId path string true "clusterId"
// @Param bucket path string true "bucket"
// @Param key path string true "key"
// @Produce json
// @Success 200 {object} api.KVEntryEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/kv/buckets/{bucket}/keys/{key} [put]
func (h *Handler) PutKVEntry(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	key := httpctx.RouteParam(ctx, "key")
	var req kvPutRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	value, err := base64.StdEncoding.DecodeString(req.Value)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		entry, err := client.PutKVEntry(c, bucket, key, value)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return entry, fasthttp.StatusOK, nil
	})
}

// DeleteKVEntry godoc
//
// @Summary Delete KVEntry
// @Tags KV
// @Param clusterId path string true "clusterId"
// @Param bucket path string true "bucket"
// @Param key path string true "key"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/kv/buckets/{bucket}/keys/{key} [delete]
func (h *Handler) DeleteKVEntry(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	key := httpctx.RouteParam(ctx, "key")
	h.Void(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.DeleteKVEntry(c, bucket, key)
	}, fasthttp.StatusBadRequest)
}

// KVHistory godoc
//
// @Summary KVHistory
// @Tags KV
// @Param clusterId path string true "clusterId"
// @Param bucket path string true "bucket"
// @Param key path string true "key"
// @Produce json
// @Success 200 {object} api.KVEntryListEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/kv/buckets/{bucket}/keys/{key}/history [get]
func (h *Handler) KVHistory(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	key := httpctx.RouteParam(ctx, "key")
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		entries, err := client.KVHistory(c, bucket, key)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return apikit.DataMeta{Data: apikit.NonNilSlice(entries), Meta: apikit.TotalMeta(len(entries))}, fasthttp.StatusOK, nil
	})
}
