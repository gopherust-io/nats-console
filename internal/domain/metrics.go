package domain

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	MetricJetStreamStorageBytes          = "jetstream.storage_bytes"
	MetricJetStreamMemoryBytes           = "jetstream.memory_bytes"
	MetricJetStreamStreams               = "jetstream.streams"
	MetricJetStreamConsumers             = "jetstream.consumers"
	MetricJetStreamSlowConsumers         = "jetstream.slow_consumers"
	MetricJetStreamConsumerMaxLag        = "jetstream.consumer_max_lag"
	MetricJetStreamConsumerMaxPending    = "jetstream.consumer_max_pending"
	MetricJetStreamConsumerMaxAckPending = "jetstream.consumer_max_ack_pending"
	MetricJSZMessages                    = "jsz.messages"
	MetricJSZStreams                     = "jsz.streams"
	MetricJSZConsumers                   = "jsz.consumers"
	MetricServerConnections              = "server.connections"
	MetricServerInMsgsTotal              = "server.in_msgs_total"
	MetricServerOutMsgsTotal             = "server.out_msgs_total"
	MetricServerInBytesTotal             = "server.in_bytes_total"
	MetricServerOutBytesTotal            = "server.out_bytes_total"
	MetricServerCPUPercent               = "server.cpu_percent"
	MetricServerMemBytes                 = "server.mem_bytes"

	StreamMetricKindLastSeq         = "last_seq"
	StreamMetricKindBytes           = "bytes"
	StreamMetricKindDeliveredSeq    = "delivered_seq"
	StreamMetricKindAckFloorSeq     = "ack_floor_seq"
	StreamMetricKindAvgPayloadBytes = "avg_payload_bytes"
)

var CounterMetrics = map[string]bool{
	MetricServerInMsgsTotal:   true,
	MetricServerOutMsgsTotal:  true,
	MetricServerInBytesTotal:  true,
	MetricServerOutBytesTotal: true,
}

var streamMetricKinds = map[string]bool{
	StreamMetricKindLastSeq:      true,
	StreamMetricKindBytes:        true,
	StreamMetricKindDeliveredSeq: true,
	StreamMetricKindAckFloorSeq:  true,
}

var streamMetricGaugeKinds = map[string]bool{
	StreamMetricKindAvgPayloadBytes: true,
}

// streamMetricRE matches stream:{name}:{kind} where name is a JetStream stream name.
var streamMetricRE = regexp.MustCompile(`^stream:([A-Za-z0-9_/-]+):(last_seq|bytes|delivered_seq|ack_floor_seq|avg_payload_bytes)$`)

var DefaultDashboardMetrics = []string{
	MetricJetStreamStorageBytes,
	MetricJetStreamMemoryBytes,
	MetricJetStreamStreams,
	MetricJetStreamConsumers,
	MetricJetStreamSlowConsumers,
	MetricJetStreamConsumerMaxLag,
	MetricJetStreamConsumerMaxPending,
	MetricJetStreamConsumerMaxAckPending,
	MetricJSZMessages,
	MetricJSZStreams,
	MetricJSZConsumers,
	MetricServerConnections,
	MetricServerInMsgsTotal,
	MetricServerOutMsgsTotal,
	MetricServerInBytesTotal,
	MetricServerOutBytesTotal,
	MetricServerCPUPercent,
	MetricServerMemBytes,
}

type MetricSample struct {
	Metric string
	Value  float64
}

type MetricPoint struct {
	T time.Time `json:"t"`
	V float64   `json:"v"`
}

type MetricSeries struct {
	Metric string        `json:"metric"`
	Points []MetricPoint `json:"points"`
}

type MetricsHistoryResponse struct {
	ClusterID string         `json:"clusterId"`
	From      time.Time      `json:"from"`
	To        time.Time      `json:"to"`
	Series    []MetricSeries `json:"series"`
}

// StreamMetric builds a per-stream history metric name: stream:{name}:{kind}.
func StreamMetric(streamName, kind string) string {
	return "stream:" + streamName + ":" + kind
}

// ParseStreamMetric returns stream name and kind for stream:{name}:{kind} metrics.
func ParseStreamMetric(metric string) (streamName, kind string, ok bool) {
	m := streamMetricRE.FindStringSubmatch(metric)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

func IsStreamRateMetricKind(kind string) bool {
	return streamMetricKinds[kind]
}

func IsStreamGaugeMetricKind(kind string) bool {
	return streamMetricGaugeKinds[kind]
}

func IsCounterMetric(name string) bool {
	if CounterMetrics[name] {
		return true
	}
	_, kind, ok := ParseStreamMetric(name)
	return ok && IsStreamRateMetricKind(kind)
}

// AvgPayloadBytes returns Δbytes/Δmsgs when messages advanced; otherwise ok=false.
func AvgPayloadBytes(deltaBytes, deltaMsgs float64) (float64, bool) {
	if deltaMsgs <= 0 || deltaBytes < 0 {
		return 0, false
	}
	return deltaBytes / deltaMsgs, true
}

// ValidMetricName is the alert-rule allowlist (cluster metrics only).
func ValidMetricName(name string) bool {
	return slices.Contains(DefaultDashboardMetrics, name)
}

// ValidHistoryMetricName accepts dashboard metrics, per-stream rate counters, or probe latency series.
func ValidHistoryMetricName(name string) bool {
	if ValidMetricName(name) {
		return true
	}
	_, kind, ok := ParseStreamMetric(name)
	if ok && (IsStreamRateMetricKind(kind) || IsStreamGaugeMetricKind(kind)) {
		return true
	}
	_, ok = ParseRequestReplyProbeMetric(name)
	return ok
}

// StreamRateMetricsCSV returns the comma-separated metrics query for a stream's rate charts.
func StreamRateMetricsCSV(streamName string) string {
	parts := []string{
		StreamMetric(streamName, StreamMetricKindLastSeq),
		StreamMetric(streamName, StreamMetricKindDeliveredSeq),
		StreamMetric(streamName, StreamMetricKindAckFloorSeq),
		StreamMetric(streamName, StreamMetricKindBytes),
	}
	return strings.Join(parts, ",")
}

func ParseMetricsStep(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return 0, nil
	}
	switch raw {
	case "1m":
		return time.Minute, nil
	case "5m":
		return 5 * time.Minute, nil
	case "15m":
		return 15 * time.Minute, nil
	case "1h":
		return time.Hour, nil
	case "1d":
		return 24 * time.Hour, nil
	default:
		return 0, errors.New("unsupported step " + strconv.Quote(raw))
	}
}

func DefaultMetricsStep(from, to time.Time) time.Duration {
	rangeDur := to.Sub(from)
	switch {
	case rangeDur <= 2*time.Hour:
		return time.Minute
	case rangeDur <= 24*time.Hour:
		return 5 * time.Minute
	case rangeDur <= 7*24*time.Hour:
		return time.Hour
	default:
		return 24 * time.Hour
	}
}
