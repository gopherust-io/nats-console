package api

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"

	"github.com/bytedance/sonic"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/nats-io/nats.go"
	"github.com/valyala/fasthttp"
)

type Handler struct {
	store *store.Store
	nats  *natsclient.Manager
}

func NewHandler(st *store.Store, nats *natsclient.Manager) *Handler {
	return &Handler{store: st, nats: nats}
}

func (h *Handler) Health(ctx *fasthttp.RequestCtx) {
	writeJSON(ctx, fasthttp.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) AccountInfo(ctx *fasthttp.RequestCtx) {
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		info, err := client.AccountInfo(c)
		return info, fasthttp.StatusOK, err
	})
}

func (h *Handler) ListStreams(ctx *fasthttp.RequestCtx) {
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		names, err := client.StreamNames(c)
		if err != nil {
			return nil, 0, err
		}
		streams := make([]*nats.StreamInfo, 0, len(names))
		for _, name := range names {
			info, err := client.StreamInfo(c, name)
			if err != nil {
				return nil, 0, err
			}
			streams = append(streams, info)
		}
		return map[string]any{"streams": streams, "total": len(streams)}, fasthttp.StatusOK, nil
	})
}

func (h *Handler) GetStream(ctx *fasthttp.RequestCtx) {
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		info, err := client.StreamInfo(c, param(ctx, "name"))
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) CreateStream(ctx *fasthttp.RequestCtx) {
	var cfg nats.StreamConfig
	if err := sonic.Unmarshal(ctx.PostBody(), &cfg); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if cfg.Name == "" {
		writeError(ctx, fasthttp.StatusBadRequest, errMissing("name"))
		return
	}
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		info, err := client.AddStream(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusCreated, nil
	})
}

func (h *Handler) UpdateStream(ctx *fasthttp.RequestCtx) {
	var cfg nats.StreamConfig
	if err := sonic.Unmarshal(ctx.PostBody(), &cfg); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if cfg.Name == "" {
		cfg.Name = param(ctx, "name")
	}
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		info, err := client.UpdateStream(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) DeleteStream(ctx *fasthttp.RequestCtx) {
	h.natsVoid(ctx, func(c context.Context, client *natsclient.Client) error {
		return client.DeleteStream(c, param(ctx, "name"))
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) PurgeStream(ctx *fasthttp.RequestCtx) {
	h.natsVoid(ctx, func(c context.Context, client *natsclient.Client) error {
		return client.PurgeStream(c, param(ctx, "name"))
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) ListConsumers(ctx *fasthttp.RequestCtx) {
	stream := param(ctx, "name")
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		names, err := client.ConsumerNames(c, stream)
		if err != nil {
			return nil, 0, err
		}
		consumers := make([]*nats.ConsumerInfo, 0, len(names))
		for _, name := range names {
			info, err := client.ConsumerInfo(c, stream, name)
			if err != nil {
				return nil, 0, err
			}
			consumers = append(consumers, info)
		}
		return map[string]any{"consumers": consumers, "total": len(consumers)}, fasthttp.StatusOK, nil
	})
}

func (h *Handler) GetConsumer(ctx *fasthttp.RequestCtx) {
	stream := param(ctx, "name")
	consumer := param(ctx, "consumer")
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		info, err := client.ConsumerInfo(c, stream, consumer)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) CreateConsumer(ctx *fasthttp.RequestCtx) {
	stream := param(ctx, "name")
	var cfg nats.ConsumerConfig
	if err := sonic.Unmarshal(ctx.PostBody(), &cfg); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		info, err := client.AddConsumer(c, stream, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusCreated, nil
	})
}

func (h *Handler) DeleteConsumer(ctx *fasthttp.RequestCtx) {
	stream := param(ctx, "name")
	consumer := param(ctx, "consumer")
	h.natsVoid(ctx, func(c context.Context, client *natsclient.Client) error {
		return client.DeleteConsumer(c, stream, consumer)
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) GetMessage(ctx *fasthttp.RequestCtx) {
	stream := param(ctx, "name")
	seqStr := string(ctx.QueryArgs().Peek("seq"))
	if seqStr == "" {
		writeError(ctx, fasthttp.StatusBadRequest, errMissing("seq"))
		return
	}
	seq, err := strconv.ParseUint(seqStr, 10, 64)
	if err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	direction := string(ctx.QueryArgs().Peek("direction"))

	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
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
		result, err := client.GetMessageNav(c, stream, seq, "")
		if err != nil {
			return map[string]any{"message": msg}, fasthttp.StatusOK, nil
		}
		return result, fasthttp.StatusOK, nil
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
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		buckets, err := client.ListKVBuckets(c)
		if err != nil {
			return nil, 0, err
		}
		return map[string]any{"buckets": buckets, "total": len(buckets)}, fasthttp.StatusOK, nil
	})
}

func (h *Handler) CreateKVBucket(ctx *fasthttp.RequestCtx) {
	var cfg nats.KeyValueConfig
	if err := sonic.Unmarshal(ctx.PostBody(), &cfg); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if cfg.Bucket == "" {
		writeError(ctx, fasthttp.StatusBadRequest, errMissing("bucket"))
		return
	}
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		info, err := client.CreateKVBucket(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusCreated, nil
	})
}

func (h *Handler) GetKVBucket(ctx *fasthttp.RequestCtx) {
	bucket := param(ctx, "bucket")
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		info, err := client.GetKVBucket(c, bucket)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) DeleteKVBucket(ctx *fasthttp.RequestCtx) {
	bucket := param(ctx, "bucket")
	h.natsVoid(ctx, func(c context.Context, client *natsclient.Client) error {
		return client.DeleteKVBucket(c, bucket)
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) ListKVKeys(ctx *fasthttp.RequestCtx) {
	bucket := param(ctx, "bucket")
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		keys, err := client.ListKVKeys(c, bucket)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return map[string]any{"keys": keys, "total": len(keys)}, fasthttp.StatusOK, nil
	})
}

func (h *Handler) GetKVEntry(ctx *fasthttp.RequestCtx) {
	bucket := param(ctx, "bucket")
	key := param(ctx, "key")
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
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
	bucket := param(ctx, "bucket")
	key := param(ctx, "key")
	var req kvPutRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	value, err := base64.StdEncoding.DecodeString(req.Value)
	if err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		entry, err := client.PutKVEntry(c, bucket, key, value)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return entry, fasthttp.StatusOK, nil
	})
}

func (h *Handler) DeleteKVEntry(ctx *fasthttp.RequestCtx) {
	bucket := param(ctx, "bucket")
	key := param(ctx, "key")
	h.natsVoid(ctx, func(c context.Context, client *natsclient.Client) error {
		return client.DeleteKVEntry(c, bucket, key)
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) KVHistory(ctx *fasthttp.RequestCtx) {
	bucket := param(ctx, "bucket")
	key := param(ctx, "key")
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		entries, err := client.KVHistory(c, bucket, key)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return map[string]any{"entries": entries, "total": len(entries)}, fasthttp.StatusOK, nil
	})
}

func (h *Handler) ListObjectBuckets(ctx *fasthttp.RequestCtx) {
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		buckets, err := client.ListObjectBuckets(c)
		if err != nil {
			return nil, 0, err
		}
		return map[string]any{"buckets": buckets, "total": len(buckets)}, fasthttp.StatusOK, nil
	})
}

func (h *Handler) CreateObjectBucket(ctx *fasthttp.RequestCtx) {
	var cfg nats.ObjectStoreConfig
	if err := sonic.Unmarshal(ctx.PostBody(), &cfg); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if cfg.Bucket == "" {
		writeError(ctx, fasthttp.StatusBadRequest, errMissing("bucket"))
		return
	}
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		info, err := client.CreateObjectBucket(c, &cfg)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusCreated, nil
	})
}

func (h *Handler) GetObjectBucket(ctx *fasthttp.RequestCtx) {
	bucket := param(ctx, "bucket")
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		info, err := client.GetObjectBucket(c, bucket)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) DeleteObjectBucket(ctx *fasthttp.RequestCtx) {
	bucket := param(ctx, "bucket")
	h.natsVoid(ctx, func(c context.Context, client *natsclient.Client) error {
		return client.DeleteObjectBucket(c, bucket)
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) ListObjects(ctx *fasthttp.RequestCtx) {
	bucket := param(ctx, "bucket")
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		objects, err := client.ListObjects(c, bucket)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		return map[string]any{"objects": objects, "total": len(objects)}, fasthttp.StatusOK, nil
	})
}

func (h *Handler) GetObject(ctx *fasthttp.RequestCtx) {
	bucket := param(ctx, "bucket")
	name := param(ctx, "objectName")
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
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
	bucket := param(ctx, "bucket")
	name := param(ctx, "objectName")
	var req objectPutRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.natsAction(ctx, func(c context.Context, client *natsclient.Client) (any, int, error) {
		info, err := client.PutObject(c, bucket, name, data)
		if err != nil {
			return nil, fasthttp.StatusBadRequest, err
		}
		return info, fasthttp.StatusOK, nil
	})
}

func (h *Handler) DeleteObject(ctx *fasthttp.RequestCtx) {
	bucket := param(ctx, "bucket")
	name := param(ctx, "objectName")
	h.natsVoid(ctx, func(c context.Context, client *natsclient.Client) error {
		return client.DeleteObject(c, bucket, name)
	}, fasthttp.StatusBadRequest)
}

func (h *Handler) natsAction(ctx *fasthttp.RequestCtx, fn func(context.Context, *natsclient.Client) (any, int, error)) {
	client, err := h.nats.Get(requestContext(ctx), clusterID(ctx))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeStoreError(ctx, err)
			return
		}
		writeError(ctx, fasthttp.StatusBadGateway, err)
		return
	}

	result, status, err := fn(requestContext(ctx), client)
	if err != nil {
		if status == 0 {
			status = fasthttp.StatusBadGateway
		}
		writeError(ctx, status, err)
		return
	}
	if status == 0 {
		status = fasthttp.StatusOK
	}
	writeJSON(ctx, status, result)
}

func (h *Handler) natsVoid(ctx *fasthttp.RequestCtx, fn func(context.Context, *natsclient.Client) error, badStatus int) {
	client, err := h.nats.Get(requestContext(ctx), clusterID(ctx))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeStoreError(ctx, err)
			return
		}
		writeError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	if err := fn(requestContext(ctx), client); err != nil {
		writeError(ctx, badStatus, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *Handler) natsRaw(ctx *fasthttp.RequestCtx, path string) {
	client, err := h.nats.Get(requestContext(ctx), clusterID(ctx))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeStoreError(ctx, err)
			return
		}
		writeError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	data, err := client.Monitoring(requestContext(ctx), path)
	if err != nil {
		writeError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	writeRawJSON(ctx, data)
}

func param(ctx *fasthttp.RequestCtx, key string) string {
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

func writeJSON(ctx *fasthttp.RequestCtx, status int, v any) {
	data, err := sonic.Marshal(v)
	if err != nil {
		writeError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	ctx.SetStatusCode(status)
	ctx.SetContentType("application/json")
	ctx.SetBody(data)
}

func writeRawJSON(ctx *fasthttp.RequestCtx, data []byte) {
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	ctx.SetBody(data)
}

func writeError(ctx *fasthttp.RequestCtx, status int, err error) {
	writeJSON(ctx, status, map[string]string{"error": err.Error()})
}

type missingFieldError string

func (e missingFieldError) Error() string {
	return "missing required field: " + string(e)
}

func errMissing(field string) error {
	return missingFieldError(field)
}
