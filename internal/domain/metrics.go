package domain

import (
	"slices"
	"time"
)

const (
	MetricJetStreamStorageBytes = "jetstream.storage_bytes"
	MetricJetStreamMemoryBytes  = "jetstream.memory_bytes"
	MetricJetStreamStreams      = "jetstream.streams"
	MetricJetStreamConsumers    = "jetstream.consumers"
	MetricJSZMessages           = "jsz.messages"
	MetricJSZStreams            = "jsz.streams"
	MetricJSZConsumers          = "jsz.consumers"
	MetricServerConnections     = "server.connections"
	MetricServerInMsgsTotal     = "server.in_msgs_total"
	MetricServerOutMsgsTotal    = "server.out_msgs_total"
	MetricServerInBytesTotal    = "server.in_bytes_total"
	MetricServerOutBytesTotal   = "server.out_bytes_total"
	MetricServerCPUPercent      = "server.cpu_percent"
	MetricServerMemBytes        = "server.mem_bytes"
)

var CounterMetrics = map[string]bool{
	MetricServerInMsgsTotal:  true,
	MetricServerOutMsgsTotal: true,
	MetricServerInBytesTotal: true,
	MetricServerOutBytesTotal: true,
}

var DefaultDashboardMetrics = []string{
	MetricJetStreamStorageBytes,
	MetricJetStreamMemoryBytes,
	MetricJetStreamStreams,
	MetricJetStreamConsumers,
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

func IsCounterMetric(name string) bool {
	return CounterMetrics[name]
}

func ValidMetricName(name string) bool {
	return slices.Contains(DefaultDashboardMetrics, name)
}
