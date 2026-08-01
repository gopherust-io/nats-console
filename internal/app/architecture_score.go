package app

import (
	"context"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
)

const architectureScoreLookback = 180 * 24 * time.Hour

// ArchitectureScoreService loads history and assists live score assembly.
type ArchitectureScoreService struct {
	metrics port.MetricsRepository
}

func NewArchitectureScoreService(metrics port.MetricsRepository) *ArchitectureScoreService {
	return &ArchitectureScoreService{metrics: metrics}
}

func (s *ArchitectureScoreService) ListHistory(ctx context.Context, clusterID string) ([]domain.ArchitectureScoreDailyRow, error) {
	to := time.Now().UTC()
	from := to.Add(-architectureScoreLookback)
	return s.metrics.ListArchitectureScoreDaily(ctx, clusterID, from, to)
}

func (s *ArchitectureScoreService) PriorDay(ctx context.Context, clusterID string, day time.Time) (domain.ArchitectureScorePrior, bool, error) {
	priorDay := day.UTC().AddDate(0, 0, -1)
	row, ok, err := s.metrics.GetArchitectureScoreDaily(ctx, clusterID, priorDay)
	if err != nil || !ok {
		return domain.ArchitectureScorePrior{}, ok, err
	}
	return domain.ArchitectureScorePrior{
		Score:  row.Score,
		AvgLag: row.AvgLag,
		HasLag: row.AvgLag > 0,
	}, true, nil
}

func (s *ArchitectureScoreService) UpsertToday(ctx context.Context, row domain.ArchitectureScoreDailyRow) error {
	return s.metrics.UpsertArchitectureScoreDaily(ctx, row)
}
