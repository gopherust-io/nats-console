package apikit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonitoringCandidates(t *testing.T) {
	t.Parallel()
	got := MonitoringCandidates(
		"nats://host.docker.internal:4222,nats://host.docker.internal:4223,nats://host.docker.internal:4224",
		"http://host.docker.internal:8222",
	)
	require.Equal(t, []string{
		"http://host.docker.internal:8222",
		"http://host.docker.internal:8223",
		"http://host.docker.internal:8224",
	}, got)
}

func TestMonitoringCandidatesHTTPS(t *testing.T) {
	t.Parallel()
	got := MonitoringCandidates("nats://nats.example:4222", "https://nats.example:8222")
	assert.Equal(t, []string{
		"https://nats.example:8222",
	}, got)
}

func TestMonitoringCandidatesDedupesPrimary(t *testing.T) {
	t.Parallel()
	got := MonitoringCandidates("nats://127.0.0.1:4222", "http://127.0.0.1:8222")
	assert.Equal(t, []string{"http://127.0.0.1:8222"}, got)
}
