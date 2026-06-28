package app

import (
	"context"

	"github.com/gopherust-io/nats-consol/internal/port"
)

type HealthStatus struct {
	Status             string `json:"status"`
	Postgres           string `json:"postgres"`
	NATSDefaultCluster string `json:"natsDefaultCluster"`
}

type HealthService struct {
	clusters port.ClusterRepository
	gateway  port.ClusterGateway
}

func NewHealthService(clusters port.ClusterRepository, gateway port.ClusterGateway) *HealthService {
	return &HealthService{clusters: clusters, gateway: gateway}
}

func (s *HealthService) Check(ctx context.Context) (HealthStatus, int) {
	postgresStatus := "ok"
	if err := s.clusters.Ping(ctx); err != nil {
		postgresStatus = "error"
	}

	natsStatus := "unknown"
	if cluster, err := s.clusters.GetDefaultCluster(ctx); err == nil {
		result, err := s.gateway.Test(ctx, cluster.ID)
		if err == nil && result.OK && result.ServerName != "" {
			natsStatus = "ok"
		} else {
			natsStatus = "error"
		}
	}

	status := "ok"
	code := 200
	if postgresStatus != "ok" {
		status = "degraded"
		code = 503
	}

	return HealthStatus{
		Status:             status,
		Postgres:           postgresStatus,
		NATSDefaultCluster: natsStatus,
	}, code
}
