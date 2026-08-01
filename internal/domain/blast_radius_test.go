package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "payment-service", ServiceID(ConsumerInfo{
		Name: "c1",
		Config: ConsumerConfigDTO{
			DurableName: "ignored",
			Metadata:    map[string]string{MetadataServiceKey: "payment-service"},
		},
	}))
	assert.Equal(t, "durable-a", ServiceID(ConsumerInfo{
		Name:   "c1",
		Config: ConsumerConfigDTO{DurableName: "durable-a"},
	}))
	assert.Equal(t, "cfg-name", ServiceID(ConsumerInfo{
		Name:   "c1",
		Config: ConsumerConfigDTO{Name: "cfg-name"},
	}))
	assert.Equal(t, "c1", ServiceID(ConsumerInfo{Name: "c1"}))
	assert.Equal(t, "", ServiceID(ConsumerInfo{}))
}

func TestIsCriticalConsumer(t *testing.T) {
	t.Parallel()

	assert.True(t, IsCriticalConsumer(ConsumerInfo{
		Config: ConsumerConfigDTO{Metadata: map[string]string{MetadataCriticalKey: "true"}},
	}))
	assert.True(t, IsCriticalConsumer(ConsumerInfo{
		Config: ConsumerConfigDTO{Metadata: map[string]string{MetadataCriticalKey: "TRUE"}},
	}))
	assert.False(t, IsCriticalConsumer(ConsumerInfo{
		Config: ConsumerConfigDTO{Metadata: map[string]string{MetadataCriticalKey: "false"}},
	}))
	assert.False(t, IsCriticalConsumer(ConsumerInfo{}))
}

func TestComputeBlastRadius(t *testing.T) {
	t.Parallel()

	target := StreamInfo{Config: StreamConfigDTO{Name: "ORDERS"}}
	consumers := []ConsumerInfo{
		{
			Name: "pay-1",
			Config: ConsumerConfigDTO{
				DurableName: "pay-1",
				Metadata: map[string]string{
					MetadataServiceKey:  "payment-service",
					MetadataCriticalKey: "true",
				},
			},
		},
		{
			Name: "pay-2",
			Config: ConsumerConfigDTO{
				DurableName: "pay-2",
				Metadata: map[string]string{
					MetadataServiceKey:  "payment-service",
					MetadataCriticalKey: "true",
				},
			},
		},
		{
			Name: "bill-1",
			Config: ConsumerConfigDTO{
				DurableName: "bill-1",
				Metadata: map[string]string{
					MetadataServiceKey:  "billing-service",
					MetadataCriticalKey: "true",
				},
			},
		},
		{
			Name:   "orders-worker",
			Config: ConsumerConfigDTO{DurableName: "orders-worker"},
		},
		{
			Name:   "orders-worker-2",
			Config: ConsumerConfigDTO{DurableName: "orders-worker"},
		},
	}

	allStreams := []StreamInfo{
		target,
		{
			Config: StreamConfigDTO{
				Name:   "ORDERS_MIRROR",
				Mirror: &StreamSourceDTO{Name: "ORDERS"},
			},
		},
		{
			Config: StreamConfigDTO{
				Name: "ORDERS_AGG",
				Sources: []StreamSourceDTO{
					{Name: "ORDERS"},
					{Name: "OTHER"},
				},
			},
		},
		{
			Config: StreamConfigDTO{Name: "ORDERS_DLQ"},
			IsDLQ:  true,
		},
		{
			Config: StreamConfigDTO{
				Name: "UNRELATED_DLQ",
			},
			IsDLQ: true,
		},
		{
			Config: StreamConfigDTO{
				Name:   "OTHER_MIRROR",
				Mirror: &StreamSourceDTO{Name: "OTHER"},
			},
		},
	}

	got := ComputeBlastRadius(target, consumers, allStreams)
	assert.Equal(t, "ORDERS", got.Stream)
	assert.Equal(t, 3, got.Services) // payment, billing, orders-worker
	assert.Equal(t, 5, got.Consumers)
	assert.Equal(t, 3, got.Streams) // ORDERS_MIRROR, ORDERS_AGG, ORDERS_DLQ
	assert.Equal(t, []string{"billing-service", "payment-service"}, got.Critical)
	assert.Equal(t, []string{"billing-service", "orders-worker", "payment-service"}, got.ServiceNames)
	assert.Equal(t, []string{"ORDERS_AGG", "ORDERS_DLQ", "ORDERS_MIRROR"}, got.RelatedStreams)
	assert.Equal(t, []string{"bill-1", "orders-worker", "orders-worker-2", "pay-1", "pay-2"}, got.ConsumerNames)
}

func TestComputeBlastRadiusEmpty(t *testing.T) {
	t.Parallel()

	got := ComputeBlastRadius(
		StreamInfo{Config: StreamConfigDTO{Name: "EMPTY"}},
		nil,
		[]StreamInfo{{Config: StreamConfigDTO{Name: "EMPTY"}}},
	)
	require.Equal(t, "EMPTY", got.Stream)
	assert.Equal(t, 0, got.Services)
	assert.Equal(t, 0, got.Streams)
	assert.Equal(t, 0, got.Consumers)
	assert.Empty(t, got.Critical)
	assert.Empty(t, got.ServiceNames)
	assert.Empty(t, got.RelatedStreams)
	assert.Empty(t, got.ConsumerNames)
}
