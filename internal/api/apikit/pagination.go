package apikit

import (
	"strconv"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"

	"github.com/nats-io/nats.go"
	"github.com/valyala/fasthttp"
)

// DataMeta is returned from natsAction list handlers so the write path can
// emit WriteDataMetaWithETag (array in data, pagination in meta).
type DataMeta struct {
	Data any
	Meta *httpstatus.Meta
}

func ParsePaginationParams(ctx *fasthttp.RequestCtx, cfg config.Config) (offset, limit int) {
	offset, _ = strconv.Atoi(strings.BytesToString(ctx.QueryArgs().Peek("offset")))
	limit, _ = strconv.Atoi(strings.BytesToString(ctx.QueryArgs().Peek("limit")))
	limit = cfg.NormalizePaginationLimit(limit)
	if offset < 0 {
		offset = 0
	}
	return offset, limit
}

func PageMeta(total, offset, limit int) *httpstatus.Meta {
	return &httpstatus.Meta{Total: total, Offset: offset, Limit: limit}
}

func TotalMeta(total int) *httpstatus.Meta {
	return &httpstatus.Meta{Total: total}
}

func StreamsPage(streams []*nats.StreamInfo, total, offset, limit int) DataMeta {
	return DataMeta{Data: domain.StreamInfosFromNATS(streams), Meta: PageMeta(total, offset, limit)}
}

func ConsumersPage(consumers []*nats.ConsumerInfo, total, offset, limit int, streamLastSeq uint64, thr domain.SlowConsumerThresholds) DataMeta {
	list := domain.ConsumerInfosFromNATS(consumers)
	for i := range list {
		domain.ApplySlowConsumerFlags(&list[i], streamLastSeq, thr)
	}
	return DataMeta{Data: list, Meta: PageMeta(total, offset, limit)}
}

func KeysPage(keys []string, total, offset, limit int) DataMeta {
	return DataMeta{Data: NonNilSlice(keys), Meta: PageMeta(total, offset, limit)}
}

func ObjectsPage(objects []string, total, offset, limit int) DataMeta {
	return DataMeta{Data: NonNilSlice(objects), Meta: PageMeta(total, offset, limit)}
}
