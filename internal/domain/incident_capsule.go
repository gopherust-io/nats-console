package domain

import (
	"errors"
	"strings"
	"time"

	libnats "github.com/gopherust-io/nats"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const (
	DefaultIncidentCapsuleBucket = libnats.DefaultIncidentCapsuleBucket
	DefaultIncidentIndexBucket   = libnats.DefaultIncidentIndexBucket
	DefaultCapsuleWindow         = 50
	MaxCapsuleWindow             = 500
	MaxCapsulePreviewBytes       = 512
	MaxCapsulePreviewMessages    = 20
)

var (
	ErrCapsuleConsumerRequired     = errors.New("consumer is required")
	ErrCapsuleSourceStreamRequired = errors.New("source stream is required for DLQ capture")
	ErrCapsuleSourceSeqRequired    = errors.New("source sequence is required for DLQ capture")
	ErrCapsuleIDRequired           = errors.New("capsule id is required")
)

// IncidentCapsuleCaptureRequest captures a forensic pack around a failing sequence.
type IncidentCapsuleCaptureRequest struct {
	Consumer   string `json:"consumer"`
	Subject    string `json:"subject,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Trigger    string `json:"trigger,omitempty"`
	FailingSeq uint64 `json:"failingSeq,omitempty"`
	Window     int    `json:"window,omitempty"`
}

// Validate checks a stream-path capture request.
func (r IncidentCapsuleCaptureRequest) Validate() error {
	if commonstrings.IsEmpty(strings.TrimSpace(r.Consumer)) {
		return ErrCapsuleConsumerRequired
	}
	if r.Window < 0 || r.Window > MaxCapsuleWindow {
		return errors.New("window must be between 0 and 500")
	}
	return nil
}

// NormalizedWindow returns the capture window with defaults.
func (r IncidentCapsuleCaptureRequest) NormalizedWindow() int {
	if r.Window <= 0 {
		return DefaultCapsuleWindow
	}
	return r.Window
}

// IncidentCapsuleSummary is a list/index entry.
type IncidentCapsuleSummary struct {
	ID         string    `json:"id"`
	Stream     string    `json:"stream,omitempty"`
	Consumer   string    `json:"consumer,omitempty"`
	Trigger    string    `json:"trigger,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	FailingSeq uint64    `json:"failingSeq,omitempty"`
	CreatedAt  time.Time `json:"createdAt,omitzero"`
}

// IncidentCapsuleMessage is one message inside a loaded capsule (API view).
type IncidentCapsuleMessage struct {
	Time      time.Time         `json:"time"`
	Headers   map[string]string `json:"headers,omitempty"`
	Subject   string            `json:"subject"`
	Data      string            `json:"data"`
	Sequence  uint64            `json:"sequence"`
	Truncated bool              `json:"truncated,omitempty"`
}

// IncidentCapsuleDetail is a loaded capsule for the console.
type IncidentCapsuleDetail struct {
	CreatedAt      time.Time                `json:"createdAt"`
	ID             string                   `json:"id"`
	Stream         string                   `json:"stream"`
	Consumer       string                   `json:"consumer"`
	Trigger        string                   `json:"trigger"`
	Subject        string                   `json:"subject,omitempty"`
	Reason         string                   `json:"reason,omitempty"`
	Messages       []IncidentCapsuleMessage `json:"messages"`
	FlightTimeline []IncidentEventPreview   `json:"flightTimeline,omitempty"`
	FailingSeq     uint64                   `json:"failingSeq,omitempty"`
	SchemaVersion  int                      `json:"schemaVersion"`
	MessageCount   int                      `json:"messageCount"`
	HasFingerprint bool                     `json:"hasFingerprint,omitempty"`
}

// IncidentEventPreview is a truncated flight-recorder event.
type IncidentEventPreview struct {
	At      time.Time `json:"at"`
	Kind    string    `json:"kind"`
	Subject string    `json:"subject,omitempty"`
	Detail  string    `json:"detail,omitempty"`
}

// IncidentCapsuleDryRun is a safe preview of ReplayLocal (no business handlers).
type IncidentCapsuleDryRun struct {
	ID           string                   `json:"id"`
	Stream       string                   `json:"stream"`
	Consumer     string                   `json:"consumer"`
	FailingSeq   uint64                   `json:"failingSeq,omitempty"`
	MessageCount int                      `json:"messageCount"`
	Subjects     []string                 `json:"subjects"`
	Preview      []IncidentCapsuleMessage `json:"preview"`
	Invoked      int                      `json:"invoked"`
}

// CapsuleBuckets returns store/index bucket names with defaults.
func CapsuleBuckets(store, index string) (string, string) {
	if commonstrings.IsEmpty(strings.TrimSpace(store)) {
		store = DefaultIncidentCapsuleBucket
	}
	if commonstrings.IsEmpty(strings.TrimSpace(index)) {
		index = DefaultIncidentIndexBucket
	}
	return store, index
}
