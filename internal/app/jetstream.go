package app

import (
	"context"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
)

// JetStreamService is the application facade over port.ClusterGateway for
// JetStream executor access and connection lifecycle used by HTTP handlers.
type JetStreamService struct {
	gateway port.ClusterGateway
}

func NewJetStreamService(gateway port.ClusterGateway) *JetStreamService {
	return &JetStreamService{gateway: gateway}
}

// Gateway returns the underlying cluster gateway (e.g. for live.Hub).
func (s *JetStreamService) Gateway() port.ClusterGateway {
	if s == nil {
		return nil
	}
	return s.gateway
}

func (s *JetStreamService) WithExecutor(ctx context.Context, clusterID string, fn func(port.JetStreamExecutor) error) error {
	return s.gateway.WithExecutor(ctx, clusterID, fn)
}

func (s *JetStreamService) GetExecutor(ctx context.Context, clusterID string) (port.JetStreamExecutor, error) {
	return s.gateway.GetExecutor(ctx, clusterID)
}

func (s *JetStreamService) Evict(clusterID string) {
	s.gateway.Evict(clusterID)
}

func (s *JetStreamService) Touch(clusterID string) {
	s.gateway.Touch(clusterID)
}

func (s *JetStreamService) ConnectionStatus(ctx context.Context, clusterID string) (domain.NATSConnectionStatus, error) {
	return s.gateway.ConnectionStatus(ctx, clusterID)
}

func (s *JetStreamService) ListConnectionStatuses(ctx context.Context) []domain.NATSConnectionStatus {
	return s.gateway.ListConnectionStatuses(ctx)
}

func (s *JetStreamService) SubscribeConnectionStatus(clusterID string) (updates <-chan domain.NATSConnectionStatus, latest domain.NATSConnectionStatus, unsubscribe func()) {
	return s.gateway.SubscribeConnectionStatus(clusterID)
}

func (s *JetStreamService) BootstrapDefault(ctx context.Context) error {
	return s.gateway.BootstrapDefault(ctx)
}

func (s *JetStreamService) Test(ctx context.Context, clusterID string) (domain.ClusterTestResult, error) {
	return s.gateway.Test(ctx, clusterID)
}

func (s *JetStreamService) Stop() {
	s.gateway.Stop()
}

var _ port.ClusterGateway = (*JetStreamService)(nil)
