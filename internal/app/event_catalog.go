package app

import (
	"context"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type EventCatalogService struct {
	catalog port.EventCatalogRepository
}

func NewEventCatalogService(catalog port.EventCatalogRepository) *EventCatalogService {
	return &EventCatalogService{catalog: catalog}
}

func (s *EventCatalogService) ListDocs(ctx context.Context, clusterID string) ([]domain.EventCatalogDoc, error) {
	return s.catalog.ListEventCatalogEntries(ctx, clusterID)
}

func (s *EventCatalogService) Upsert(ctx context.Context, clusterID, subject, updatedBy string, in domain.EventCatalogUpsert) (domain.EventCatalogDoc, error) {
	return s.catalog.UpsertEventCatalogEntry(ctx, clusterID, subject, updatedBy, in)
}

func (s *EventCatalogService) Delete(ctx context.Context, clusterID, subject string) error {
	return s.catalog.DeleteEventCatalogEntry(ctx, clusterID, subject)
}
