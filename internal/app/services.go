package app

import (
	"time"

	"github.com/gopherust-io/nats-consol/internal/app/monitoring"
	"github.com/gopherust-io/nats-consol/internal/app/query"
	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
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
	JetStream    *JetStreamService
	Auth         *auth.Service
	Assistant    *assistant.Service
	Monitoring   *monitoring.Service
	Queries      *query.Service
}

// NewServices wires application services.
//
// db is accepted as port.DB for convenient composition at the edge; each
// constructor takes only the embedded repository interface it needs (fat DB
// is composition-only).
func NewServices(
	db port.DB,
	gateway port.ClusterGateway,
	authSvc *auth.Service,
	assistantSvc *assistant.Service,
	healthTimeout,
	lookBackDuration time.Duration,
) *Services {
	js := NewJetStreamService(gateway)
	queries := query.NewService(js, nil, 0)
	return &Services{
		Cluster:      NewClusterService(db, gateway),
		Health:       NewHealthService(db, gateway, healthTimeout),
		Users:        NewUserService(db),
		Audit:        NewAuditService(db),
		Admin:        NewAdminService(db),
		Metrics:      NewMetricsService(db),
		Bottlenecks:  NewBottleneckService(db, lookBackDuration),
		ArchScore:    NewArchitectureScoreService(db),
		Incident:     NewIncidentService(db),
		EventCatalog: NewEventCatalogService(db),
		Alerts:       NewAlertService(db),
		Access:       NewAccessService(db),
		NATSAccounts: NewNATSAccountService(db),
		JetStream:    js,
		Auth:         authSvc,
		Assistant:    assistantSvc,
		Queries:      queries,
		Monitoring:   monitoring.NewService(queries, 0),
	}
}

// SetSnapshotHub wires the metrics snapshot hub into Queries and Monitoring (bootstrap after hub exists).
func (s *Services) SetSnapshotHub(hub *snapshot.Hub) {
	if s == nil {
		return
	}
	if s.Queries != nil {
		s.Queries.SetHub(hub)
	}
	if s.Monitoring != nil {
		s.Monitoring.SetHub(hub)
	}
}

// ConfigureMonitoring sets live body size limit and optional JSZ cache TTL (0 = package defaults).
func (s *Services) ConfigureMonitoring(maxBodyBytes int64, cacheTTL time.Duration) {
	if s == nil {
		return
	}
	if s.Queries != nil {
		s.Queries.SetMaxBodyBytes(maxBodyBytes)
	}
	if s.Monitoring != nil {
		s.Monitoring.SetMaxBodyBytes(maxBodyBytes)
		if cacheTTL > 0 {
			s.Monitoring.SetCacheTTL(cacheTTL)
		}
	}
}
