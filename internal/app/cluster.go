package app

import (
	"context"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type ClusterService struct {
	clusters port.ClusterRepository
	gateway  port.ClusterGateway
}

func NewClusterService(clusters port.ClusterRepository, gateway port.ClusterGateway) *ClusterService {
	return &ClusterService{clusters: clusters, gateway: gateway}
}

func (s *ClusterService) List(ctx context.Context) ([]domain.Cluster, error) {
	return s.clusters.ListClusters(ctx)
}

func (s *ClusterService) Get(ctx context.Context, id string) (domain.Cluster, error) {
	return s.clusters.GetCluster(ctx, id)
}

func (s *ClusterService) Create(ctx context.Context, in domain.ClusterCreate) (domain.Cluster, error) {
	cluster, err := s.clusters.CreateCluster(ctx, in)
	if err != nil {
		return domain.Cluster{}, err
	}
	s.gateway.Evict(cluster.ID)
	return cluster, nil
}

func (s *ClusterService) Update(ctx context.Context, id string, in domain.ClusterUpdate) (domain.Cluster, error) {
	cluster, err := s.clusters.UpdateCluster(ctx, id, in)
	if err != nil {
		return domain.Cluster{}, err
	}
	s.gateway.Evict(id)
	return cluster, nil
}

func (s *ClusterService) Delete(ctx context.Context, id string) error {
	if err := s.clusters.DeleteCluster(ctx, id); err != nil {
		return err
	}
	s.gateway.Evict(id)
	return nil
}

func (s *ClusterService) Test(ctx context.Context, id string) (domain.ClusterTestResult, error) {
	return s.gateway.Test(ctx, id)
}

func (s *ClusterService) ConnectionStatus(ctx context.Context, id string) (domain.NATSConnectionStatus, error) {
	return s.gateway.ConnectionStatus(ctx, id)
}

func (s *ClusterService) ListConnectionStatuses(ctx context.Context) []domain.NATSConnectionStatus {
	return s.gateway.ListConnectionStatuses(ctx)
}

func (s *ClusterService) BootstrapDefault(ctx context.Context) error {
	return s.gateway.BootstrapDefault(ctx)
}
