package domain

import (
	"encoding/json"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"strings"
	"time"
)

// DefaultBehaviorFingerprintKVBucket is where workers publish fingerprint snapshots.
const DefaultBehaviorFingerprintKVBucket = "nats_consol_fingerprints"

// BehaviorFingerprintSnapshot is msg/min + processing latency for consol UI.
type BehaviorFingerprintSnapshot struct {
	MsgPerMin    float64 `json:"msgPerMin"`
	ProcessingMs float64 `json:"processingMs"`
}

// BehaviorFingerprintReport is the GET .../behavior-fingerprint payload.
// goalign:ignore // trailing bool padding is unavoidable
type BehaviorFingerprintReport struct {
	Normal         *BehaviorFingerprintSnapshot `json:"normal,omitempty"`
	Current        *BehaviorFingerprintSnapshot `json:"current,omitempty"`
	UpdatedAt      *time.Time                   `json:"updatedAt,omitempty"`
	Stream         string                       `json:"stream,omitempty"`
	Durable        string                       `json:"durable,omitempty"`
	SustainedForMs int64                        `json:"sustainedForMs,omitempty"`
	Available      bool                         `json:"available"`
	Anomaly        bool                         `json:"anomaly"`
}

// BehaviorFingerprintKVKey builds the KV key for a consumer fingerprint.
func BehaviorFingerprintKVKey(stream, durable string) string {
	return strings.TrimSpace(stream) + "/" + strings.TrimSpace(durable)
}

// ParseBehaviorFingerprintKV decodes a worker-published KV value.
// Missing/empty/invalid payloads return Available=false without error.
func ParseBehaviorFingerprintKV(raw []byte, fallbackStream, fallbackDurable string) BehaviorFingerprintReport {
	out := BehaviorFingerprintReport{Available: false}
	if len(raw) == 0 {
		return out
	}

	var payload struct {
		Normal         *BehaviorFingerprintSnapshot `json:"normal"`
		Current        *BehaviorFingerprintSnapshot `json:"current"`
		UpdatedAt      *time.Time                   `json:"updatedAt"`
		Stream         string                       `json:"stream"`
		Durable        string                       `json:"durable"`
		SustainedForMs int64                        `json:"sustainedForMs"`
		Anomaly        bool                         `json:"anomaly"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return out
	}
	if payload.Normal == nil || payload.Current == nil {
		return out
	}

	stream := strings.TrimSpace(payload.Stream)
	if commonstrings.IsEmpty(stream) {
		stream = strings.TrimSpace(fallbackStream)
	}
	durable := strings.TrimSpace(payload.Durable)
	if commonstrings.IsEmpty(durable) {
		durable = strings.TrimSpace(fallbackDurable)
	}

	out.Available = true
	out.Stream = stream
	out.Durable = durable
	out.Anomaly = payload.Anomaly
	out.Normal = payload.Normal
	out.Current = payload.Current
	out.SustainedForMs = payload.SustainedForMs
	out.UpdatedAt = payload.UpdatedAt
	return out
}
