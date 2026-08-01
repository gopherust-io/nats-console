package domain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"strings"
	"time"
	"unicode"
)

const RequestReplyProbeMetricPrefix = "request_reply.probe_latency_ms:"

// Probe subject / payload limits applied before CRUD or background runs.
const (
	DefaultRequestReplyTimeoutMs    = 2000
	MaxRequestReplyTimeoutMs        = 30_000
	MaxRequestReplyPayloadBytes     = 64 * 1024
	MaxRequestReplyReplyPreview     = 16 * 1024
	MaxRequestReplyProbesPerCluster = 50
)

// RequestReplyPayloadFormat is the wire format for probe requests.
type RequestReplyPayloadFormat string

func (f RequestReplyPayloadFormat) String() string {
	return string(f)
}

const (
	RequestReplyFormatJSON     RequestReplyPayloadFormat = "json"
	RequestReplyFormatMsgPack  RequestReplyPayloadFormat = "msgpack"
	RequestReplyFormatProtobuf RequestReplyPayloadFormat = "protobuf"
	RequestReplyFormatRaw      RequestReplyPayloadFormat = "raw"
)

// Normalize returns a known format, defaulting empty to json.
func (f RequestReplyPayloadFormat) Normalize() RequestReplyPayloadFormat {
	switch RequestReplyPayloadFormat(strings.ToLower(strings.TrimSpace(f.String()))) {
	case RequestReplyFormatMsgPack:
		return RequestReplyFormatMsgPack
	case RequestReplyFormatProtobuf:
		return RequestReplyFormatProtobuf
	case RequestReplyFormatRaw:
		return RequestReplyFormatRaw
	case RequestReplyFormatJSON, "":
		return RequestReplyFormatJSON
	default:
		return ""
	}
}

func (f RequestReplyPayloadFormat) Valid() bool {
	return !commonstrings.IsEmpty(string(f.Normalize()))
}

// RequestReplySnapshot is the live request/reply inspector payload.
type RequestReplySnapshot struct {
	CapturedAt  time.Time                `json:"capturedAt,omitzero"`
	MedianRttMs *float64                 `json:"medianRttMs,omitempty"`
	MaxProbeMs  *float64                 `json:"maxProbeMs,omitempty"`
	Patterns    []RequestReplyPattern    `json:"patterns"`
	Connections []RequestReplyConnection `json:"connections"`
	Requesters  int                      `json:"requesters"`
	Responders  int                      `json:"responders"`
}

// RequestReplyPattern groups responder subscriptions with latency stats.
type RequestReplyPattern struct {
	RttMinMs       *float64 `json:"rttMinMs,omitempty"`
	RttMedianMs    *float64 `json:"rttMedianMs,omitempty"`
	RttMaxMs       *float64 `json:"rttMaxMs,omitempty"`
	ProbeLatencyMs *float64 `json:"probeLatencyMs,omitempty"`
	ProbeOk        *bool    `json:"probeOk,omitempty"`
	Subject        string   `json:"subject"`
	Queue          string   `json:"queue,omitempty"`
	ProbeError     string   `json:"probeError,omitempty"`
	RequesterCount int      `json:"requesterCount"`
	ResponderCount int      `json:"responderCount"`
}

// RequestReplyConnection is a connz row participating in request/reply.
type RequestReplyConnection struct {
	RttMs        *float64 `json:"rttMs,omitempty"`
	Name         string   `json:"name,omitempty"`
	Account      string   `json:"account,omitempty"`
	InboxSubs    []string `json:"inboxSubs,omitempty"`
	ServiceSubs  []string `json:"serviceSubs,omitempty"`
	CID          int      `json:"cid"`
	PendingBytes int      `json:"pendingBytes,omitempty"`
}

// RequestReplyProbe is persisted probe configuration.
type RequestReplyProbe struct {
	CreatedAt     time.Time                 `json:"createdAt"`
	UpdatedAt     time.Time                 `json:"updatedAt"`
	ID            string                    `json:"id"`
	ClusterID     string                    `json:"clusterId"`
	Subject       string                    `json:"subject"`
	PayloadB64    string                    `json:"payloadB64,omitempty"`
	PayloadFormat RequestReplyPayloadFormat `json:"payloadFormat"`
	TimeoutMs     int                       `json:"timeoutMs"`
	Enabled       bool                      `json:"enabled"`
}

// WithoutPayload returns a copy with payload cleared (safe for list DTOs).
func (p RequestReplyProbe) WithoutPayload() RequestReplyProbe {
	p.PayloadB64 = ""
	return p
}

// RequestReplyProbeCreate creates a probe.
type RequestReplyProbeCreate struct {
	Enabled       *bool                     `json:"enabled"`
	Subject       string                    `json:"subject"`
	PayloadB64    string                    `json:"payloadB64"`
	PayloadFormat RequestReplyPayloadFormat `json:"payloadFormat"`
	TimeoutMs     int                       `json:"timeoutMs"`
}

// RequestReplyProbeUpdate updates a probe.
type RequestReplyProbeUpdate struct {
	Subject       *string                    `json:"subject"`
	PayloadB64    *string                    `json:"payloadB64"`
	PayloadFormat *RequestReplyPayloadFormat `json:"payloadFormat"`
	TimeoutMs     *int                       `json:"timeoutMs"`
	Enabled       *bool                      `json:"enabled"`
}

// RequestReplyProbeRunResult is returned by on-demand probe execution.
type RequestReplyProbeRunResult struct {
	MeasuredAt   time.Time         `json:"measuredAt"`
	ReplyHeaders map[string]string `json:"replyHeaders,omitempty"`
	Subject      string            `json:"subject"`
	Error        string            `json:"error,omitempty"`
	ReplyDataB64 string            `json:"replyDataB64,omitempty"`
	ReplyFormat  string            `json:"replyFormat,omitempty"`
	LatencyMs    float64           `json:"latencyMs"`
	OK           bool              `json:"ok"`
}

// RequestReplyProbeResult caches the latest probe measurement.
type RequestReplyProbeResult struct {
	MeasuredAt time.Time `json:"measuredAt"`
	Subject    string    `json:"subject"`
	Error      string    `json:"error,omitempty"`
	LatencyMs  float64   `json:"latencyMs"`
	OK         bool      `json:"ok"`
}

// CanonicalProbeSubject trims and validates a probe subject.
func CanonicalProbeSubject(subject string) (string, error) {
	subject = strings.TrimSpace(subject)
	if commonstrings.IsEmpty(subject) {
		return "", errors.New("subject is required")
	}
	if err := ValidateProbeSubject(subject); err != nil {
		return "", err
	}
	return subject, nil
}

// ValidateProbeSubject rejects wildcards, reserved prefixes, and malformed tokens.
func ValidateProbeSubject(subject string) error {
	if strings.ContainsAny(subject, "*>") {
		return errors.New("probe subject cannot contain wildcards")
	}
	upper := strings.ToUpper(subject)
	switch {
	case strings.HasPrefix(upper, "$JS."):
		return errors.New("JetStream API subjects cannot be probed")
	case strings.HasPrefix(upper, "$SYS."):
		return errors.New("system subjects cannot be probed")
	case strings.HasPrefix(upper, "_INBOX."):
		return errors.New("inbox subjects cannot be probed")
	case strings.HasPrefix(subject, "$"):
		return errors.New("reserved $ subjects cannot be probed")
	}
	for part := range strings.SplitSeq(subject, ".") {
		if commonstrings.IsEmpty(part) {
			return errors.New("probe subject cannot contain empty tokens")
		}
		for _, r := range part {
			if unicode.IsSpace(r) {
				return errors.New("probe subject cannot contain whitespace in tokens")
			}
		}
	}
	return nil
}

// NormalizeProbeTimeoutMs clamps timeout into the allowed range.
func NormalizeProbeTimeoutMs(timeoutMs int) (int, error) {
	if timeoutMs < 0 {
		return 0, errors.New("timeoutMs must be non-negative")
	}
	if timeoutMs == 0 {
		return DefaultRequestReplyTimeoutMs, nil
	}
	if timeoutMs > MaxRequestReplyTimeoutMs {
		return 0, fmt.Errorf("timeoutMs must be <= %d", MaxRequestReplyTimeoutMs)
	}
	return timeoutMs, nil
}

func (in RequestReplyProbeCreate) Validate() error {
	if _, err := CanonicalProbeSubject(in.Subject); err != nil {
		return err
	}
	if _, err := NormalizeProbeTimeoutMs(in.TimeoutMs); err != nil {
		return err
	}
	format := in.PayloadFormat.Normalize()
	if commonstrings.IsEmpty(string(format)) {
		return errors.New("payloadFormat must be json, msgpack, protobuf, or raw")
	}
	return validateProbePayload(format, in.PayloadB64)
}

// ValidateUpdate validates a partial update against the current stored format when needed.
func (in RequestReplyProbeUpdate) ValidateUpdate(currentFormat RequestReplyPayloadFormat) error {
	if in.Subject != nil {
		if _, err := CanonicalProbeSubject(*in.Subject); err != nil {
			return err
		}
	}
	if in.TimeoutMs != nil {
		if _, err := NormalizeProbeTimeoutMs(*in.TimeoutMs); err != nil {
			return err
		}
	}
	format := currentFormat.Normalize()
	if commonstrings.IsEmpty(string(format)) {
		format = RequestReplyFormatJSON
	}
	if in.PayloadFormat != nil {
		format = in.PayloadFormat.Normalize()
		if commonstrings.IsEmpty(string(format)) {
			return errors.New("payloadFormat must be json, msgpack, protobuf, or raw")
		}
	}
	if in.PayloadB64 != nil {
		return validateProbePayload(format, *in.PayloadB64)
	}
	return nil
}

// Validate is kept for callers that do not have the current format; payload-only
// updates without PayloadFormat are validated as JSON (prefer ValidateUpdate).
func (in RequestReplyProbeUpdate) Validate() error {
	return in.ValidateUpdate(RequestReplyFormatJSON)
}

func validateProbePayload(format RequestReplyPayloadFormat, payloadB64 string) error {
	if commonstrings.IsEmpty(payloadB64) {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(payloadB64)
	if err != nil {
		return errors.New("payloadB64 must be valid base64")
	}
	if len(raw) > MaxRequestReplyPayloadBytes {
		return fmt.Errorf("payload must be <= %d bytes", MaxRequestReplyPayloadBytes)
	}
	switch format.Normalize() {
	case RequestReplyFormatJSON:
		if len(raw) == 0 {
			return nil
		}
		if !json.Valid(raw) {
			return errors.New("payload must be valid JSON for json format")
		}
	case RequestReplyFormatMsgPack, RequestReplyFormatProtobuf, RequestReplyFormatRaw:
		// Opaque binary after base64 decode.
	}
	return nil
}

// TruncateReplyPreview caps reply bytes returned to API clients.
func TruncateReplyPreview(data []byte) []byte {
	if len(data) <= MaxRequestReplyReplyPreview {
		return data
	}
	out := make([]byte, MaxRequestReplyReplyPreview)
	copy(out, data[:MaxRequestReplyReplyPreview])
	return out
}

// RequestReplyProbeMetric builds a history metric name for a probe subject.
func RequestReplyProbeMetric(subject string) string {
	return RequestReplyProbeMetricPrefix + sanitizeProbeSubject(subject)
}

// ParseRequestReplyProbeMetric returns the subject encoded in a probe metric name.
func ParseRequestReplyProbeMetric(metric string) (subject string, ok bool) {
	if !strings.HasPrefix(metric, RequestReplyProbeMetricPrefix) {
		return "", false
	}
	subject = strings.TrimPrefix(metric, RequestReplyProbeMetricPrefix)
	if commonstrings.IsEmpty(subject) {
		return "", false
	}
	return subject, true
}

func sanitizeProbeSubject(subject string) string {
	subject = strings.TrimSpace(subject)
	replacer := strings.NewReplacer(":", "_", ",", "_", " ", "_")
	return replacer.Replace(subject)
}
