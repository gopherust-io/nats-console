package natsclient

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/nats-io/nats.go"
)

func ExtractAccountMetrics(info *nats.AccountInfo) []store.MetricSampleRow {
	if info == nil {
		return nil
	}
	return []store.MetricSampleRow{
		{Metric: domain.MetricJetStreamStorageBytes, Value: float64(info.Store)},
		{Metric: domain.MetricJetStreamMemoryBytes, Value: float64(info.Memory)},
		{Metric: domain.MetricJetStreamStreams, Value: float64(info.Streams)},
		{Metric: domain.MetricJetStreamConsumers, Value: float64(info.Consumers)},
	}
}

func ExtractVarzMetrics(raw []byte) ([]store.MetricSampleRow, error) {
	var payload struct {
		Connections int     `json:"connections"`
		InMsgs      int64   `json:"in_msgs"`
		OutMsgs     int64   `json:"out_msgs"`
		InBytes     int64   `json:"in_bytes"`
		OutBytes    int64   `json:"out_bytes"`
		CPU         float64 `json:"cpu"`
		Mem         int64   `json:"mem"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := []store.MetricSampleRow{
		{Metric: domain.MetricServerConnections, Value: float64(payload.Connections)},
		{Metric: domain.MetricServerInMsgsTotal, Value: float64(payload.InMsgs)},
		{Metric: domain.MetricServerOutMsgsTotal, Value: float64(payload.OutMsgs)},
		{Metric: domain.MetricServerInBytesTotal, Value: float64(payload.InBytes)},
		{Metric: domain.MetricServerOutBytesTotal, Value: float64(payload.OutBytes)},
	}
	if payload.CPU > 0 {
		out = append(out, store.MetricSampleRow{Metric: domain.MetricServerCPUPercent, Value: payload.CPU})
	}
	if payload.Mem > 0 {
		out = append(out, store.MetricSampleRow{Metric: domain.MetricServerMemBytes, Value: float64(payload.Mem)})
	}
	return out, nil
}

func ExtractJSZMetrics(raw []byte) ([]store.MetricSampleRow, error) {
	// NATS /jsz exposes streams/consumers/messages at the top level; `total` is an int
	// (account count), not a nested object.
	var payload struct {
		Streams   int    `json:"streams"`
		Consumers int    `json:"consumers"`
		Messages  uint64 `json:"messages"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return []store.MetricSampleRow{
		{Metric: domain.MetricJSZStreams, Value: float64(payload.Streams)},
		{Metric: domain.MetricJSZConsumers, Value: float64(payload.Consumers)},
		{Metric: domain.MetricJSZMessages, Value: float64(payload.Messages)},
	}, nil
}

func CollectClusterMetrics(client interface {
	AccountInfo(ctx context.Context) (*nats.AccountInfo, error)
	Monitoring(ctx context.Context, path string) ([]byte, error)
}, ctx context.Context) ([]store.MetricSampleRow, error) {
	result, err := CollectClusterSnapshot(client, ctx)
	if err != nil {
		return nil, err
	}
	return result.Samples, nil
}

// ClusterSnapshotResult holds normalized metrics plus raw monitoring payloads.
type ClusterSnapshotResult struct {
	Samples     []store.MetricSampleRow
	Varz        []byte
	Jsz         []byte
	JszTopology []byte
}

const topologyJSZPath = "/jsz?streams=1&consumers=1&config=1"

// CollectClusterSnapshot scrapes account + monitoring endpoints for metrics and hub reuse.
func CollectClusterSnapshot(client interface {
	AccountInfo(ctx context.Context) (*nats.AccountInfo, error)
	Monitoring(ctx context.Context, path string) ([]byte, error)
}, ctx context.Context) (ClusterSnapshotResult, error) {
	var out ClusterSnapshotResult

	if info, err := client.AccountInfo(ctx); err == nil {
		out.Samples = append(out.Samples, ExtractAccountMetrics(info)...)
	}

	if raw, err := client.Monitoring(ctx, "/varz"); err == nil {
		out.Varz = raw
		if samples, parseErr := ExtractVarzMetrics(raw); parseErr == nil {
			out.Samples = append(out.Samples, samples...)
		}
	}

	if raw, err := client.Monitoring(ctx, "/jsz"); err == nil {
		out.Jsz = raw
		if samples, parseErr := ExtractJSZMetrics(raw); parseErr == nil {
			out.Samples = append(out.Samples, samples...)
		}
	}

	if raw, err := client.Monitoring(ctx, topologyJSZPath); err == nil {
		out.JszTopology = raw
	}

	if len(out.Samples) == 0 && len(out.Varz) == 0 && len(out.Jsz) == 0 {
		return out, errors.New("no metrics collected")
	}
	return out, nil
}
