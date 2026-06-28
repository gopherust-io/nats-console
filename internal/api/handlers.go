package api

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"

	"github.com/nats-io/nats.go"
	"github.com/valyala/fasthttp"
)

type Handler struct {
	svc *app.Services
	cfg config.Config
}

func NewHandler(svc *app.Services, cfg config.Config) *Handler {
	return &Handler{svc: svc, cfg: cfg}
}

func (h *Handler) Health(ctx *fasthttp.RequestCtx) {
	status, code := h.svc.Health.Check(requestContext(ctx))
	serializer.WriteJSON(ctx, code, status)
}

func (h *Handler) AccountInfo(ctx *fasthttp.RequestCtx) {
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.AccountInfo(c)
		return info, fasthttp.StatusOK, err
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
	name := routeParam(ctx, "name")
	if err := validateResourceName(name); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.StreamInfo(c, name)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) CreateStream(ctx *fasthttp.RequestCtx) {
	var cfg nats.StreamConfig
	if err := serializer.UnmarshalNATSRequest(ctx.PostBody(), &cfg); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if cfg.Name == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("name"))
		return
	}
	if err := validateResourceName(cfg.Name); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.AddStream(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusCreated, nil
	})
}

func (h *Handler) UpdateStream(ctx *fasthttp.RequestCtx) {
	var cfg nats.StreamConfig
	if err := serializer.UnmarshalNATSRequest(ctx.PostBody(), &cfg); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if cfg.Name == "" {
		cfg.Name = routeParam(ctx, "name")
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.UpdateStream(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) DeleteStream(ctx *fasthttp.RequestCtx) {
	h.natsVoid(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.DeleteStream(c, routeParam(ctx, "name"))
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) PurgeStream(ctx *fasthttp.RequestCtx) {
	h.natsVoid(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.PurgeStream(c, routeParam(ctx, "name"))
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) ListConsumers(ctx *fasthttp.RequestCtx) {
	stream := routeParam(ctx, "name")
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
	stream := routeParam(ctx, "name")
	consumer := routeParam(ctx, "consumer")
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.ConsumerInfo(c, stream, consumer)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) CreateConsumer(ctx *fasthttp.RequestCtx) {
	stream := routeParam(ctx, "name")
	var cfg nats.ConsumerConfig
	if err := serializer.UnmarshalNATSRequest(ctx.PostBody(), &cfg); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.AddConsumer(c, stream, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusCreated, nil
	})
}

func (h *Handler) DeleteConsumer(ctx *fasthttp.RequestCtx) {
	stream := routeParam(ctx, "name")
	consumer := routeParam(ctx, "consumer")
	h.natsVoid(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.DeleteConsumer(c, stream, consumer)
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) GetMessage(ctx *fasthttp.RequestCtx) {
	stream := routeParam(ctx, "name")
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
		if direction != "" {
			result, err := client.GetMessageNav(c, stream, seq, direction)
			if err != nil {
				return nil, fasthttp.StatusNotFound, err
			}
			return result, fasthttp.StatusOK, nil
		}
		msg, err := client.GetMessage(c, stream, seq)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return domain.MessageResult{Message: domain.StreamMessageFromRaw(msg)}, fasthttp.StatusOK, nil
	})
}

func (h *Handler) PublishMessage(ctx *fasthttp.RequestCtx) {
	stream := routeParam(ctx, "name")
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
	var cfg nats.KeyValueConfig
	if err := serializer.UnmarshalNATSRequest(ctx.PostBody(), &cfg); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if cfg.Bucket == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("bucket"))
		return
	}
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.CreateKVBucket(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusCreated, nil
	})
}

func (h *Handler) GetKVBucket(ctx *fasthttp.RequestCtx) {
	bucket := routeParam(ctx, "bucket")
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.GetKVBucket(c, bucket)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) DeleteKVBucket(ctx *fasthttp.RequestCtx) {
	bucket := routeParam(ctx, "bucket")
	h.natsVoid(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.DeleteKVBucket(c, bucket)
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) ListKVKeys(ctx *fasthttp.RequestCtx) {
	bucket := routeParam(ctx, "bucket")
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
	bucket := routeParam(ctx, "bucket")
	key := routeParam(ctx, "key")
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
	bucket := routeParam(ctx, "bucket")
	key := routeParam(ctx, "key")
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
	bucket := routeParam(ctx, "bucket")
	key := routeParam(ctx, "key")
	h.natsVoid(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.DeleteKVEntry(c, bucket, key)
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) KVHistory(ctx *fasthttp.RequestCtx) {
	bucket := routeParam(ctx, "bucket")
	key := routeParam(ctx, "key")
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
	var cfg nats.ObjectStoreConfig
	if err := serializer.UnmarshalNATSRequest(ctx.PostBody(), &cfg); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if cfg.Bucket == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("bucket"))
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

func (h *Handler) GetObjectBucket(ctx *fasthttp.RequestCtx) {
	bucket := routeParam(ctx, "bucket")
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.GetObjectBucket(c, bucket)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) DeleteObjectBucket(ctx *fasthttp.RequestCtx) {
	bucket := routeParam(ctx, "bucket")
	h.natsVoid(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.DeleteObjectBucket(c, bucket)
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) ListObjects(ctx *fasthttp.RequestCtx) {
	bucket := routeParam(ctx, "bucket")
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
	bucket := routeParam(ctx, "bucket")
	name := routeParam(ctx, "objectName")
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
	bucket := routeParam(ctx, "bucket")
	name := routeParam(ctx, "objectName")
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
	bucket := routeParam(ctx, "bucket")
	name := routeParam(ctx, "objectName")
	h.natsVoid(ctx, func(c context.Context, client port.JetStreamExecutor) error {
		return client.DeleteObject(c, bucket, name)
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) natsAction(ctx *fasthttp.RequestCtx, fn func(context.Context, port.JetStreamExecutor) (any, int, error)) {
	c := requestContext(ctx)
	var (
		result any
		status int
	)
	err := h.svc.JetStream.WithExecutor(c, clusterID(ctx), func(client port.JetStreamExecutor) error {
		var actionErr error
		result, status, actionErr = fn(c, client)
		return actionErr
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeDomainError(ctx, err)
			return
		}
		if status == 0 {
			status = fasthttp.StatusBadGateway
		}
		serializer.WriteError(ctx, status, err)
		return
	}
	if status == 0 {
		status = fasthttp.StatusOK
	}
	serializer.WriteJSON(ctx, status, result)
}

func (h *Handler) natsVoid(ctx *fasthttp.RequestCtx, fn func(context.Context, port.JetStreamExecutor) error, badStatus int) {
	c := requestContext(ctx)
	err := h.svc.JetStream.WithExecutor(c, clusterID(ctx), func(client port.JetStreamExecutor) error {
		return fn(c, client)
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeDomainError(ctx, err)
			return
		}
		serializer.WriteError(ctx, badStatus, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *Handler) natsRaw(ctx *fasthttp.RequestCtx, path string) {
	c := requestContext(ctx)
	client, err := h.svc.JetStream.GetExecutor(c, clusterID(ctx))
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
	serializer.WriteRawJSON(ctx, data)
}

func routeParam(ctx *fasthttp.RequestCtx, key string) string {
	value, ok := ctx.UserValue(key).(string)
	if !ok {
		return ""
	}
	return value
}

func requestContext(ctx *fasthttp.RequestCtx) context.Context {
	if c, ok := ctx.UserValue("context").(context.Context); ok {
		return c
	}
	return context.Background()
}

type missingFieldError string

func (e missingFieldError) Error() string {
	return "missing required field: " + string(e)
}

func errMissing(field string) error {
	return missingFieldError(field)
}
