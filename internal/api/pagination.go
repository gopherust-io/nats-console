package api

import (
	"strconv"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"

	"github.com/nats-io/nats.go"
	"github.com/valyala/fasthttp"
)

// dataMeta is returned from natsAction list handlers so the write path can
// emit WriteDataMetaWithETag (array in data, pagination in meta).
type dataMeta struct {
	Data any
	Meta *httpstatus.Meta
}

func parsePaginationParams(ctx *fasthttp.RequestCtx, cfg config.Config) (offset, limit int) {
	offset, _ = strconv.Atoi(strings.BytesToString(ctx.QueryArgs().Peek("offset")))
	limit, _ = strconv.Atoi(strings.BytesToString(ctx.QueryArgs().Peek("limit")))
	limit = cfg.NormalizePaginationLimit(limit)
	if offset < 0 {
		offset = 0
	}
	return offset, limit
}

func pageMeta(total, offset, limit int) *httpstatus.Meta {
	return &httpstatus.Meta{Total: total, Offset: offset, Limit: limit}
}

func totalMeta(total int) *httpstatus.Meta {
	return &httpstatus.Meta{Total: total}
}

func streamsPage(streams []*nats.StreamInfo, total, offset, limit int) dataMeta {
	return dataMeta{Data: domain.StreamInfosFromNATS(streams), Meta: pageMeta(total, offset, limit)}
}

func consumersPage(consumers []*nats.ConsumerInfo, total, offset, limit int, streamLastSeq uint64, thr domain.SlowConsumerThresholds) dataMeta {
	list := domain.ConsumerInfosFromNATS(consumers)
	for i := range list {
		domain.ApplySlowConsumerFlags(&list[i], streamLastSeq, thr)
	}
	return dataMeta{Data: list, Meta: pageMeta(total, offset, limit)}
}

func keysPage(keys []string, total, offset, limit int) dataMeta {
	return dataMeta{Data: nonNilSlice(keys), Meta: pageMeta(total, offset, limit)}
}

func objectsPage(objects []string, total, offset, limit int) dataMeta {
	return dataMeta{Data: nonNilSlice(objects), Meta: pageMeta(total, offset, limit)}
}
