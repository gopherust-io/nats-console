package jetstream

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type streamConfigRequest struct {
	AllowMsgTTL            *bool                           `json:"allowMsgTTL,omitempty"`
	AllowRollup            *bool                           `json:"allowRollup,omitempty"`
	Sealed                 *bool                           `json:"sealed,omitempty"`
	NoAck                  *bool                           `json:"noAck,omitempty"`
	DiscardNewPerSubject   *bool                           `json:"discardNewPerSubject,omitempty"`
	DenyPurge              *bool                           `json:"denyPurge,omitempty"`
	DenyDelete             *bool                           `json:"denyDelete,omitempty"`
	AllowDirect            *bool                           `json:"allowDirect,omitempty"`
	SubjectTransform       *apikit.SubjectTransformRequest `json:"subjectTransform,omitempty"`
	ConsumerLimits         *apikit.ConsumerLimitsRequest   `json:"consumerLimits,omitempty"`
	MirrorDirect           *bool                           `json:"mirrorDirect,omitempty"`
	RePublish              *apikit.RePublishRequest        `json:"republish,omitempty"`
	Metadata               map[string]string               `json:"metadata,omitempty"`
	Mirror                 *apikit.StreamSourceRequest     `json:"mirror,omitempty"`
	Placement              *apikit.StreamPlacementRequest  `json:"placement,omitempty"`
	Name                   string                          `json:"name"`
	Description            string                          `json:"description,omitempty"`
	Compression            string                          `json:"compression,omitempty"`
	Discard                string                          `json:"discard,omitempty"`
	Storage                string                          `json:"storage,omitempty"`
	Retention              string                          `json:"retention,omitempty"`
	Sources                []apikit.StreamSourceRequest    `json:"sources,omitempty"`
	Subjects               []string                        `json:"subjects,omitempty"`
	MaxBytes               int64                           `json:"maxBytes,omitempty"`
	SubjectDeleteMarkerTTL int64                           `json:"subjectDeleteMarkerTTL,omitempty"`
	FirstSeq               uint64                          `json:"firstSeq,omitempty"`
	Duplicates             int64                           `json:"duplicates,omitempty"`
	Replicas               int                             `json:"replicas,omitempty"`
	MaxMsgsPerSubject      int64                           `json:"maxMsgsPerSubject,omitempty"`
	MaxMsgSize             int64                           `json:"maxMsgSize,omitempty"`
	MaxConsumers           int                             `json:"maxConsumers,omitempty"`
	MaxAge                 int64                           `json:"maxAge,omitempty"`
	MaxMsgs                int64                           `json:"maxMsgs,omitempty"`
}

func (r streamConfigRequest) toNATS() (nats.StreamConfig, error) {
	replicas := r.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	cfg := nats.StreamConfig{
		Name:                   r.Name,
		Description:            r.Description,
		Subjects:               r.Subjects,
		MaxMsgs:                r.MaxMsgs,
		MaxBytes:               r.MaxBytes,
		MaxAge:                 time.Duration(r.MaxAge),
		MaxConsumers:           r.MaxConsumers,
		MaxMsgsPerSubject:      r.MaxMsgsPerSubject,
		Replicas:               replicas,
		Duplicates:             time.Duration(r.Duplicates),
		FirstSeq:               r.FirstSeq,
		SubjectDeleteMarkerTTL: time.Duration(r.SubjectDeleteMarkerTTL),
		AllowRollup:            true,
		Metadata:               apikit.CloneStringMap(r.Metadata),
	}
	if r.MaxMsgSize != 0 {
		if r.MaxMsgSize < math.MinInt32 || r.MaxMsgSize > math.MaxInt32 {
			return cfg, errors.New("maxMsgSize out of range")
		}
		cfg.MaxMsgSize = int32(r.MaxMsgSize)
	}
	if r.AllowRollup != nil {
		cfg.AllowRollup = *r.AllowRollup
	}
	if r.DenyDelete != nil {
		cfg.DenyDelete = *r.DenyDelete
	}
	if r.DenyPurge != nil {
		cfg.DenyPurge = *r.DenyPurge
	}
	if r.DiscardNewPerSubject != nil {
		cfg.DiscardNewPerSubject = *r.DiscardNewPerSubject
	}
	if r.NoAck != nil {
		cfg.NoAck = *r.NoAck
	}
	if r.Sealed != nil {
		cfg.Sealed = *r.Sealed
	}
	if r.AllowDirect != nil {
		cfg.AllowDirect = *r.AllowDirect
	}
	if r.MirrorDirect != nil {
		cfg.MirrorDirect = *r.MirrorDirect
	}
	if r.AllowMsgTTL != nil {
		cfg.AllowMsgTTL = *r.AllowMsgTTL
	}
	if r.Mirror != nil && !commonstrings.IsEmpty(r.Mirror.Name) {
		src, err := r.Mirror.ToNATS()
		if err != nil {
			return cfg, fmt.Errorf("mirror: %w", err)
		}
		cfg.Mirror = src
		cfg.Subjects = nil
	}
	sources, err := apikit.SourcesToNATS(r.Sources)
	if err != nil {
		return cfg, err
	}
	cfg.Sources = sources
	if !commonstrings.IsEmpty(r.Retention) {
		if err := apikit.UnmarshalEnum(r.Retention, &cfg.Retention); err != nil {
			return cfg, fmt.Errorf("retention: %w", err)
		}
	}
	if !commonstrings.IsEmpty(r.Storage) {
		if err := apikit.UnmarshalEnum(r.Storage, &cfg.Storage); err != nil {
			return cfg, fmt.Errorf("storage: %w", err)
		}
	} else {
		cfg.Storage = nats.FileStorage
	}
	if !commonstrings.IsEmpty(r.Discard) {
		if err := apikit.UnmarshalEnum(r.Discard, &cfg.Discard); err != nil {
			return cfg, fmt.Errorf("discard: %w", err)
		}
	}
	if !commonstrings.IsEmpty(r.Compression) {
		if err := apikit.UnmarshalEnum(r.Compression, &cfg.Compression); err != nil {
			return cfg, fmt.Errorf("compression: %w", err)
		}
	}
	cfg.Placement = r.Placement.ToNATSPlacement()
	if r.SubjectTransform != nil && !commonstrings.IsEmpty(r.SubjectTransform.Destination) {
		cfg.SubjectTransform = &nats.SubjectTransformConfig{
			Source:      r.SubjectTransform.Source,
			Destination: r.SubjectTransform.Destination,
		}
	}
	cfg.RePublish = r.RePublish.ToNATSRePublish()
	if r.ConsumerLimits != nil && (r.ConsumerLimits.InactiveThreshold > 0 || r.ConsumerLimits.MaxAckPending != 0) {
		cfg.ConsumerLimits = nats.StreamConsumerLimits{
			InactiveThreshold: time.Duration(r.ConsumerLimits.InactiveThreshold),
			MaxAckPending:     r.ConsumerLimits.MaxAckPending,
		}
	}
	return cfg, nil
}

// goalign:ignore
type consumerConfigRequest struct {
	DurableName         string            `json:"durableName,omitempty"`
	Name                string            `json:"name,omitempty"`
	Description         string            `json:"description,omitempty"`
	DeliverPolicy       string            `json:"deliverPolicy,omitempty"`
	AckPolicy           string            `json:"ackPolicy,omitempty"`
	FilterSubject       string            `json:"filterSubject,omitempty"`
	OptStartTime        string            `json:"optStartTime,omitempty"` // RFC3339
	ReplayPolicy        string            `json:"replayPolicy,omitempty"`
	FilterSubjects      []string          `json:"filterSubjects,omitempty"`
	SampleFreq          string            `json:"sampleFreq,omitempty"`
	DeliverSubject      string            `json:"deliverSubject,omitempty"`
	DeliverGroup        string            `json:"deliverGroup,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	BackoffNs           []int64           `json:"backoffNs,omitempty"`
	OptStartSeq         uint64            `json:"optStartSeq,omitempty"`
	AckWaitNs           int64             `json:"ackWaitNs,omitempty"`
	InactiveThresholdNs int64             `json:"inactiveThresholdNs,omitempty"`
	MaxRequestExpiresNs int64             `json:"maxRequestExpiresNs,omitempty"`
	HeartbeatNs         int64             `json:"heartbeatNs,omitempty"`
	RateLimitBps        uint64            `json:"rateLimitBps,omitempty"`
	MaxDeliver          int               `json:"maxDeliver,omitempty"`
	MaxAckPending       int               `json:"maxAckPending,omitempty"`
	MaxWaiting          int               `json:"maxWaiting,omitempty"`
	MaxRequestBatch     int               `json:"maxRequestBatch,omitempty"`
	MaxRequestMaxBytes  int               `json:"maxRequestMaxBytes,omitempty"`
	Replicas            int               `json:"replicas,omitempty"`
	FlowControl         bool              `json:"flowControl,omitempty"`
	HeadersOnly         bool              `json:"headersOnly,omitempty"`
	MemoryStorage       bool              `json:"memoryStorage,omitempty"`
}

func (r consumerConfigRequest) toNATS() (nats.ConsumerConfig, error) {
	cfg := nats.ConsumerConfig{
		Durable:            r.DurableName,
		Name:               r.Name,
		Description:        r.Description,
		FilterSubject:      r.FilterSubject,
		FilterSubjects:     r.FilterSubjects,
		OptStartSeq:        r.OptStartSeq,
		AckWait:            time.Duration(r.AckWaitNs),
		MaxDeliver:         r.MaxDeliver,
		RateLimit:          r.RateLimitBps,
		SampleFrequency:    r.SampleFreq,
		MaxWaiting:         r.MaxWaiting,
		MaxAckPending:      r.MaxAckPending,
		FlowControl:        r.FlowControl,
		Heartbeat:          time.Duration(r.HeartbeatNs),
		HeadersOnly:        r.HeadersOnly,
		MaxRequestBatch:    r.MaxRequestBatch,
		MaxRequestExpires:  time.Duration(r.MaxRequestExpiresNs),
		MaxRequestMaxBytes: r.MaxRequestMaxBytes,
		DeliverSubject:     r.DeliverSubject,
		DeliverGroup:       r.DeliverGroup,
		InactiveThreshold:  time.Duration(r.InactiveThresholdNs),
		Replicas:           r.Replicas,
		MemoryStorage:      r.MemoryStorage,
		Metadata:           apikit.CloneStringMap(r.Metadata),
	}
	if !commonstrings.IsEmpty(r.DeliverPolicy) {
		if err := apikit.UnmarshalEnum(r.DeliverPolicy, &cfg.DeliverPolicy); err != nil {
			return cfg, fmt.Errorf("deliverPolicy: %w", err)
		}
	}
	if !commonstrings.IsEmpty(r.AckPolicy) {
		if err := apikit.UnmarshalEnum(r.AckPolicy, &cfg.AckPolicy); err != nil {
			return cfg, fmt.Errorf("ackPolicy: %w", err)
		}
	}
	if !commonstrings.IsEmpty(r.ReplayPolicy) {
		if err := apikit.UnmarshalEnum(r.ReplayPolicy, &cfg.ReplayPolicy); err != nil {
			return cfg, fmt.Errorf("replayPolicy: %w", err)
		}
	}
	if !commonstrings.IsEmpty(strings.TrimSpace(r.OptStartTime)) {
		t, err := time.Parse(time.RFC3339Nano, r.OptStartTime)
		if err != nil {
			t, err = time.Parse(time.RFC3339, r.OptStartTime)
			if err != nil {
				return cfg, fmt.Errorf("optStartTime: %w", err)
			}
		}
		cfg.OptStartTime = &t
	}
	if len(r.BackoffNs) > 0 {
		cfg.BackOff = make([]time.Duration, len(r.BackoffNs))
		for i, ns := range r.BackoffNs {
			if ns <= 0 {
				return cfg, fmt.Errorf("backoffNs[%d] must be positive", i)
			}
			cfg.BackOff[i] = time.Duration(ns)
		}
	}
	switch cfg.DeliverPolicy {
	case nats.DeliverByStartSequencePolicy:
		if cfg.OptStartSeq == 0 {
			return cfg, errors.New("optStartSeq is required when deliverPolicy is by_start_sequence")
		}
	case nats.DeliverByStartTimePolicy:
		if cfg.OptStartTime == nil {
			return cfg, errors.New("optStartTime is required when deliverPolicy is by_start_time")
		}
	}
	return cfg, nil
}
