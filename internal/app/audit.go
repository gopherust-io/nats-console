package app

import (
	"context"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type AuditService struct {
	audit port.AuditRepository
}

func NewAuditService(audit port.AuditRepository) *AuditService {
	return &AuditService{audit: audit}
}

func (s *AuditService) List(ctx context.Context, filter domain.AuditFilter) ([]domain.AuditEntry, int, error) {
	return s.audit.ListAudit(ctx, filter)
}

func (s *AuditService) Log(ctx context.Context, in domain.AuditCreate) {
	_ = s.audit.Insert(ctx, in)
}
