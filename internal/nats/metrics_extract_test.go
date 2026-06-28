package natsclient

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractVarzMetrics(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"connections": 12,
		"in_msgs": 1000,
		"out_msgs": 2000,
		"in_bytes": 3000,
		"out_bytes": 4000,
		"cpu": 1.5,
		"mem": 50000000
	}`)
	samples, err := ExtractVarzMetrics(raw)
	require.NoError(t, err)
	assert.Len(t, samples, 7)
	assert.Equal(t, float64(12), sampleValue(samples, domain.MetricServerConnections))
	assert.Equal(t, float64(1000), sampleValue(samples, domain.MetricServerInMsgsTotal))
}

func TestExtractJSZMetrics(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"total":{"streams":3,"consumers":4,"messages":99}}`)
	samples, err := ExtractJSZMetrics(raw)
	require.NoError(t, err)
	assert.Equal(t, float64(99), sampleValue(samples, domain.MetricJSZMessages))
}

func TestExtractAccountMetrics(t *testing.T) {
	t.Parallel()

	info := &nats.AccountInfo{
		Tier: nats.Tier{
			Store:     2048,
			Memory:    1024,
			Streams:   2,
			Consumers: 5,
		},
	}
	samples := ExtractAccountMetrics(info)
	assert.Equal(t, float64(2048), sampleValue(samples, domain.MetricJetStreamStorageBytes))
	assert.Equal(t, float64(5), sampleValue(samples, domain.MetricJetStreamConsumers))
}

func sampleValue(samples []store.MetricSampleRow, metric string) float64 {
	for _, s := range samples {
		if s.Metric == metric {
			return s.Value
		}
	}
	return 0
}
