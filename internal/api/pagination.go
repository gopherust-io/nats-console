package api

import (
	"strconv"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/nats-io/nats.go"
	"github.com/valyala/fasthttp"
)

func parsePaginationParams(ctx *fasthttp.RequestCtx, cfg config.Config) (offset, limit int) {
	offset, _ = strconv.Atoi(string(ctx.QueryArgs().Peek("offset")))
	limit, _ = strconv.Atoi(string(ctx.QueryArgs().Peek("limit")))
	limit = cfg.NormalizePaginationLimit(limit)
	if offset < 0 {
		offset = 0
	}
	return offset, limit
}

func newStreamsListResponse(streams []*nats.StreamInfo, total, offset, limit int) StreamsListResponse {
	meta := paginationMeta{Total: total, Offset: offset, Limit: limit}
	return StreamsListResponse{
		Streams: domain.StreamInfosFromNATS(streams),
		Total:   meta.Total,
		Offset:  meta.Offset,
		Limit:   meta.Limit,
	}
}

func newConsumersListResponse(consumers []*nats.ConsumerInfo, total, offset, limit int) ConsumersListResponse {
	meta := paginationMeta{Total: total, Offset: offset, Limit: limit}
	return ConsumersListResponse{
		Consumers: domain.ConsumerInfosFromNATS(consumers),
		Total:     meta.Total,
		Offset:    meta.Offset,
		Limit:     meta.Limit,
	}
}

func newKeysListResponse(keys []string, total, offset, limit int) KeysListResponse {
	meta := paginationMeta{Total: total, Offset: offset, Limit: limit}
	return KeysListResponse{
		Keys:   nonNilSlice(keys),
		Total:  meta.Total,
		Offset: meta.Offset,
		Limit:  meta.Limit,
	}
}

func newObjectsListResponse(objects []string, total, offset, limit int) ObjectsListResponse {
	meta := paginationMeta{Total: total, Offset: offset, Limit: limit}
	return ObjectsListResponse{
		Objects: nonNilSlice(objects),
		Total:   meta.Total,
		Offset:  meta.Offset,
		Limit:   meta.Limit,
	}
}
