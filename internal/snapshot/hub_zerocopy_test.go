package snapshot_test

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonitoringPayloadSharesImmutableBytes(t *testing.T) {
	hub := snapshot.NewHub()
	payload := strings.StringToBytes(`{"connections":1}`)
	hub.Publish("c1", snapshot.ClusterSnapshot{Varz: payload})

	a, _, ok := hub.MonitoringPayload("c1", "/varz")
	require.True(t, ok)
	b, _, ok := hub.MonitoringPayload("c1", "/varz")
	require.True(t, ok)
	// Same backing array after zero-copy reads.
	assert.Equal(t, &a[0], &b[0])
}
