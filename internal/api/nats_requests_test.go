package api

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsumerConfigRequestToNATSStartFields(t *testing.T) {
	t.Parallel()

	cfg, err := (consumerConfigRequest{
		DurableName:   "worker",
		DeliverPolicy: "by_start_sequence",
		AckPolicy:     "explicit",
		OptStartSeq:   42,
		ReplayPolicy:  "original",
	}).toNATS()
	require.NoError(t, err)
	assert.Equal(t, nats.DeliverByStartSequencePolicy, cfg.DeliverPolicy)
	assert.Equal(t, uint64(42), cfg.OptStartSeq)
	assert.Equal(t, nats.ReplayOriginalPolicy, cfg.ReplayPolicy)

	cfg, err = (consumerConfigRequest{
		DurableName:   "timed",
		DeliverPolicy: "by_start_time",
		OptStartTime:  "2026-07-25T10:00:00Z",
	}).toNATS()
	require.NoError(t, err)
	require.NotNil(t, cfg.OptStartTime)
	assert.True(t, cfg.OptStartTime.Equal(time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)))
}
