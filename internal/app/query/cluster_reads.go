package query

import (
	"context"
	"errors"
	"time"

	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
)

// DefaultMaxBodyBytes matches config MaxMonitoringBodyBytes default (8 MiB).
const DefaultMaxBodyBytes int64 = 8 << 20

// ErrPayloadTooLarge is returned when a live monitoring body exceeds the configured limit.
var ErrPayloadTooLarge = errors.New("monitoring payload too large")

// ExecutorGetter is the subset of port.ClusterGateway needed for live monitoring reads.
type ExecutorGetter interface {
	GetExecutor(ctx context.Context, clusterID string) (port.JetStreamExecutor, error)
}

// Service is the CQRS-lite read side for monitoring payloads: prefer snapshot hub, else live executor.
type Service struct {
	gateway      ExecutorGetter
	hub          *snapshot.Hub
	maxBodyBytes int64
}

// NewService constructs a cluster monitoring read service. hub may be nil until wired.
// maxBodyBytes <= 0 uses DefaultMaxBodyBytes.
func NewService(gateway ExecutorGetter, hub *snapshot.Hub, maxBodyBytes int64) *Service {
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxBodyBytes
	}
	return &Service{gateway: gateway, hub: hub, maxBodyBytes: maxBodyBytes}
}

// SetHub wires or replaces the optional snapshot hub (bootstrap after metrics start).
func (s *Service) SetHub(hub *snapshot.Hub) {
	if s == nil {
		return
	}
	s.hub = hub
}

// SetMaxBodyBytes updates the live-fetch size limit (0 or negative restores default).
func (s *Service) SetMaxBodyBytes(n int64) {
	if s == nil {
		return
	}
	if n <= 0 {
		n = DefaultMaxBodyBytes
	}
	s.maxBodyBytes = n
}

// PreferSnapshotMonitoring returns a cached monitoring body from the hub when present.
// The returned slice is shared/immutable (same contract as snapshot.Hub.MonitoringPayload).
func (s *Service) PreferSnapshotMonitoring(clusterID, path string) ([]byte, time.Time, bool) {
	if s == nil || s.hub == nil {
		return nil, time.Time{}, false
	}
	return s.hub.MonitoringPayload(clusterID, path)
}

// FetchMonitoring prefers the snapshot hub unless fresh is true, then falls back to the live executor.
func (s *Service) FetchMonitoring(ctx context.Context, clusterID, path string, fresh bool) ([]byte, time.Time, error) {
	if s == nil {
		return nil, time.Time{}, errors.New("query service unavailable")
	}
	if !fresh {
		if raw, at, ok := s.PreferSnapshotMonitoring(clusterID, path); ok {
			return raw, at, nil
		}
	}
	return s.fetchLive(ctx, clusterID, path)
}

func (s *Service) fetchLive(ctx context.Context, clusterID, path string) ([]byte, time.Time, error) {
	if s.gateway == nil {
		return nil, time.Time{}, errors.New("cluster gateway unavailable")
	}
	client, err := s.gateway.GetExecutor(ctx, clusterID)
	if err != nil {
		return nil, time.Time{}, err
	}
	raw, err := client.Monitoring(ctx, path)
	if err != nil {
		return nil, time.Time{}, err
	}
	if s.maxBodyBytes > 0 && int64(len(raw)) > s.maxBodyBytes {
		return nil, time.Time{}, ErrPayloadTooLarge
	}
	return raw, time.Now().UTC(), nil
}
