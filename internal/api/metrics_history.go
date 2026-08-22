package api

import (
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type MetricsHistoryHandler struct {
	metrics *app.MetricsService
}

func NewMetricsHistoryHandler(metrics *app.MetricsService) *MetricsHistoryHandler {
	return &MetricsHistoryHandler{metrics: metrics}
}

// History godoc
//
// @Summary History
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} MetricsHistoryEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/metrics/history [get]
func (h *MetricsHistoryHandler) History(ctx *fasthttp.RequestCtx) {
	c := httpctx.FromRequest(ctx)
	clusterID := apikit.ClusterID(ctx)

	to := time.Now().UTC()
	from := to.Add(-24 * time.Hour)

	if raw := commonstrings.BytesToString(ctx.QueryArgs().Peek("from")); !commonstrings.IsEmpty(raw) {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		from = parsed.UTC()
	}
	if raw := commonstrings.BytesToString(ctx.QueryArgs().Peek("to")); !commonstrings.IsEmpty(raw) {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		to = parsed.UTC()
	}
	if !from.Before(to) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, domain.ErrInvalidRange)
		return
	}

	metrics := parseMetricsQuery(commonstrings.BytesToString(ctx.QueryArgs().Peek("metrics")))
	stepRaw := commonstrings.BytesToString(ctx.QueryArgs().Peek("step"))
	step, err := domain.ParseMetricsStep(stepRaw)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if step <= 0 {
		step = domain.DefaultMetricsStep(from, to)
	}

	seriesMap, err := h.metrics.QuerySeries(c, clusterID, metrics, from, to, step)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}

	series := make([]domain.MetricSeries, 0, len(metrics))
	for _, metric := range metrics {
		points := seriesMap[metric]
		if domain.IsCounterMetric(metric) {
			points = counterRates(points, step)
		}
		if points == nil {
			points = []domain.MetricPoint{}
		}
		series = append(series, domain.MetricSeries{Metric: metric, Points: points})
	}

	httpstatus.WriteData(ctx, fasthttp.StatusOK, domain.MetricsHistoryResponse{
		ClusterID: clusterID,
		From:      from,
		To:        to,
		Series:    series,
	})
}

func parseMetricsQuery(raw string) []string {
	if commonstrings.IsEmpty(strings.TrimSpace(raw)) {
		return append([]string(nil), domain.DefaultDashboardMetrics...)
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if commonstrings.IsEmpty(part) {
			continue
		}
		if !domain.ValidHistoryMetricName(part) {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return append([]string(nil), domain.DefaultDashboardMetrics...)
	}
	return out
}

// counterRates converts monotonic counter buckets into per-second rates.
// Values are (bucket_n - bucket_n-1) / elapsed_seconds so UI labels like "/ s" match the data.
func counterRates(points []domain.MetricPoint, step time.Duration) []domain.MetricPoint {
	if len(points) == 0 {
		return []domain.MetricPoint{}
	}
	if len(points) == 1 {
		return []domain.MetricPoint{{T: points[0].T, V: 0}}
	}
	fallbackSecs := step.Seconds()
	if fallbackSecs <= 0 {
		fallbackSecs = 60
	}
	out := make([]domain.MetricPoint, 0, len(points)-1)
	for i := 1; i < len(points); i++ {
		delta := points[i].V - points[i-1].V
		if delta < 0 {
			delta = 0
		}
		secs := points[i].T.Sub(points[i-1].T).Seconds()
		if secs <= 0 {
			secs = fallbackSecs
		}
		out = append(out, domain.MetricPoint{T: points[i].T, V: delta / secs})
	}
	return out
}
