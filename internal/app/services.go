package app

import (
	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type Services struct {
	Cluster   *ClusterService
	Health    *HealthService
	Users     *UserService
	Audit     *AuditService
	JetStream port.ClusterGateway
	Auth      *auth.Service
	Assistant *assistant.Service
}

func NewServices(
	uow port.UnitOfWork,
	gateway port.ClusterGateway,
	authSvc *auth.Service,
	assistantSvc *assistant.Service,
) *Services {
	return &Services{
		Cluster:   NewClusterService(uow, gateway),
		Health:    NewHealthService(uow, gateway),
		Users:     NewUserService(uow),
		Audit:     NewAuditService(uow),
		JetStream: gateway,
		Auth:      authSvc,
		Assistant: assistantSvc,
	}
}
