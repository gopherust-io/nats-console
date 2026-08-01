package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

func TestAvgPayloadBytes(t *testing.T) {
	t.Parallel()
	got, ok := domain.AvgPayloadBytes(8192, 2)
	assert.True(t, ok)
	assert.Equal(t, 4096.0, got)

	_, ok = domain.AvgPayloadBytes(100, 0)
	assert.False(t, ok)
	_, ok = domain.AvgPayloadBytes(-1, 2)
	assert.False(t, ok)
}

func TestDiscoverHiddenBottlenecksCorrelated(t *testing.T) {
	t.Parallel()
	procSlow := 2400.0
	procOK := 200.0
	var buckets []domain.BottleneckHourBucket
	for weeksAgo := range 3 {
		base := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -7*weeksAgo) // Friday
		require.Equal(t, time.Friday, base.Weekday())
		fri18 := time.Date(base.Year(), base.Month(), base.Day(), 18, 0, 0, 0, time.UTC)
		wed18 := fri18.AddDate(0, 0, -2)
		wed18 = time.Date(wed18.Year(), wed18.Month(), wed18.Day(), 18, 0, 0, 0, time.UTC)

		buckets = append(buckets,
			domain.BottleneckHourBucket{
				StreamName: "ORDERS", ConsumerName: "billing-worker", BucketHour: fri18,
				AvgLag: 400, MaxLag: 500, AvgPayloadBytes: 8000, AvgProcessingMs: &procSlow, Samples: 10,
			},
			domain.BottleneckHourBucket{
				StreamName: "ORDERS", ConsumerName: "", BucketHour: fri18,
				AvgPayloadBytes: 8000, Samples: 10,
			},
			domain.BottleneckHourBucket{
				StreamName: "ORDERS", ConsumerName: "billing-worker", BucketHour: wed18,
				AvgLag: 40, MaxLag: 50, AvgPayloadBytes: 4000, AvgProcessingMs: &procOK, Samples: 10,
			},
			domain.BottleneckHourBucket{
				StreamName: "ORDERS", ConsumerName: "", BucketHour: wed18,
				AvgPayloadBytes: 4000, Samples: 10,
			},
			domain.BottleneckHourBucket{
				StreamName: "ORDERS", ConsumerName: "billing-worker",
				BucketHour: time.Date(base.Year(), base.Month(), base.Day(), 10, 0, 0, 0, time.UTC),
				AvgLag:     35, MaxLag: 40, AvgPayloadBytes: 3900, AvgProcessingMs: &procOK, Samples: 10,
			},
			domain.BottleneckHourBucket{
				StreamName: "ORDERS", ConsumerName: "",
				BucketHour:      time.Date(base.Year(), base.Month(), base.Day(), 10, 0, 0, 0, time.UTC),
				AvgPayloadBytes: 3900, Samples: 10,
			},
		)
	}

	snap := domain.DiscoverHiddenBottlenecks(buckets)
	require.NotEmpty(t, snap.Findings)
	assert.NotEqual(t, domain.BottleneckVerdictHealthy, snap.Verdict)

	var correlated bool
	for _, f := range snap.Findings {
		if f.Kind == domain.BottleneckKindCorrelatedPayloadLag {
			correlated = true
			assert.Equal(t, "ORDERS", f.Stream)
			assert.Equal(t, "billing-worker", f.Consumer)
			assert.Contains(t, f.Schedule, "Friday")
			assert.Contains(t, f.Schedule, "18:00")
		}
	}
	assert.True(t, correlated, "expected correlated_payload_lag finding")
}

func TestDiscoverHiddenBottlenecksEmpty(t *testing.T) {
	t.Parallel()
	snap := domain.DiscoverHiddenBottlenecks(nil)
	assert.Equal(t, domain.BottleneckVerdictHealthy, snap.Verdict)
	assert.Empty(t, snap.Findings)
}

func TestDemoHiddenBottlenecksSnapshot(t *testing.T) {
	t.Parallel()
	snap := domain.DemoHiddenBottlenecksSnapshot()
	assert.True(t, snap.Demo)
	assert.NotEmpty(t, snap.Findings)
	assert.Equal(t, domain.BottleneckKindCorrelatedPayloadLag, snap.Findings[0].Kind)
}

func TestParseStreamMetricAvgPayload(t *testing.T) {
	t.Parallel()
	stream, kind, ok := domain.ParseStreamMetric("stream:ORDERS:avg_payload_bytes")
	assert.True(t, ok)
	assert.Equal(t, "ORDERS", stream)
	assert.Equal(t, domain.StreamMetricKindAvgPayloadBytes, kind)
	assert.True(t, domain.IsStreamGaugeMetricKind(kind))
	assert.False(t, domain.IsCounterMetric("stream:ORDERS:avg_payload_bytes"))
	assert.True(t, domain.ValidHistoryMetricName("stream:ORDERS:avg_payload_bytes"))
}
