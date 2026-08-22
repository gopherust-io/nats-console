package kvobj

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ListObjectBuckets godoc
//
// @Summary List Object Buckets
// @Tags Objects
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.ObjectBucketListEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/objects/buckets [get]
func (h *Handler) ListObjectBuckets(ctx *fasthttp.RequestCtx) {
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		buckets, err := client.ListObjectBuckets(c)
		if err != nil {
			return nil, 0, err
		}
		return apikit.DataMeta{Data: apikit.NonNilSlice(buckets), Meta: apikit.TotalMeta(len(buckets))}, fasthttp.StatusOK, nil
	})
}

// CreateObjectBucket godoc
//
// @Summary Create Object Bucket
// @Tags Objects
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 201 {object} api.ObjectBucketEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/objects/buckets [post]
func (h *Handler) CreateObjectBucket(ctx *fasthttp.RequestCtx) {
	var req objectBucketConfigRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.Bucket) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, apikit.ErrMissing("bucket"))
		return
	}
	cfg, err := req.toObjectConfig()
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.CreateObjectBucket(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusCreated, nil
	})
}

// UpdateObjectBucket godoc
//
// @Summary Update Object Bucket
// @Tags Objects
// @Param clusterId path string true "clusterId"
// @Param bucket path string true "bucket"
// @Produce json
// @Success 200 {object} api.ObjectBucketEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/objects/buckets/{bucket} [put]
func (h *Handler) UpdateObjectBucket(ctx *fasthttp.RequestCtx) {
	var req objectBucketConfigRequest
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
	cfg, err := req.toObjectConfig()
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.UpdateObjectBucket(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

// GetObjectBucket godoc
//
// @Summary Get Object Bucket
// @Tags Objects
// @Param clusterId path string true "clusterId"
// @Param bucket path string true "bucket"
// @Produce json
// @Success 200 {object} api.ObjectBucketEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/objects/buckets/{bucket} [get]
func (h *Handler) GetObjectBucket(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.GetObjectBucket(c, bucket)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

// DeleteObjectBucket godoc
//
// @Summary Delete Object Bucket
// @Tags Objects
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
// @Router /api/v1/clusters/{clusterId}/objects/buckets/{bucket} [delete]
func (h *Handler) DeleteObjectBucket(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	h.Void(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.DeleteObjectBucket(c, bucket)
	}, fasthttp.StatusBadRequest)
}

// ListObjects godoc
//
// @Summary List Objects
// @Tags Objects
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
// @Router /api/v1/clusters/{clusterId}/objects/buckets/{bucket}/objects [get]
func (h *Handler) ListObjects(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	offset, limit := apikit.ParsePaginationParams(ctx, h.Cfg)
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		objects, total, err := client.ListObjects(c, bucket, offset, limit)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return apikit.ObjectsPage(objects, total, offset, limit), fasthttp.StatusOK, nil
	})
}

// GetObject godoc
//
// @Summary Get Object
// @Tags Objects
// @Param clusterId path string true "clusterId"
// @Param bucket path string true "bucket"
// @Param objectName path string true "objectName"
// @Produce json
// @Success 200 {object} api.ObjectInfoEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/objects/buckets/{bucket}/objects/{objectName} [get]
func (h *Handler) GetObject(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	name := httpctx.RouteParam(ctx, "objectName")
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.GetObject(c, bucket, name)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

type objectPutRequest struct {
	Data string `json:"data"`
}

// PutObject godoc
//
// @Summary Put Object
// @Tags Objects
// @Param clusterId path string true "clusterId"
// @Param bucket path string true "bucket"
// @Param objectName path string true "objectName"
// @Produce json
// @Success 200 {object} api.ObjectInfoEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/objects/buckets/{bucket}/objects/{objectName} [put]
func (h *Handler) PutObject(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	name := httpctx.RouteParam(ctx, "objectName")
	var req objectPutRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.PutObject(c, bucket, name, data)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

// DeleteObject godoc
//
// @Summary Delete Object
// @Tags Objects
// @Param clusterId path string true "clusterId"
// @Param bucket path string true "bucket"
// @Param objectName path string true "objectName"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/objects/buckets/{bucket}/objects/{objectName} [delete]
func (h *Handler) DeleteObject(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	name := httpctx.RouteParam(ctx, "objectName")
	h.Void(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.DeleteObject(c, bucket, name)
	}, fasthttp.StatusBadRequest)
}
