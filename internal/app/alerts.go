package app

import (
	"context"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type AlertService struct {
	alerts port.AlertRepository
}

func NewAlertService(alerts port.AlertRepository) *AlertService {
	return &AlertService{alerts: alerts}
}

func (s *AlertService) List(ctx context.Context, filter domain.AlertFilter) ([]domain.Alert, int, error) {
	return s.alerts.ListAlerts(ctx, filter)
}

func (s *AlertService) OpenUnacknowledged(ctx context.Context, clusterIDs []string, limit int) ([]domain.Alert, int, error) {
	return s.alerts.ListOpenUnacknowledged(ctx, clusterIDs, limit)
}

func (s *AlertService) Get(ctx context.Context, id string) (domain.Alert, error) {
	return s.alerts.GetAlert(ctx, id)
}

func (s *AlertService) Acknowledge(ctx context.Context, id, actor string) (domain.Alert, error) {
	return s.alerts.AcknowledgeAlert(ctx, id, actor)
}

func (s *AlertService) ListRules(ctx context.Context, clusterID string, enabledOnly bool) ([]domain.AlertRule, error) {
	return s.alerts.ListAlertRules(ctx, clusterID, enabledOnly)
}

func (s *AlertService) GetRule(ctx context.Context, id string) (domain.AlertRule, error) {
	return s.alerts.GetAlertRule(ctx, id)
}

func (s *AlertService) CreateRule(ctx context.Context, in domain.AlertRuleCreate, createdBy string) (domain.AlertRule, error) {
	return s.alerts.CreateAlertRule(ctx, in, createdBy)
}

func (s *AlertService) UpdateRule(ctx context.Context, id string, in domain.AlertRuleUpdate) (domain.AlertRule, error) {
	return s.alerts.UpdateAlertRule(ctx, id, in)
}

func (s *AlertService) DeleteRule(ctx context.Context, id string) error {
	return s.alerts.DeleteAlertRule(ctx, id)
}
