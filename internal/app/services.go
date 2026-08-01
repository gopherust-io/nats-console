package app

import (
	"time"

	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type Services struct {
	Cluster      *ClusterService
	Health       *HealthService
	Users        *UserService
	Audit        *AuditService
	Admin        *AdminService
	Metrics      *MetricsService
	Bottlenecks  *BottleneckService
	ArchScore    *ArchitectureScoreService
	Incident     *IncidentService
	EventCatalog *EventCatalogService
	Alerts       *AlertService
	Access       *AccessService
	NATSAccounts *NATSAccountService
	JetStream    port.ClusterGateway
	Auth         *auth.Service
	Assistant    *assistant.Service
}

func NewServices(
	uow port.UnitOfWork,
	gateway port.ClusterGateway,
	authSvc *auth.Service,
	assistantSvc *assistant.Service,
	healthTimeout time.Duration,
) *Services {
	return &Services{
		Cluster:      NewClusterService(uow, gateway),
		Health:       NewHealthService(uow, gateway, healthTimeout),
		Users:        NewUserService(uow),
		Audit:        NewAuditService(uow),
		Admin:        NewAdminService(uow),
		Metrics:      NewMetricsService(uow),
		Bottlenecks:  NewBottleneckService(uow, 672*time.Hour),
		ArchScore:    NewArchitectureScoreService(uow),
		Incident:     NewIncidentService(uow),
		EventCatalog: NewEventCatalogService(uow),
		Alerts:       NewAlertService(uow),
		Access:       NewAccessService(uow),
		NATSAccounts: NewNATSAccountService(uow),
		JetStream:    gateway,
		Auth:         authSvc,
		Assistant:    assistantSvc,
	}
}
