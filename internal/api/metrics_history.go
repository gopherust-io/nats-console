package api

import (
	"strings"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/valyala/fasthttp"
)

type MetricsHistoryHandler struct {
	store *store.Store
}

func NewMetricsHistoryHandler(st *store.Store) *MetricsHistoryHandler {
	return &MetricsHistoryHandler{store: st}
}

func (h *MetricsHistoryHandler) History(ctx *fasthttp.RequestCtx) {
	c := requestContext(ctx)
	clusterID := clusterID(ctx)

	to := time.Now().UTC()
	from := to.Add(-24 * time.Hour)

	if raw := string(ctx.QueryArgs().Peek("from")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		from = parsed.UTC()
	}
	if raw := string(ctx.QueryArgs().Peek("to")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		to = parsed.UTC()
	}
	if !from.Before(to) {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, domain.ErrInvalidRange)
		return
	}

	metrics := parseMetricsQuery(string(ctx.QueryArgs().Peek("metrics")))
	stepRaw := string(ctx.QueryArgs().Peek("step"))
	step, err := store.ParseMetricsStep(stepRaw)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if step <= 0 {
		step = store.DefaultMetricsStep(from, to)
	}

	seriesMap, err := h.store.QueryMetricSeries(c, clusterID, metrics, from, to, step)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}

	series := make([]domain.MetricSeries, 0, len(metrics))
	for _, metric := range metrics {
		points := seriesMap[metric]
		if domain.IsCounterMetric(metric) {
			points = counterDeltas(points)
		}
		if points == nil {
			points = []domain.MetricPoint{}
		}
		series = append(series, domain.MetricSeries{Metric: metric, Points: points})
	}

	serializer.WriteJSON(ctx, fasthttp.StatusOK, domain.MetricsHistoryResponse{
		ClusterID: clusterID,
		From:      from,
		To:        to,
		Series:    series,
	})
}

func parseMetricsQuery(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return append([]string(nil), domain.DefaultDashboardMetrics...)
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !domain.ValidMetricName(part) {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return append([]string(nil), domain.DefaultDashboardMetrics...)
	}
	return out
}

func counterDeltas(points []domain.MetricPoint) []domain.MetricPoint {
	if len(points) == 0 {
		return []domain.MetricPoint{}
	}
	if len(points) == 1 {
		return []domain.MetricPoint{{T: points[0].T, V: 0}}
	}
	out := make([]domain.MetricPoint, 0, len(points)-1)
	for i := 1; i < len(points); i++ {
		delta := points[i].V - points[i-1].V
		if delta < 0 {
			delta = 0
		}
		out = append(out, domain.MetricPoint{T: points[i].T, V: delta})
	}
	return out
}
