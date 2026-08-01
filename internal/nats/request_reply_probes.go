package natsclient

import (
	"context"
	"encoding/base64"
	"time"

	libnats "github.com/gopherust-io/nats"
	"github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/pkg/common/b64util"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type probeRunner interface {
	ProbeRequest(
		ctx context.Context,
		subject string,
		format domain.RequestReplyPayloadFormat,
		payload []byte,
		timeout time.Duration,
	) (*nats.Msg, time.Duration, error)
}

// ProbeRequestHeaders builds Nats-Content-Type headers for the given format.
func ProbeRequestHeaders(format domain.RequestReplyPayloadFormat, hasPayload bool) nats.Header {
	format = format.Normalize()
	if strings.IsEmpty(string(format)) {
		format = domain.RequestReplyFormatJSON
	}
	switch format {
	case domain.RequestReplyFormatRaw:
		return nil
	case domain.RequestReplyFormatMsgPack:
		return nats.Header{libnats.HeaderContentType: []string{libnats.ContentTypeMsgPack}}
	case domain.RequestReplyFormatProtobuf:
		return nats.Header{libnats.HeaderContentType: []string{libnats.ContentTypeProto}}
	case domain.RequestReplyFormatJSON:
		if !hasPayload {
			return nil
		}
		return nats.Header{libnats.HeaderContentType: []string{libnats.ContentTypeJSON}}
	default:
		return nil
	}
}

// DetectReplyFormat maps Nats-Content-Type (or Content-Type) to a reply format string.
func DetectReplyFormat(headers nats.Header) string {
	if headers == nil {
		return "json"
	}
	ct := headers.Get(libnats.HeaderContentType)
	if strings.IsEmpty(ct) {
		ct = headers.Get("Content-Type")
	}
	switch ct {
	case libnats.ContentTypeMsgPack, "application/msgpack", "application/x-msgpack":
		return "msgpack"
	case libnats.ContentTypeProto, "application/protobuf", "application/x-protobuf", "application/vnd.google.protobuf":
		return "protobuf"
	case libnats.ContentTypeJSON, "application/json", "":
		return "json"
	default:
		return "bytes"
	}
}

func headerMap(headers nats.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for k, vals := range headers {
		if len(vals) > 0 {
			out[k] = vals[0]
		}
	}
	return out
}

// RunRequestReplyProbe executes a single probe and returns a run result (with reply preview fields).
func RunRequestReplyProbe(
	ctx context.Context,
	client probeRunner,
	probe domain.RequestReplyProbe,
	measuredAt time.Time,
) (domain.RequestReplyProbeRunResult, *store.MetricSampleRow) {
	payload := decodeProbePayloadBytes(probe.PayloadB64)
	timeout := time.Duration(probe.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	format := probe.PayloadFormat.Normalize()
	if strings.IsEmpty(string(format)) {
		format = domain.RequestReplyFormatJSON
	}

	result := domain.RequestReplyProbeRunResult{
		Subject:    probe.Subject,
		MeasuredAt: measuredAt,
	}

	reply, latency, err := client.ProbeRequest(ctx, probe.Subject, format, payload, timeout)
	if err != nil {
		result.OK = false
		result.Error = err.Error()
		return result, nil
	}

	result.OK = true
	result.LatencyMs = float64(latency) / float64(time.Millisecond)
	if reply != nil {
		result.ReplyDataB64 = b64util.EncodeToString(domain.TruncateReplyPreview(reply.Data))
		result.ReplyHeaders = headerMap(reply.Header)
		result.ReplyFormat = DetectReplyFormat(reply.Header)
	}
	sample := &store.MetricSampleRow{
		Metric: domain.RequestReplyProbeMetric(probe.Subject),
		Value:  result.LatencyMs,
	}
	return result, sample
}

// RunRequestReplyProbes executes enabled probes and returns results plus metric samples.
func RunRequestReplyProbes(
	ctx context.Context,
	client probeRunner,
	probes []domain.RequestReplyProbe,
	measuredAt time.Time,
) ([]domain.RequestReplyProbeResult, []store.MetricSampleRow) {
	results := make([]domain.RequestReplyProbeResult, 0, len(probes))
	samples := make([]store.MetricSampleRow, 0, len(probes))

	for _, probe := range probes {
		if !probe.Enabled {
			continue
		}
		run, sample := RunRequestReplyProbe(ctx, client, probe, measuredAt)
		results = append(results, domain.RequestReplyProbeResult{
			Subject:    run.Subject,
			LatencyMs:  run.LatencyMs,
			OK:         run.OK,
			Error:      run.Error,
			MeasuredAt: run.MeasuredAt,
		})
		if sample != nil {
			samples = append(samples, *sample)
		}
	}

	return results, samples
}

func decodeProbePayloadBytes(payloadB64 string) []byte {
	if strings.IsEmpty(payloadB64) {
		return []byte{}
	}
	raw, err := base64.StdEncoding.DecodeString(payloadB64)
	if err != nil {
		return []byte{}
	}
	return raw
}
