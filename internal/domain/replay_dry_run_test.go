package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeReplayDryRunSeqRange(t *testing.T) {
	t.Parallel()

	stream := StreamInfo{State: StreamStateDTO{FirstSeq: 1, LastSeq: 10_000_000, Messages: 10_000_000}}
	target := ConsumerInfo{
		Name: "pay",
		Config: ConsumerConfigDTO{
			DurableName: "pay",
			Metadata:    map[string]string{MetadataServiceKey: "payment-service"},
		},
	}
	req := ReplayConsumerRequest{
		Mode:         ReplayModeReset,
		From:         ReplayFromSeq,
		Seq:          1,
		UntilSeq:     10_000_000,
		Limit:        10_000_000,
		ReplayPolicy: ReplayPolicyInstant,
	}

	got, err := ComputeReplayDryRun(req, stream, target)
	require.NoError(t, err)
	assert.Equal(t, uint64(10_000_000), got.Messages)
	assert.Equal(t, int64(10_000)*1000, got.EstimatedDurationMs) // 10M / 1000/s = 10000s
	assert.Equal(t, 1, got.ConsumersAffected)
	assert.Equal(t, []string{"payment-service"}, got.PotentialDuplicates)
	assert.False(t, got.Unbounded)
	assert.False(t, got.Approximate)
}

func TestComputeReplayDryRunBeginningToLastSeqUnbounded(t *testing.T) {
	t.Parallel()

	stream := StreamInfo{State: StreamStateDTO{FirstSeq: 5, LastSeq: 104}}
	target := ConsumerInfo{Name: "worker", Config: ConsumerConfigDTO{DurableName: "worker"}}
	req := ReplayConsumerRequest{From: ReplayFromBeginning, Mode: ReplayModeReset}

	got, err := ComputeReplayDryRun(req, stream, target)
	require.NoError(t, err)
	assert.Equal(t, uint64(100), got.Messages) // 104-5+1
	assert.True(t, got.Unbounded)
	assert.Equal(t, []string{"worker"}, got.PotentialDuplicates)
	assert.Equal(t, int64(1000), got.EstimatedDurationMs) // ceil(100/1000)=1s
}

func TestComputeReplayDryRunUntilSeqWithoutLimit(t *testing.T) {
	t.Parallel()

	stream := StreamInfo{State: StreamStateDTO{FirstSeq: 1, LastSeq: 500}}
	req := ReplayConsumerRequest{From: ReplayFromSeq, Seq: 10, UntilSeq: 19}
	got, err := ComputeReplayDryRun(req, stream, ConsumerInfo{Name: "c"})
	require.NoError(t, err)
	assert.Equal(t, uint64(10), got.Messages)
	assert.False(t, got.Unbounded)
}

func TestComputeReplayDryRunCapsUntilAtLastSeq(t *testing.T) {
	t.Parallel()

	stream := StreamInfo{State: StreamStateDTO{FirstSeq: 1, LastSeq: 50}}
	req := ReplayConsumerRequest{From: ReplayFromSeq, Seq: 40, UntilSeq: 100}
	got, err := ComputeReplayDryRun(req, stream, ConsumerInfo{Name: "c"})
	require.NoError(t, err)
	assert.Equal(t, uint64(11), got.Messages) // 50-40+1
}

func TestComputeReplayDryRunTimeApproximate(t *testing.T) {
	t.Parallel()

	stream := StreamInfo{State: StreamStateDTO{FirstSeq: 1, LastSeq: 1000}}
	req := ReplayConsumerRequest{
		From:     ReplayFromTime,
		Time:     "2024-01-01T00:00:00Z",
		UntilTime: "2024-01-01T01:00:00Z",
	}
	got, err := ComputeReplayDryRun(req, stream, ConsumerInfo{Name: "c"})
	require.NoError(t, err)
	assert.True(t, got.Approximate)
	assert.False(t, got.Unbounded)
	assert.Equal(t, uint64(0), got.Messages)
}

func TestComputeReplayDryRunOriginalPolicyUsesWallClock(t *testing.T) {
	t.Parallel()

	stream := StreamInfo{State: StreamStateDTO{FirstSeq: 1, LastSeq: 100}}
	req := ReplayConsumerRequest{
		From:         ReplayFromTime,
		Time:         "2024-01-01T00:00:00Z",
		UntilTime:    "2024-01-01T03:00:00Z",
		Limit:        100,
		ReplayPolicy: ReplayPolicyOriginal,
	}
	got, err := ComputeReplayDryRun(req, stream, ConsumerInfo{Name: "c"})
	require.NoError(t, err)
	assert.Equal(t, uint64(100), got.Messages)
	assert.Equal(t, int64(3*60*60*1000), got.EstimatedDurationMs)
	assert.False(t, got.Approximate) // limit set → known count
}

func TestComputeReplayDryRunFromNew(t *testing.T) {
	t.Parallel()

	got, err := ComputeReplayDryRun(
		ReplayConsumerRequest{From: ReplayFromNew},
		StreamInfo{State: StreamStateDTO{LastSeq: 99}},
		ConsumerInfo{},
	)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), got.Messages)
	assert.True(t, got.Unbounded)
	assert.Empty(t, got.PotentialDuplicates)
	assert.Equal(t, int64(0), got.EstimatedDurationMs)
}

func TestComputeReplayDryRunRejectsInvalid(t *testing.T) {
	t.Parallel()

	_, err := ComputeReplayDryRun(ReplayConsumerRequest{}, StreamInfo{}, ConsumerInfo{})
	require.Error(t, err)
}

func TestEstimateReplayDurationMs(t *testing.T) {
	t.Parallel()

	assert.Equal(t, int64(0), EstimateReplayDurationMs(0, 1000))
	assert.Equal(t, int64(1000), EstimateReplayDurationMs(1, 1000))
	assert.Equal(t, int64(1000), EstimateReplayDurationMs(1000, 1000))
	assert.Equal(t, int64(2000), EstimateReplayDurationMs(1001, 1000))
	assert.Equal(t, int64(1000), EstimateReplayDurationMs(5, 0)) // fallback rate
}
