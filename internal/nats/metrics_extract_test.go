package natsclient

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractVarzMetrics(t *testing.T) {
	t.Parallel()

	raw := strings.StringToBytes(`{
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

	// Shape matches nats-server JSInfo monitor JSON (top-level counts; total is an int).
	raw := strings.StringToBytes(`{"streams":3,"consumers":4,"messages":99,"total":1}`)
	samples, err := ExtractJSZMetrics(raw)
	require.NoError(t, err)
	assert.Equal(t, float64(3), sampleValue(samples, domain.MetricJSZStreams))
	assert.Equal(t, float64(4), sampleValue(samples, domain.MetricJSZConsumers))
	assert.Equal(t, float64(99), sampleValue(samples, domain.MetricJSZMessages))
}

func TestExtractStreamRateMetrics(t *testing.T) {
	t.Parallel()

	raw := strings.StringToBytes(`{
		"account_details": [{
			"name": "ACC",
			"stream_detail": [
				{
					"name": "ORDERS",
					"state": {"messages": 10, "bytes": 2048, "last_seq": 100},
					"consumer_detail": [
						{
							"name": "worker",
							"delivered": {"consumer_seq": 40, "stream_seq": 90},
							"ack_floor": {"consumer_seq": 35, "stream_seq": 85}
						},
						{
							"name": "audit",
							"delivered": {"consumer_seq": 10, "stream_seq": 50},
							"ack_floor": {"consumer_seq": 8, "stream_seq": 48}
						}
					]
				},
				{
					"name": "EMPTY",
					"state": {"messages": 0, "bytes": 0, "last_seq": 0},
					"consumer_detail": []
				}
			]
		}]
	}`)
	samples, err := ExtractStreamRateMetrics(raw)
	require.NoError(t, err)

	assert.Equal(t, float64(100), sampleValue(samples, domain.StreamMetric("ORDERS", domain.StreamMetricKindLastSeq)))
	assert.Equal(t, float64(2048), sampleValue(samples, domain.StreamMetric("ORDERS", domain.StreamMetricKindBytes)))
	assert.Equal(t, float64(50), sampleValue(samples, domain.StreamMetric("ORDERS", domain.StreamMetricKindDeliveredSeq)))
	assert.Equal(t, float64(43), sampleValue(samples, domain.StreamMetric("ORDERS", domain.StreamMetricKindAckFloorSeq)))

	assert.Equal(t, float64(0), sampleValue(samples, domain.StreamMetric("EMPTY", domain.StreamMetricKindLastSeq)))
	assert.Equal(t, float64(0), sampleValue(samples, domain.StreamMetric("EMPTY", domain.StreamMetricKindDeliveredSeq)))
}

func TestExtractConsumerHealthMetrics(t *testing.T) {
	t.Parallel()

	raw := strings.StringToBytes(`{
		"account_details": [{
			"stream_detail": [{
				"name": "ORDERS",
				"state": {"last_seq": 3000},
				"consumer_detail": [
					{
						"name": "slow",
						"num_pending": 2000,
						"num_ack_pending": 950,
						"delivered": {"stream_seq": 90},
						"config": {"max_ack_pending": 1000}
					},
					{
						"name": "ok",
						"num_pending": 1,
						"num_ack_pending": 0,
						"delivered": {"stream_seq": 2999},
						"config": {"max_ack_pending": 1000}
					}
				]
			}]
		}]
	}`)
	samples, err := ExtractConsumerHealthMetrics(raw, domain.SlowConsumerThresholds{})
	require.NoError(t, err)
	assert.Equal(t, float64(1), sampleValue(samples, domain.MetricJetStreamSlowConsumers))
	assert.Equal(t, float64(2910), sampleValue(samples, domain.MetricJetStreamConsumerMaxLag))
	assert.Equal(t, float64(2000), sampleValue(samples, domain.MetricJetStreamConsumerMaxPending))
	assert.Equal(t, float64(950), sampleValue(samples, domain.MetricJetStreamConsumerMaxAckPending))
}

func TestExtractIncidentConsumerSamples(t *testing.T) {
	t.Parallel()

	raw := strings.StringToBytes(`{
		"account_details": [{
			"stream_detail": [{
				"name": "ORDERS",
				"state": {"last_seq": 3000},
				"consumer_detail": [
					{
						"name": "billing-worker",
						"num_redelivered": 12,
						"delivered": {"stream_seq": 90},
						"ack_floor": {"stream_seq": 80}
					},
					{
						"config": {"durable_name": "from-config"},
						"num_redelivered": 1,
						"delivered": {"stream_seq": 2990},
						"ack_floor": {"stream_seq": 2980}
					}
				]
			}]
		}]
	}`)
	samples, err := ExtractIncidentConsumerSamples(raw)
	require.NoError(t, err)
	require.Len(t, samples, 2)
	assert.Equal(t, "ORDERS", samples[0].StreamName)
	assert.Equal(t, "billing-worker", samples[0].ConsumerName)
	assert.Equal(t, float64(2910), samples[0].Lag)
	assert.Equal(t, float64(12), samples[0].NumRedelivered)
	assert.Equal(t, float64(90), samples[0].DeliveredSeq)
	assert.Equal(t, float64(80), samples[0].AckFloorSeq)
	assert.Equal(t, "from-config", samples[1].ConsumerName)
}

func TestExtractRouteNodeNamesAndDiff(t *testing.T) {
	t.Parallel()

	raw := strings.StringToBytes(`{
		"server_name": "Node A",
		"routes": [
			{"remote_name": "Node B"},
			{"name": "Node C"},
			{"remote_id": "nid-d"}
		]
	}`)
	nodes, err := ExtractRouteNodeNames(raw)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Node A", "Node B", "Node C", "nid-d"}, nodes)

	disconnected, reconnected := DiffRouteNodes(
		[]string{"Node A", "Node B", "Node C"},
		[]string{"Node A", "Node C", "Node D"},
	)
	assert.Equal(t, []string{"Node B"}, disconnected)
	assert.Equal(t, []string{"Node D"}, reconnected)
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
