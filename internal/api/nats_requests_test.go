package api

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamConfigRequestToNATS(t *testing.T) {
	cfg, err := streamConfigRequest{
		Name:      "ORDERS",
		Subjects:  []string{"orders.>"},
		Retention: "limits",
		Storage:   "file",
		MaxMsgs:   100,
	}.toNATS()
	require.NoError(t, err)
	assert.Equal(t, "ORDERS", cfg.Name)
	assert.Equal(t, nats.LimitsPolicy, cfg.Retention)
	assert.Equal(t, int64(100), cfg.MaxMsgs)
}

func TestConsumerConfigRequestToNATS(t *testing.T) {
	cfg, err := consumerConfigRequest{
		DurableName:   "worker",
		DeliverPolicy: "all",
		AckPolicy:     "explicit",
	}.toNATS()
	require.NoError(t, err)
	assert.Equal(t, "worker", cfg.Durable)
	assert.Equal(t, nats.DeliverAllPolicy, cfg.DeliverPolicy)
	assert.Equal(t, nats.AckExplicitPolicy, cfg.AckPolicy)
}
