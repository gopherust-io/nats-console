package app

import (
	"context"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type MetricsService struct {
	metrics port.MetricsRepository
}

func NewMetricsService(metrics port.MetricsRepository) *MetricsService {
	return &MetricsService{metrics: metrics}
}

func (s *MetricsService) QuerySeries(
	ctx context.Context,
	clusterID string,
	metricNames []string,
	from, to time.Time,
	step time.Duration,
) (map[string][]domain.MetricPoint, error) {
	return s.metrics.QueryMetricSeries(ctx, clusterID, metricNames, from, to, step)
}
