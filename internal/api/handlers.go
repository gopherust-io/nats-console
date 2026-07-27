package api

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"

	"github.com/valyala/fasthttp"
)

type Handler struct {
	svc *app.Services
	hub *snapshot.Hub
	cfg config.Config
}

func NewHandler(svc *app.Services, cfg config.Config, hub *snapshot.Hub) *Handler {
	return &Handler{svc: svc, cfg: cfg, hub: hub}
}

func (h *Handler) Health(ctx *fasthttp.RequestCtx) {
	status, code := h.svc.Health.Check(httpctx.FromRequest(ctx))
	serializer.WriteJSON(ctx, code, status)
}

func (h *Handler) AccountInfo(ctx *fasthttp.RequestCtx) {
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.AccountInfo(c)
		if err != nil {
			return nil, 0, err
		}
		out := domain.AccountInfoFromNATS(info)
		return out, fasthttp.StatusOK, nil
	})
}

func (h *Handler) ListStreams(ctx *fasthttp.RequestCtx) {
	offset, limit := parsePaginationParams(ctx, h.cfg)
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		streams, total, err := client.ListStreams(c, offset, limit)
		if err != nil {
			return nil, 0, err
		}
		return newStreamsListResponse(streams, total, offset, limit), fasthttp.StatusOK, nil
	})
}

func (h *Handler) GetStream(ctx *fasthttp.RequestCtx) {
	name := httpctx.RouteParam(ctx, "name")
	if err := validateResourceName(name); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.StreamInfo(c, name)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return domain.StreamInfoFromNATS(info), fasthttp.StatusOK, nil
	})
}

func (h *Handler) CreateStream(ctx *fasthttp.RequestCtx) {
	var req streamConfigRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("name"))
		return
	}
	if err := validateResourceName(req.Name); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	cfg, err := req.toNATS()
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.AddStream(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		h.invalidateTopologySnapshot(ctx)
		return domain.StreamInfoFromNATS(info), fasthttp.StatusCreated, nil
	})
}

func (h *Handler) UpdateStream(ctx *fasthttp.RequestCtx) {
	var req streamConfigRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		req.Name = httpctx.RouteParam(ctx, "name")
	}
	cfg, err := req.toNATS()
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.UpdateStream(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		h.invalidateTopologySnapshot(ctx)
		return domain.StreamInfoFromNATS(info), fasthttp.StatusOK, nil
	})
}

func (h *Handler) invalidateTopologySnapshot(ctx *fasthttp.RequestCtx) {
	if h.hub == nil {
		return
	}
	h.hub.Invalidate(clusterID(ctx))
}

func (h *Handler) DeleteStream(ctx *fasthttp.RequestCtx) {
	h.natsVoid(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		if err := client.DeleteStream(c, httpctx.RouteParam(ctx, "name")); err != nil {
			return err
		}
		h.invalidateTopologySnapshot(ctx)
		return nil
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) PurgeStream(ctx *fasthttp.RequestCtx) {
	h.natsVoid(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		if err := client.PurgeStream(c, httpctx.RouteParam(ctx, "name")); err != nil {
			return err
		}
		h.invalidateTopologySnapshot(ctx)
		return nil
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) ListConsumers(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	offset, limit := parsePaginationParams(ctx, h.cfg)
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		consumers, total, err := client.ListConsumers(c, stream, offset, limit)
		if err != nil {
			return nil, 0, err
		}
		return newConsumersListResponse(consumers, total, offset, limit), fasthttp.StatusOK, nil
	})
}

func (h *Handler) GetConsumer(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	consumer := httpctx.RouteParam(ctx, "consumer")
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.ConsumerInfo(c, stream, consumer)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return domain.ConsumerInfoFromNATS(info), fasthttp.StatusOK, nil
	})
}

func (h *Handler) CreateConsumer(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	var req consumerConfigRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	cfg, err := req.toNATS()
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.AddConsumer(c, stream, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		h.invalidateTopologySnapshot(ctx)
		return domain.ConsumerInfoFromNATS(info), fasthttp.StatusCreated, nil
	})
}

func (h *Handler) UpdateConsumer(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	consumer := httpctx.RouteParam(ctx, "consumer")
	var req consumerConfigRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.DurableName == "" {
		req.DurableName = consumer
	}
	if req.DurableName != consumer {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("consumer name cannot be changed"))
		return
	}
	cfg, err := req.toNATS()
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.UpdateConsumer(c, stream, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		h.invalidateTopologySnapshot(ctx)
		return domain.ConsumerInfoFromNATS(info), fasthttp.StatusOK, nil
	})
}

func (h *Handler) DeleteConsumer(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	consumer := httpctx.RouteParam(ctx, "consumer")
	h.natsVoid(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		if err := client.DeleteConsumer(c, stream, consumer); err != nil {
			return err
		}
		h.invalidateTopologySnapshot(ctx)
		return nil
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) ReplayConsumer(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	consumer := httpctx.RouteParam(ctx, "consumer")
	if err := validateResourceName(stream); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := validateResourceName(consumer); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	var req domain.ReplayConsumerRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := req.Validate(); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		result, err := client.ReplayConsumer(c, stream, consumer, req)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return result, fasthttp.StatusOK, nil
	})
}

func (h *Handler) GetMessage(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	seqStr := string(ctx.QueryArgs().Peek("seq"))
	if seqStr == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("seq"))
		return
	}
	seq, err := strconv.ParseUint(seqStr, 10, 64)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	direction := string(ctx.QueryArgs().Peek("direction"))

	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		result, err := client.GetMessageNav(c, stream, seq, direction)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return result, fasthttp.StatusOK, nil
	})
}

func (h *Handler) PublishMessage(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	if err := validateResourceName(stream); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	var req domain.PublishMessageRequest
	if err := serializer.UnmarshalRequest(ctx.PostBody(), &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.Data == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("data"))
		return
	}

	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		result, err := client.PublishStreamMessage(c, stream, req)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return result, fasthttp.StatusCreated, nil
	})
}

func (h *Handler) Varz(ctx *fasthttp.RequestCtx) {
	h.natsRaw(ctx, "/varz")
}

func (h *Handler) Jsz(ctx *fasthttp.RequestCtx) {
	path := "/jsz"
	if query := string(ctx.URI().QueryString()); query != "" {
		path += "?" + query
	}
	h.natsRaw(ctx, path)
}

func (h *Handler) ListKVBuckets(ctx *fasthttp.RequestCtx) {
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		buckets, err := client.ListKVBuckets(c)
		if err != nil {
			return nil, 0, err
		}
		return KVBucketsListResponse{Buckets: nonNilSlice(buckets), Total: len(buckets)}, fasthttp.StatusOK, nil
	})
}

func (h *Handler) CreateKVBucket(ctx *fasthttp.RequestCtx) {
	var req kvBucketConfigRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.Bucket == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("bucket"))
		return
	}
	cfg, err := req.toKVConfig()
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	opts := domain.KVBucketWriteOpts{
		LimitMarkerTTLNs: req.LimitMarkerTTLNs,
		Metadata:         req.Metadata,
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.CreateKVBucket(c, &cfg, opts)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusCreated, nil
	})
}

func (h *Handler) UpdateKVBucket(ctx *fasthttp.RequestCtx) {
	var req kvBucketConfigRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	bucket := httpctx.RouteParam(ctx, "bucket")
	if req.Bucket == "" {
		req.Bucket = bucket
	}
	if req.Bucket != bucket {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("bucket name cannot be changed"))
		return
	}
	cfg, err := req.toKVConfig()
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	opts := domain.KVBucketWriteOpts{
		LimitMarkerTTLNs: req.LimitMarkerTTLNs,
		Metadata:         req.Metadata,
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.UpdateKVBucket(c, &cfg, opts)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) GetKVBucket(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.GetKVBucket(c, bucket)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) DeleteKVBucket(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	h.natsVoid(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.DeleteKVBucket(c, bucket)
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) ListKVKeys(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	offset, limit := parsePaginationParams(ctx, h.cfg)
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		keys, total, err := client.ListKVKeys(c, bucket, offset, limit)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return newKeysListResponse(keys, total, offset, limit), fasthttp.StatusOK, nil
	})
}

func (h *Handler) GetKVEntry(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	key := httpctx.RouteParam(ctx, "key")
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
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

func (h *Handler) PutKVEntry(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	key := httpctx.RouteParam(ctx, "key")
	var req kvPutRequest
	if err := serializer.UnmarshalRequest(ctx.PostBody(), &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	value, err := base64.StdEncoding.DecodeString(req.Value)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		entry, err := client.PutKVEntry(c, bucket, key, value)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return entry, fasthttp.StatusOK, nil
	})
}

func (h *Handler) DeleteKVEntry(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	key := httpctx.RouteParam(ctx, "key")
	h.natsVoid(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.DeleteKVEntry(c, bucket, key)
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) KVHistory(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	key := httpctx.RouteParam(ctx, "key")
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		entries, err := client.KVHistory(c, bucket, key)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return KVHistoryResponse{Entries: nonNilSlice(entries), Total: len(entries)}, fasthttp.StatusOK, nil
	})
}

func (h *Handler) ListObjectBuckets(ctx *fasthttp.RequestCtx) {
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		buckets, err := client.ListObjectBuckets(c)
		if err != nil {
			return nil, 0, err
		}
		return ObjectBucketsListResponse{Buckets: nonNilSlice(buckets), Total: len(buckets)}, fasthttp.StatusOK, nil
	})
}

func (h *Handler) CreateObjectBucket(ctx *fasthttp.RequestCtx) {
	var req objectBucketConfigRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.Bucket == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("bucket"))
		return
	}
	cfg, err := req.toObjectConfig()
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.CreateObjectBucket(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusCreated, nil
	})
}

func (h *Handler) UpdateObjectBucket(ctx *fasthttp.RequestCtx) {
	var req objectBucketConfigRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	bucket := httpctx.RouteParam(ctx, "bucket")
	if req.Bucket == "" {
		req.Bucket = bucket
	}
	if req.Bucket != bucket {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("bucket name cannot be changed"))
		return
	}
	cfg, err := req.toObjectConfig()
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.UpdateObjectBucket(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) GetObjectBucket(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.GetObjectBucket(c, bucket)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) DeleteObjectBucket(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	h.natsVoid(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.DeleteObjectBucket(c, bucket)
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) ListObjects(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	offset, limit := parsePaginationParams(ctx, h.cfg)
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		objects, total, err := client.ListObjects(c, bucket, offset, limit)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return newObjectsListResponse(objects, total, offset, limit), fasthttp.StatusOK, nil
	})
}

func (h *Handler) GetObject(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	name := httpctx.RouteParam(ctx, "objectName")
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
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

func (h *Handler) PutObject(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	name := httpctx.RouteParam(ctx, "objectName")
	var req objectPutRequest
	if err := serializer.UnmarshalRequest(ctx.PostBody(), &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.PutObject(c, bucket, name, data)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) DeleteObject(ctx *fasthttp.RequestCtx) {
	bucket := httpctx.RouteParam(ctx, "bucket")
	name := httpctx.RouteParam(ctx, "objectName")
	h.natsVoid(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.DeleteObject(c, bucket, name)
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) natsAction(ctx *fasthttp.RequestCtx, fn func(context.Context, port.JetStreamExecutor) (any, int, error)) {
	c := httpctx.FromRequest(ctx)
	var (
		result any
		status int
		etag   string
	)
	err := h.svc.JetStream.WithExecutor(c, clusterID(ctx), func(client port.JetStreamExecutor) error {
		var actionErr error
		result, status, actionErr = fn(c, client)
		if tagged, ok := client.(interface{ LastETag() string }); ok {
			etag = tagged.LastETag()
		}
		return actionErr
	})
	if err != nil {
		status = mapNATSErrorStatus(err, status)
		if status == fasthttp.StatusNotFound {
			writeDomainError(ctx, domain.ErrNotFound)
			return
		}
		serializer.WriteError(ctx, status, err)
		return
	}
	if status == 0 {
		status = fasthttp.StatusOK
	}
	if etag != "" && serializer.CheckIfNoneMatch(ctx, etag) {
		ctx.SetStatusCode(fasthttp.StatusNotModified)
		return
	}
	serializer.WriteJSONWithETag(ctx, status, result, etag)
}

func (h *Handler) natsVoid(ctx *fasthttp.RequestCtx, fn func(context.Context, port.JetStreamExecutor) error, badStatus int) {
	c := httpctx.FromRequest(ctx)
	err := h.svc.JetStream.WithExecutor(c, clusterID(ctx), func(client port.JetStreamExecutor) error {
		return fn(c, client)
	})
	if err != nil {
		status := mapNATSErrorStatus(err, badStatus)
		if status == fasthttp.StatusNotFound {
			writeDomainError(ctx, domain.ErrNotFound)
			return
		}
		serializer.WriteError(ctx, status, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func mapNATSErrorStatus(err error, requested int) int {
	if err == nil {
		return fasthttp.StatusOK
	}
	if errors.Is(err, domain.ErrNotFound) || isNATSNotFound(err) {
		return fasthttp.StatusNotFound
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return fasthttp.StatusGatewayTimeout
	}
	if requested == fasthttp.StatusNotFound || requested == 0 {
		return fasthttp.StatusBadGateway
	}
	return requested
}

func isNATSNotFound(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no stream") ||
		strings.Contains(msg, "no consumers") ||
		strings.Contains(msg, "no keys found") ||
		strings.Contains(msg, "bucket not found")
}

func (h *Handler) natsRaw(ctx *fasthttp.RequestCtx, path string) {
	c := httpctx.FromRequest(ctx)
	cluster := clusterID(ctx)
	fresh := string(ctx.QueryArgs().Peek("fresh")) == "1"

	if !fresh && h.hub != nil {
		if data, capturedAt, ok := h.hub.MonitoringPayload(cluster, path); ok {
			if int64(len(data)) > h.cfg.MaxMonitoringBytes() {
				serializer.WriteError(ctx, fasthttp.StatusBadGateway, errMonitoringTooLarge)
				return
			}
			etag := `"` + capturedAt.UTC().Format("20060102T150405") + `"`
			if serializer.CheckIfNoneMatch(ctx, etag) {
				ctx.SetStatusCode(fasthttp.StatusNotModified)
				return
			}
			ctx.Response.Header.Set("X-Snapshot-Age", capturedAt.UTC().Format(timeRFC3339))
			serializer.WriteRawJSONWithETag(ctx, data, etag)
			return
		}
		metrics.IncSnapshotHubMiss(path)
	}

	client, err := h.svc.JetStream.GetExecutor(c, cluster)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeDomainError(ctx, err)
			return
		}
		serializer.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	data, err := client.Monitoring(c, path)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	if int64(len(data)) > h.cfg.MaxMonitoringBytes() {
		serializer.WriteError(ctx, fasthttp.StatusBadGateway, errMonitoringTooLarge)
		return
	}
	etag := ""
	if tagged, ok := client.(interface{ LastETag() string }); ok {
		etag = tagged.LastETag()
	}
	if etag != "" && serializer.CheckIfNoneMatch(ctx, etag) {
		ctx.SetStatusCode(fasthttp.StatusNotModified)
		return
	}
	serializer.WriteRawJSONWithETag(ctx, data, etag)
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"

type missingFieldError string

func (e missingFieldError) Error() string {
	return "missing required field: " + string(e)
}

func errMissing(field string) error {
	return missingFieldError(field)
}
