package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamMetricRoundTrip(t *testing.T) {
	t.Parallel()

	name := StreamMetric("ORDERS", StreamMetricKindLastSeq)
	assert.Equal(t, "stream:ORDERS:last_seq", name)

	stream, kind, ok := ParseStreamMetric(name)
	require.True(t, ok)
	assert.Equal(t, "ORDERS", stream)
	assert.Equal(t, StreamMetricKindLastSeq, kind)

	stream, kind, ok = ParseStreamMetric("stream:foo/bar:delivered_seq")
	require.True(t, ok)
	assert.Equal(t, "foo/bar", stream)
	assert.Equal(t, StreamMetricKindDeliveredSeq, kind)

	_, _, ok = ParseStreamMetric("stream:bad:unknown")
	assert.False(t, ok)
	_, _, ok = ParseStreamMetric("server.in_msgs_total")
	assert.False(t, ok)
}

func TestIsCounterMetricStream(t *testing.T) {
	t.Parallel()

	assert.True(t, IsCounterMetric(MetricServerInMsgsTotal))
	assert.True(t, IsCounterMetric(StreamMetric("ORDERS", StreamMetricKindBytes)))
	assert.True(t, IsCounterMetric(StreamMetric("ORDERS", StreamMetricKindAckFloorSeq)))
	assert.False(t, IsCounterMetric(MetricJetStreamStreams))
}

func TestValidHistoryMetricName(t *testing.T) {
	t.Parallel()

	assert.True(t, ValidHistoryMetricName(MetricJetStreamStreams))
	assert.True(t, ValidHistoryMetricName(StreamMetric("ORDERS", StreamMetricKindLastSeq)))
	assert.True(t, ValidHistoryMetricName(RequestReplyProbeMetric("orders.status")))
	assert.False(t, ValidMetricName(StreamMetric("ORDERS", StreamMetricKindLastSeq)))
	assert.False(t, ValidHistoryMetricName("stream:ORDERS:redeliver"))
	assert.False(t, ValidHistoryMetricName("not.a.metric"))
}

func TestStreamRateMetricsCSV(t *testing.T) {
	t.Parallel()

	csv := StreamRateMetricsCSV("ORDERS")
	assert.Equal(t,
		"stream:ORDERS:last_seq,stream:ORDERS:delivered_seq,stream:ORDERS:ack_floor_seq,stream:ORDERS:bytes",
		csv,
	)
}
