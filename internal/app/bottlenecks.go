package app

import (
	"context"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type BottleneckService struct {
	metrics  port.MetricsRepository
	lookBack time.Duration
}

func NewBottleneckService(metrics port.MetricsRepository, lookBack time.Duration) *BottleneckService {
	return &BottleneckService{metrics: metrics, lookBack: lookBack}
}

func (s *BottleneckService) Discover(ctx context.Context, clusterID string) (domain.HiddenBottleneckSnapshot, error) {
	to := time.Now().UTC()
	from := to.Add(-s.lookBack)
	buckets, err := s.metrics.ListBottleneckHourBuckets(ctx, clusterID, from, to)
	if err != nil {
		return domain.HiddenBottleneckSnapshot{}, err
	}
	snap := domain.DiscoverHiddenBottlenecks(buckets)
	snap.CapturedAt = to
	snap.From = from
	snap.To = to
	if snap.Findings == nil {
		snap.Findings = []domain.HiddenBottleneckFinding{}
	}
	if snap.Suggestions == nil {
		snap.Suggestions = []string{}
	}
	return snap, nil
}
