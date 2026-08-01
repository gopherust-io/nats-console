package app

import (
	"context"
	"errors"

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

func (s *ClusterService) Delete(ctx context.Context, id string) error {
	cluster, err := s.clusters.GetCluster(ctx, id)
	if err != nil {
		return err
	}
	if cluster.IsDefault {
		return errors.New("cannot delete the default cluster; set another as default first")
	}
	count, err := s.clusters.CountClusters(ctx)
	if err != nil {
		return err
	}
	if count <= 1 {
		return errors.New("cannot delete the last cluster")
	}
	if err := s.clusters.DeleteCluster(ctx, id); err != nil {
		return err
	}
	s.gateway.Evict(id)
	return nil
}
