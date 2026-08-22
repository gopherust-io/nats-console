package natsclient

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/repo"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"

	"github.com/nats-io/nats.go"
	"golang.org/x/sync/errgroup"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func ExtractAccountMetrics(info *nats.AccountInfo) []repo.MetricSampleRow {
	if info == nil {
		return nil
	}
	return []repo.MetricSampleRow{
		{Metric: domain.MetricJetStreamStorageBytes, Value: float64(info.Store)},
		{Metric: domain.MetricJetStreamMemoryBytes, Value: float64(info.Memory)},
		{Metric: domain.MetricJetStreamStreams, Value: float64(info.Streams)},
		{Metric: domain.MetricJetStreamConsumers, Value: float64(info.Consumers)},
	}
}

func ExtractVarzMetrics(raw []byte) ([]repo.MetricSampleRow, error) {
	var payload struct {
		Connections int     `json:"connections"`
		InMsgs      int64   `json:"in_msgs"`
		OutMsgs     int64   `json:"out_msgs"`
		InBytes     int64   `json:"in_bytes"`
		OutBytes    int64   `json:"out_bytes"`
		CPU         float64 `json:"cpu"`
		Mem         int64   `json:"mem"`
	}
	if err := serializer.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := []repo.MetricSampleRow{
		{Metric: domain.MetricServerConnections, Value: float64(payload.Connections)},
		{Metric: domain.MetricServerInMsgsTotal, Value: float64(payload.InMsgs)},
		{Metric: domain.MetricServerOutMsgsTotal, Value: float64(payload.OutMsgs)},
		{Metric: domain.MetricServerInBytesTotal, Value: float64(payload.InBytes)},
		{Metric: domain.MetricServerOutBytesTotal, Value: float64(payload.OutBytes)},
	}
	if payload.CPU > 0 {
		out = append(out, repo.MetricSampleRow{Metric: domain.MetricServerCPUPercent, Value: payload.CPU})
	}
	if payload.Mem > 0 {
		out = append(out, repo.MetricSampleRow{Metric: domain.MetricServerMemBytes, Value: float64(payload.Mem)})
	}
	return out, nil
}

func ExtractJSZMetrics(raw []byte) ([]repo.MetricSampleRow, error) {
	var payload struct {
		Streams   int    `json:"streams"`
		Consumers int    `json:"consumers"`
		Messages  uint64 `json:"messages"`
	}
	if err := serializer.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return []repo.MetricSampleRow{
		{Metric: domain.MetricJSZStreams, Value: float64(payload.Streams)},
		{Metric: domain.MetricJSZConsumers, Value: float64(payload.Consumers)},
		{Metric: domain.MetricJSZMessages, Value: float64(payload.Messages)},
	}, nil
}

// ExtractStreamRateMetrics derives per-stream counter samples from topology jsz
// (streams=1&consumers=1). Values are treated as counters for rate charts
func ExtractStreamRateMetrics(raw []byte) ([]repo.MetricSampleRow, error) {
	payload, err := parseTopologyJSZ(raw)
	if err != nil {
		return nil, err
	}
	return streamRateSamplesFromTopology(payload), nil
}

// ExtractConsumerHealthMetrics evaluates each consumer against slow-consumer
// thresholds and emits cluster-level aggregate gauges
func ExtractConsumerHealthMetrics(raw []byte, thr domain.SlowConsumerThresholds) ([]repo.MetricSampleRow, error) {
	payload, err := parseTopologyJSZ(raw)
	if err != nil {
		return nil, err
	}
	return consumerHealthSamplesFromTopology(payload, thr), nil
}

// ExtractEventArchitectureInputs builds stream/consumer inventory for architecture analysis
func ExtractEventArchitectureInputs(raw []byte) ([]domain.EventArchitectureInput, error) {
	payload, err := parseTopologyJSZ(raw)
	if err != nil {
		return nil, err
	}
	return architectureInputsFromTopology(payload), nil
}

// ExtractIncidentConsumerSamples builds per-consumer lag/redelivery/progress rows from topology JSZ
func ExtractIncidentConsumerSamples(raw []byte) ([]domain.IncidentConsumerSample, error) {
	payload, err := parseTopologyJSZ(raw)
	if err != nil {
		return nil, err
	}
	return incidentSamplesFromTopology(payload), nil
}

// ExtractRouteNodeNames returns unique remote route node names from /routez.
func ExtractRouteNodeNames(raw []byte) ([]string, error) {
	var payload struct {
		ServerName string `json:"server_name"`
		Routes     []struct {
			RemoteID   string `json:"remote_id"`
			RemoteName string `json:"remote_name"`
			Name       string `json:"name"`
		} `json:"routes"`
	}
	if err := serializer.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(payload.Routes)+1)
	add := func(name string) {
		name = strings.TrimSpace(name)
		if commonstrings.IsEmpty(name) {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	add(payload.ServerName)
	for _, r := range payload.Routes {
		if !commonstrings.IsEmpty(r.RemoteName) {
			add(r.RemoteName)
			continue
		}
		if !commonstrings.IsEmpty(r.Name) {
			add(r.Name)
			continue
		}
		add(r.RemoteID)
	}
	return out, nil
}

// DiffRouteNodes compares previous and current route membership and emits transitions.
func DiffRouteNodes(previous, current []string) (disconnected, reconnected []string) {
	prev := make(map[string]struct{}, len(previous))
	for _, n := range previous {
		n = strings.TrimSpace(n)
		if commonstrings.IsEmpty(n) {
			continue
		}
		prev[n] = struct{}{}
	}
	cur := make(map[string]struct{}, len(current))
	for _, n := range current {
		n = strings.TrimSpace(n)
		if commonstrings.IsEmpty(n) {
			continue
		}
		cur[n] = struct{}{}
		if _, ok := prev[n]; !ok {
			reconnected = append(reconnected, n)
		}
	}
	for n := range prev {
		if _, ok := cur[n]; !ok {
			disconnected = append(disconnected, n)
		}
	}
	sort.Strings(disconnected)
	sort.Strings(reconnected)
	return disconnected, reconnected
}

func CollectClusterMetrics(client interface {
	AccountInfo(ctx context.Context) (*nats.AccountInfo, error)
	Monitoring(ctx context.Context, path string) ([]byte, error)
}, ctx context.Context, thr domain.SlowConsumerThresholds) ([]repo.MetricSampleRow, error) {
	result, err := CollectClusterSnapshot(client, ctx, thr)
	if err != nil {
		return nil, err
	}
	return result.Samples, nil
}

// ClusterSnapshotResult holds normalized metrics plus raw monitoring payloads.
type ClusterSnapshotResult struct {
	Samples            []repo.MetricSampleRow
	ConsumerSamples    []domain.IncidentConsumerSample
	ArchitectureInputs []domain.EventArchitectureInput
	RouteNodes         []string
	Varz               []byte
	Jsz                []byte
	JszTopology        []byte
	Connz              []byte
	Routez             []byte
}

const topologyJSZPath = "/jsz?streams=1&consumers=1&config=1"
const routezPath = "/routez"

// topologyJSZPayload is the single unmarshal target for topology jsz scrapes.
type topologyJSZPayload struct {
	Streams        int    `json:"streams"`
	Consumers      int    `json:"consumers"`
	Messages       uint64 `json:"messages"`
	AccountDetails []struct {
		StreamDetail []struct {
			Name   string `json:"name"`
			Config *struct {
				Subjects []string `json:"subjects"`
			} `json:"config"`
			State *struct {
				Messages uint64 `json:"messages"`
				Bytes    uint64 `json:"bytes"`
				LastSeq  uint64 `json:"last_seq"`
			} `json:"state"`
			ConsumerDetail []struct {
				Name   string `json:"name"`
				Config *struct {
					FilterSubject  string   `json:"filter_subject"`
					FilterSubjects []string `json:"filter_subjects"`
					DurableName    string   `json:"durable_name"`
					MaxAckPending  int      `json:"max_ack_pending"`
				} `json:"config"`
				Delivered *struct {
					StreamSeq   uint64 `json:"stream_seq"`
					ConsumerSeq uint64 `json:"consumer_seq"`
				} `json:"delivered"`
				AckFloor *struct {
					StreamSeq   uint64 `json:"stream_seq"`
					ConsumerSeq uint64 `json:"consumer_seq"`
				} `json:"ack_floor"`
				NumPending     int `json:"num_pending"`
				NumAckPending  int `json:"num_ack_pending"`
				NumRedelivered int `json:"num_redelivered"`
			} `json:"consumer_detail"`
		} `json:"stream_detail"`
	} `json:"account_details"`
}

func parseTopologyJSZ(raw []byte) (*topologyJSZPayload, error) {
	var payload topologyJSZPayload
	if err := serializer.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func samplesFromTopologyJSZ(payload *topologyJSZPayload, thr domain.SlowConsumerThresholds) (
	samples []repo.MetricSampleRow,
	consumerSamples []domain.IncidentConsumerSample,
) {
	if payload == nil {
		return nil, nil
	}
	samples = append(samples,
		repo.MetricSampleRow{Metric: domain.MetricJSZStreams, Value: float64(payload.Streams)},
		repo.MetricSampleRow{Metric: domain.MetricJSZConsumers, Value: float64(payload.Consumers)},
		repo.MetricSampleRow{Metric: domain.MetricJSZMessages, Value: float64(payload.Messages)},
	)
	samples = append(samples, streamRateSamplesFromTopology(payload)...)
	samples = append(samples, consumerHealthSamplesFromTopology(payload, thr)...)
	consumerSamples = incidentSamplesFromTopology(payload)
	return samples, consumerSamples
}

func streamRateSamplesFromTopology(payload *topologyJSZPayload) []repo.MetricSampleRow {
	out := make([]repo.MetricSampleRow, 0)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			if commonstrings.IsEmpty(stream.Name) {
				continue
			}
			var lastSeq, bytes uint64
			if stream.State != nil {
				lastSeq = stream.State.LastSeq
				bytes = stream.State.Bytes
			}
			var deliveredSum, ackFloorSum uint64
			for _, c := range stream.ConsumerDetail {
				if c.Delivered != nil {
					deliveredSum += c.Delivered.ConsumerSeq
				}
				if c.AckFloor != nil {
					ackFloorSum += c.AckFloor.ConsumerSeq
				}
			}
			out = append(out,
				repo.MetricSampleRow{
					Metric: domain.StreamMetric(stream.Name, domain.StreamMetricKindLastSeq),
					Value:  float64(lastSeq),
				},
				repo.MetricSampleRow{
					Metric: domain.StreamMetric(stream.Name, domain.StreamMetricKindBytes),
					Value:  float64(bytes),
				},
				repo.MetricSampleRow{
					Metric: domain.StreamMetric(stream.Name, domain.StreamMetricKindDeliveredSeq),
					Value:  float64(deliveredSum),
				},
				repo.MetricSampleRow{
					Metric: domain.StreamMetric(stream.Name, domain.StreamMetricKindAckFloorSeq),
					Value:  float64(ackFloorSum),
				},
			)
		}
	}
	return out
}

func consumerHealthSamplesFromTopology(payload *topologyJSZPayload, thr domain.SlowConsumerThresholds) []repo.MetricSampleRow {
	thr = thr.WithDefaults()
	var (
		slowCount   float64
		maxLag      float64
		maxPending  float64
		maxAckPend  float64
		sawConsumer bool
	)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			var lastSeq uint64
			if stream.State != nil {
				lastSeq = stream.State.LastSeq
			}
			for _, c := range stream.ConsumerDetail {
				sawConsumer = true
				var delivered uint64
				if c.Delivered != nil {
					delivered = c.Delivered.StreamSeq
				}
				lag := domain.ConsumerLagMessages(lastSeq, delivered)
				pending := uint64(0)
				if c.NumPending > 0 {
					pending = uint64(c.NumPending)
				}
				maxAck := 0
				if c.Config != nil {
					maxAck = c.Config.MaxAckPending
				}
				if float64(lag) > maxLag {
					maxLag = float64(lag)
				}
				if float64(pending) > maxPending {
					maxPending = float64(pending)
				}
				if float64(c.NumAckPending) > maxAckPend {
					maxAckPend = float64(c.NumAckPending)
				}
				if slow, _ := domain.EvaluateSlowConsumer(pending, lag, c.NumAckPending, maxAck, thr); slow {
					slowCount++
				}
			}
		}
	}
	if !sawConsumer {
		return []repo.MetricSampleRow{
			{Metric: domain.MetricJetStreamSlowConsumers, Value: 0},
			{Metric: domain.MetricJetStreamConsumerMaxLag, Value: 0},
			{Metric: domain.MetricJetStreamConsumerMaxPending, Value: 0},
			{Metric: domain.MetricJetStreamConsumerMaxAckPending, Value: 0},
		}
	}
	return []repo.MetricSampleRow{
		{Metric: domain.MetricJetStreamSlowConsumers, Value: slowCount},
		{Metric: domain.MetricJetStreamConsumerMaxLag, Value: maxLag},
		{Metric: domain.MetricJetStreamConsumerMaxPending, Value: maxPending},
		{Metric: domain.MetricJetStreamConsumerMaxAckPending, Value: maxAckPend},
	}
}

func incidentSamplesFromTopology(payload *topologyJSZPayload) []domain.IncidentConsumerSample {
	out := make([]domain.IncidentConsumerSample, 0)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			if commonstrings.IsEmpty(stream.Name) {
				continue
			}
			var lastSeq uint64
			if stream.State != nil {
				lastSeq = stream.State.LastSeq
			}
			for _, c := range stream.ConsumerDetail {
				name := strings.TrimSpace(c.Name)
				if commonstrings.IsEmpty(name) && c.Config != nil {
					name = strings.TrimSpace(c.Config.DurableName)
				}
				if commonstrings.IsEmpty(name) {
					continue
				}
				var delivered, ackFloor uint64
				if c.Delivered != nil {
					delivered = c.Delivered.StreamSeq
				}
				if c.AckFloor != nil {
					ackFloor = c.AckFloor.StreamSeq
				}
				out = append(out, domain.IncidentConsumerSample{
					StreamName:     stream.Name,
					ConsumerName:   name,
					Lag:            float64(domain.ConsumerLagMessages(lastSeq, delivered)),
					NumRedelivered: float64(c.NumRedelivered),
					DeliveredSeq:   float64(delivered),
					AckFloorSeq:    float64(ackFloor),
				})
			}
		}
	}
	return out
}

func architectureInputsFromTopology(payload *topologyJSZPayload) []domain.EventArchitectureInput {
	out := make([]domain.EventArchitectureInput, 0)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			if commonstrings.IsEmpty(stream.Name) {
				continue
			}
			in := domain.EventArchitectureInput{Name: stream.Name}
			if stream.Config != nil {
				in.Subjects = append([]string(nil), stream.Config.Subjects...)
			}
			if stream.State != nil {
				in.Messages = stream.State.Messages
				in.Bytes = stream.State.Bytes
			}
			for _, c := range stream.ConsumerDetail {
				name := strings.TrimSpace(c.Name)
				if commonstrings.IsEmpty(name) && c.Config != nil {
					name = strings.TrimSpace(c.Config.DurableName)
				}
				if commonstrings.IsEmpty(name) {
					continue
				}
				cin := domain.EventArchitectureConsumerInput{Name: name}
				if c.Config != nil {
					cin.FilterSubject = c.Config.FilterSubject
					cin.FilterSubjects = append([]string(nil), c.Config.FilterSubjects...)
				}
				in.Consumers = append(in.Consumers, cin)
			}
			out = append(out, in)
		}
	}
	return out
}

func slimJSZFromTopology(payload *topologyJSZPayload) []byte {
	if payload == nil {
		return nil
	}
	// Fixed shape: {"streams":N,"consumers":N,"messages":N} — avoid Sonic on every scrape tick.
	buf := make([]byte, 0, 64)
	buf = append(buf, `{"streams":`...)
	buf = strconv.AppendInt(buf, int64(payload.Streams), 10)
	buf = append(buf, `,"consumers":`...)
	buf = strconv.AppendInt(buf, int64(payload.Consumers), 10)
	buf = append(buf, `,"messages":`...)
	buf = strconv.AppendUint(buf, payload.Messages, 10)
	return append(buf, '}')
}

// CollectClusterSnapshot scrapes account + monitoring endpoints for metrics and hub reuse.
func CollectClusterSnapshot(client interface {
	AccountInfo(ctx context.Context) (*nats.AccountInfo, error)
	Monitoring(ctx context.Context, path string) ([]byte, error)
}, ctx context.Context, thr domain.SlowConsumerThresholds) (ClusterSnapshotResult, error) {
	var out ClusterSnapshotResult

	type scrapeBuf struct {
		varz   []byte
		topo   []byte
		connz  []byte
		routez []byte
	}
	var (
		account *nats.AccountInfo
		bufs    scrapeBuf
	)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		info, err := client.AccountInfo(gctx)
		if err == nil {
			account = info
		}
		return nil
	})
	g.Go(func() error {
		raw, err := client.Monitoring(gctx, "/varz")
		if err == nil {
			bufs.varz = raw
		}
		return nil
	})
	g.Go(func() error {
		raw, err := client.Monitoring(gctx, topologyJSZPath)
		if err == nil {
			bufs.topo = raw
		}
		return nil
	})
	g.Go(func() error {
		raw, err := client.Monitoring(gctx, RequestReplyConnzPath)
		if err == nil {
			bufs.connz = raw
		}
		return nil
	})
	g.Go(func() error {
		raw, err := client.Monitoring(gctx, routezPath)
		if err == nil {
			bufs.routez = raw
		}
		return nil
	})
	_ = g.Wait()

	if account != nil {
		out.Samples = append(out.Samples, ExtractAccountMetrics(account)...)
	}
	if len(bufs.varz) > 0 {
		out.Varz = bufs.varz
		if samples, parseErr := ExtractVarzMetrics(bufs.varz); parseErr == nil {
			out.Samples = append(out.Samples, samples...)
		}
	}
	if len(bufs.topo) > 0 {
		out.JszTopology = bufs.topo
		if parsed, parseErr := parseTopologyJSZ(bufs.topo); parseErr == nil {
			samples, consumerSamples := samplesFromTopologyJSZ(parsed, thr)
			out.Samples = append(out.Samples, samples...)
			out.ConsumerSamples = consumerSamples
			out.ArchitectureInputs = architectureInputsFromTopology(parsed)
			out.Jsz = slimJSZFromTopology(parsed)
		}
	} else {
		// Topology miss: fall back to lightweight /jsz only.
		if raw, err := client.Monitoring(ctx, "/jsz"); err == nil {
			out.Jsz = raw
			if samples, parseErr := ExtractJSZMetrics(raw); parseErr == nil {
				out.Samples = append(out.Samples, samples...)
			}
		}
	}
	if len(bufs.connz) > 0 {
		out.Connz = bufs.connz
	}
	if len(bufs.routez) > 0 {
		out.Routez = bufs.routez
		if nodes, parseErr := ExtractRouteNodeNames(bufs.routez); parseErr == nil {
			out.RouteNodes = nodes
		}
	}

	if len(out.Samples) == 0 && len(out.Varz) == 0 && len(out.Jsz) == 0 && len(out.Connz) == 0 {
		return out, errors.New("no metrics collected")
	}
	return out, nil
}
