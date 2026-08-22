package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/pkg/common/safe"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const (
	depPostgres           = "postgres"
	depNATSDefaultCluster = "natsDefaultCluster"
)

const (
	healthStatusOK       = "ok"
	healthStatusError    = "error"
	healthStatusUnknown  = "unknown"
	healthStatusDegraded = "degraded"
)

// ErrDependencyUnknown marks a required dependency that could not be evaluated
// (e.g. no default NATS cluster). Check maps it to "unknown" and still fails readiness.
var ErrDependencyUnknown = errors.New("dependency status unknown")

type HealthStatus struct {
	Status             string `json:"status"`
	Postgres           string `json:"postgres"`
	NATSDefaultCluster string `json:"natsDefaultCluster"`
}

// Dependency is a named external dependency that HealthService can ping.
// goalign:ignore // trailing bool padding is unavoidable
type Dependency struct {
	Ping     func(ctx context.Context) error
	Name     string
	Required bool
}

type HealthService struct {
	deps    []Dependency
	timeout time.Duration
}

func NewHealthService(clusters port.ClusterRepository, gateway port.ClusterGateway, timeout time.Duration) *HealthService {
	return &HealthService{
		timeout: max(timeout, 2*time.Second),
		deps: []Dependency{
			postgresDependency(clusters),
			natsDefaultClusterDependency(clusters, gateway),
		},
	}
}

func postgresDependency(clusters port.ClusterRepository) Dependency {
	return Dependency{
		Name:     depPostgres,
		Required: true,
		Ping:     clusters.Ping,
	}
}

func natsDefaultClusterDependency(clusters port.ClusterRepository, gateway port.ClusterGateway) Dependency {
	return Dependency{
		Name:     depNATSDefaultCluster,
		Required: true,
		Ping: func(ctx context.Context) error {
			cluster, err := clusters.GetDefaultCluster(ctx)
			if err != nil {
				return ErrDependencyUnknown
			}
			result, err := gateway.Test(ctx, cluster.ID)
			if err != nil || !result.OK || strings.IsEmpty(result.ServerName) {
				return errors.New("nats default cluster unhealthy")
			}
			return nil
		},
	}
}

func (s *HealthService) Check(ctx context.Context) (HealthStatus, int) {
	type result struct {
		name   string
		status string
	}

	results := make([]result, len(s.deps))
	var wg sync.WaitGroup

	for i, dep := range s.deps {
		wg.Go(func() {
			defer func() {
				if rec := recover(); rec != nil {
					safe.Log("health", rec)
					results[i] = result{name: dep.Name, status: healthStatusError}
				}
			}()
			status := healthStatusOK
			pCtx, cancel := context.WithTimeout(ctx, s.timeout)
			defer cancel()
			if err := dep.Ping(pCtx); err != nil {
				if errors.Is(err, ErrDependencyUnknown) {
					status = healthStatusUnknown
				} else {
					status = healthStatusError
				}
			}
			results[i] = result{name: dep.Name, status: status}
		})
	}
	wg.Wait()

	byName := make(map[string]string, len(results))
	for _, r := range results {
		byName[r.name] = r.status
	}

	overall := healthStatusOK
	code := 200
	for _, dep := range s.deps {
		if !dep.Required {
			continue
		}
		if byName[dep.Name] != healthStatusOK {
			overall = healthStatusDegraded
			code = 503
			break
		}
	}

	return HealthStatus{
		Status:             overall,
		Postgres:           byName[depPostgres],
		NATSDefaultCluster: byName[depNATSDefaultCluster],
	}, code
}
