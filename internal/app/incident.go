package app

import (
	"context"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type IncidentService struct {
	incidents port.IncidentRepository
}

func NewIncidentService(incidents port.IncidentRepository) *IncidentService {
	return &IncidentService{incidents: incidents}
}

func (s *IncidentService) CreateAnnotation(ctx context.Context, clusterID string, in domain.IncidentAnnotationCreate) (domain.IncidentAnnotation, error) {
	return s.incidents.InsertIncidentAnnotation(ctx, clusterID, in)
}

func (s *IncidentService) Reconstruction(
	ctx context.Context,
	clusterID, stream, consumer string,
	from, to time.Time,
) (domain.IncidentReconstruction, error) {
	annotations, err := s.incidents.ListIncidentAnnotations(ctx, clusterID, from, to)
	if err != nil {
		return domain.IncidentReconstruction{}, err
	}
	samples, err := s.incidents.ListIncidentConsumerSamples(ctx, clusterID, stream, consumer, from, to)
	if err != nil {
		return domain.IncidentReconstruction{}, err
	}
	nodeEvents, err := s.incidents.ListIncidentNodeEvents(ctx, clusterID, from, to)
	if err != nil {
		return domain.IncidentReconstruction{}, err
	}
	audit, err := s.incidents.ListAuditInRange(ctx, clusterID, from, to, 200)
	if err != nil {
		return domain.IncidentReconstruction{}, err
	}
	out := domain.ComputeIncidentTimeline(domain.IncidentReconstructionInput{
		ClusterID:   clusterID,
		Stream:      stream,
		Consumer:    consumer,
		From:        from,
		To:          to,
		Annotations: annotations,
		Samples:     samples,
		NodeEvents:  nodeEvents,
		Audit:       audit,
	})
	if out.Events == nil {
		out.Events = []domain.IncidentTimelineEvent{}
	}
	return out, nil
}
