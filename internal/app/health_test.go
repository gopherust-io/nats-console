package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthService_AllOK(t *testing.T) {
	t.Parallel()
	s := &HealthService{
		timeout: time.Second,
		deps: []Dependency{
			{Name: depPostgres, Required: true, Ping: func(context.Context) error { return nil }},
			{Name: depNATSDefaultCluster, Required: true, Ping: func(context.Context) error { return nil }},
		},
	}

	status, code := s.Check(context.Background())
	require.Equal(t, 200, code)
	assert.Equal(t, healthStatusOK, status.Status)
	assert.Equal(t, healthStatusOK, status.Postgres)
	assert.Equal(t, healthStatusOK, status.NATSDefaultCluster)
}

func TestHealthService_PostgresFail(t *testing.T) {
	t.Parallel()
	s := &HealthService{
		timeout: time.Second,
		deps: []Dependency{
			{Name: depPostgres, Required: true, Ping: func(context.Context) error { return errors.New("db down") }},
			{Name: depNATSDefaultCluster, Required: true, Ping: func(context.Context) error { return nil }},
		},
	}

	status, code := s.Check(context.Background())
	require.Equal(t, 503, code)
	assert.Equal(t, healthStatusDegraded, status.Status)
	assert.Equal(t, healthStatusError, status.Postgres)
	assert.Equal(t, healthStatusOK, status.NATSDefaultCluster)
}

func TestHealthService_NATSFail(t *testing.T) {
	t.Parallel()
	s := &HealthService{
		timeout: time.Second,
		deps: []Dependency{
			{Name: depPostgres, Required: true, Ping: func(context.Context) error { return nil }},
			{Name: depNATSDefaultCluster, Required: true, Ping: func(context.Context) error { return errors.New("nats down") }},
		},
	}

	status, code := s.Check(context.Background())
	require.Equal(t, 503, code)
	assert.Equal(t, healthStatusDegraded, status.Status)
	assert.Equal(t, healthStatusOK, status.Postgres)
	assert.Equal(t, healthStatusError, status.NATSDefaultCluster)
}

func TestHealthService_NATSUnknown(t *testing.T) {
	t.Parallel()
	s := &HealthService{
		timeout: time.Second,
		deps: []Dependency{
			{Name: depPostgres, Required: true, Ping: func(context.Context) error { return nil }},
			{Name: depNATSDefaultCluster, Required: true, Ping: func(context.Context) error { return ErrDependencyUnknown }},
		},
	}

	status, code := s.Check(context.Background())
	require.Equal(t, 503, code)
	assert.Equal(t, healthStatusDegraded, status.Status)
	assert.Equal(t, healthStatusUnknown, status.NATSDefaultCluster)
}

func TestHealthService_PingTimeout(t *testing.T) {
	t.Parallel()
	s := &HealthService{
		timeout: 20 * time.Millisecond,
		deps: []Dependency{
			{
				Name:     depPostgres,
				Required: true,
				Ping: func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(time.Second):
						return nil
					}
				},
			},
			{Name: depNATSDefaultCluster, Required: true, Ping: func(context.Context) error { return nil }},
		},
	}

	status, code := s.Check(context.Background())
	require.Equal(t, 503, code)
	assert.Equal(t, healthStatusDegraded, status.Status)
	assert.Equal(t, healthStatusError, status.Postgres)
	assert.Equal(t, healthStatusOK, status.NATSDefaultCluster)
}
