package domain

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamInfoFromNATSUsesCamelCaseFields(t *testing.T) {
	info := StreamInfoFromNATS(&nats.StreamInfo{
		Config: nats.StreamConfig{
			Name:        "ORDERS",
			Description: "orders",
			Retention:   nats.LimitsPolicy,
			Storage:     nats.FileStorage,
			Discard:     nats.DiscardNew,
			MaxMsgs:     100,
			Replicas:    3,
			AllowRollup: true,
			DenyDelete:  true,
			Placement: &nats.Placement{
				Cluster: "east",
				Tags:    []string{"ssd"},
			},
		},
		State: nats.StreamState{
			Msgs:      5,
			FirstSeq:  1,
			LastSeq:   5,
			Consumers: 2,
		},
	})
	assert.Equal(t, "ORDERS", info.Config.Name)
	assert.False(t, info.IsDLQ)
	assert.Equal(t, "orders", info.Config.Description)
	assert.Equal(t, "limits", info.Config.Retention)
	assert.Equal(t, "new", info.Config.Discard)
	assert.Equal(t, 3, info.Config.Replicas)
	assert.True(t, info.Config.AllowRollup)
	assert.True(t, info.Config.DenyDelete)
	require.NotNil(t, info.Config.Placement)
	assert.Equal(t, "east", info.Config.Placement.Cluster)
	assert.Equal(t, uint64(1), info.State.FirstSeq)
	assert.Equal(t, 2, info.State.ConsumerCount)
}

func TestConsumerInfoFromNATSUsesCamelCaseFields(t *testing.T) {
	info := ConsumerInfoFromNATS(&nats.ConsumerInfo{
		Stream: "ORDERS",
		Name:   "worker",
		Config: nats.ConsumerConfig{
			Durable:       "worker",
			DeliverPolicy: nats.DeliverAllPolicy,
			AckPolicy:     nats.AckExplicitPolicy,
		},
		NumPending:    3,
		NumAckPending: 1,
		Delivered: nats.SequenceInfo{
			Consumer: 10,
			Stream:   100,
		},
	})
	require.Equal(t, "ORDERS", info.StreamName)
	assert.Equal(t, "worker", info.Config.DurableName)
	assert.Equal(t, uint64(3), info.NumPending)
	require.NotNil(t, info.Delivered)
	assert.Equal(t, uint64(10), info.Delivered.ConsumerSeq)
}

func TestAccountInfoFromNATSUsesCamelCaseFields(t *testing.T) {
	info := AccountInfoFromNATS(&nats.AccountInfo{
		Memory:    1024,
		Store:     2048,
		Streams:   1,
		Consumers: 2,
		Limits: nats.AccountLimits{
			MaxMemory:    4096,
			MaxStore:     8192,
			MaxStreams:   10,
			MaxConsumers: 20,
		},
	})
	assert.Equal(t, uint64(1024), info.Memory)
	assert.Equal(t, int64(4096), info.Limits.MaxMemory)
	assert.Equal(t, 10, info.Limits.MaxStreams)
}

func TestStreamInfoFromNATSMarksDLQ(t *testing.T) {
	byName := StreamInfoFromNATS(&nats.StreamInfo{
		Config: nats.StreamConfig{Name: "ORDERS_DLQ", Retention: nats.LimitsPolicy, Storage: nats.FileStorage},
	})
	assert.True(t, byName.IsDLQ)

	byMeta := StreamInfoFromNATS(&nats.StreamInfo{
		Config: nats.StreamConfig{
			Name:     "POISON",
			Metadata: map[string]string{MetadataRoleKey: MetadataRoleDLQ},
			Retention: nats.LimitsPolicy,
			Storage:   nats.FileStorage,
		},
	})
	assert.True(t, byMeta.IsDLQ)
}
