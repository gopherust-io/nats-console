package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

type streamPlacementRequest struct {
	Cluster string   `json:"cluster,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

type streamExternalRequest struct {
	APIPrefix     string `json:"api,omitempty"`
	DeliverPrefix string `json:"deliver,omitempty"`
}

type streamSourceRequest struct {
	External      *streamExternalRequest `json:"external,omitempty"`
	Name          string                 `json:"name"`
	FilterSubject string                 `json:"filterSubject,omitempty"`
	OptStartTime  string                 `json:"optStartTime,omitempty"`
	OptStartSeq   uint64                 `json:"optStartSeq,omitempty"`
}

type subjectTransformRequest struct {
	Source      string `json:"src,omitempty"`
	Destination string `json:"dest"`
}

// goalign:ignore
type rePublishRequest struct {
	Source      string `json:"src,omitempty"`
	Destination string `json:"dest"`
	HeadersOnly bool   `json:"headersOnly,omitempty"`
}

type consumerLimitsRequest struct {
	InactiveThreshold int64 `json:"inactiveThreshold,omitempty"` // nanoseconds
	MaxAckPending     int   `json:"maxAckPending,omitempty"`
}

type streamConfigRequest struct {
	AllowMsgTTL            *bool                    `json:"allowMsgTTL,omitempty"`
	AllowRollup            *bool                    `json:"allowRollup,omitempty"`
	Sealed                 *bool                    `json:"sealed,omitempty"`
	NoAck                  *bool                    `json:"noAck,omitempty"`
	DiscardNewPerSubject   *bool                    `json:"discardNewPerSubject,omitempty"`
	DenyPurge              *bool                    `json:"denyPurge,omitempty"`
	DenyDelete             *bool                    `json:"denyDelete,omitempty"`
	AllowDirect            *bool                    `json:"allowDirect,omitempty"`
	SubjectTransform       *subjectTransformRequest `json:"subjectTransform,omitempty"`
	ConsumerLimits         *consumerLimitsRequest   `json:"consumerLimits,omitempty"`
	MirrorDirect           *bool                    `json:"mirrorDirect,omitempty"`
	RePublish              *rePublishRequest        `json:"republish,omitempty"`
	Metadata               map[string]string        `json:"metadata,omitempty"`
	Mirror                 *streamSourceRequest     `json:"mirror,omitempty"`
	Placement              *streamPlacementRequest  `json:"placement,omitempty"`
	Name                   string                   `json:"name"`
	Description            string                   `json:"description,omitempty"`
	Compression            string                   `json:"compression,omitempty"`
	Discard                string                   `json:"discard,omitempty"`
	Storage                string                   `json:"storage,omitempty"`
	Retention              string                   `json:"retention,omitempty"`
	Sources                []streamSourceRequest    `json:"sources,omitempty"`
	Subjects               []string                 `json:"subjects,omitempty"`
	MaxBytes               int64                    `json:"maxBytes,omitempty"`
	SubjectDeleteMarkerTTL int64                    `json:"subjectDeleteMarkerTTL,omitempty"`
	FirstSeq               uint64                   `json:"firstSeq,omitempty"`
	Duplicates             int64                    `json:"duplicates,omitempty"`
	Replicas               int                      `json:"replicas,omitempty"`
	MaxMsgsPerSubject      int64                    `json:"maxMsgsPerSubject,omitempty"`
	MaxMsgSize             int64                    `json:"maxMsgSize,omitempty"`
	MaxConsumers           int                      `json:"maxConsumers,omitempty"`
	MaxAge                 int64                    `json:"maxAge,omitempty"`
	MaxMsgs                int64                    `json:"maxMsgs,omitempty"`
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
		Metadata:               cloneStringMap(r.Metadata),
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
	if r.Mirror != nil && r.Mirror.Name != "" {
		src, err := r.Mirror.toNATS()
		if err != nil {
			return cfg, fmt.Errorf("mirror: %w", err)
		}
		cfg.Mirror = src
		cfg.Subjects = nil
	}
	if len(r.Sources) > 0 {
		cfg.Sources = make([]*nats.StreamSource, 0, len(r.Sources))
		for i, srcReq := range r.Sources {
			if strings.TrimSpace(srcReq.Name) == "" {
				continue
			}
			src, err := srcReq.toNATS()
			if err != nil {
				return cfg, fmt.Errorf("sources[%d]: %w", i, err)
			}
			cfg.Sources = append(cfg.Sources, src)
		}
	}
	if r.Retention != "" {
		if err := unmarshalEnum(r.Retention, &cfg.Retention); err != nil {
			return cfg, fmt.Errorf("retention: %w", err)
		}
	}
	if r.Storage != "" {
		if err := unmarshalEnum(r.Storage, &cfg.Storage); err != nil {
			return cfg, fmt.Errorf("storage: %w", err)
		}
	} else {
		cfg.Storage = nats.FileStorage
	}
	if r.Discard != "" {
		if err := unmarshalEnum(r.Discard, &cfg.Discard); err != nil {
			return cfg, fmt.Errorf("discard: %w", err)
		}
	}
	if r.Compression != "" {
		if err := unmarshalEnum(r.Compression, &cfg.Compression); err != nil {
			return cfg, fmt.Errorf("compression: %w", err)
		}
	}
	if r.Placement != nil && (r.Placement.Cluster != "" || len(r.Placement.Tags) > 0) {
		cfg.Placement = &nats.Placement{
			Cluster: r.Placement.Cluster,
			Tags:    append([]string(nil), r.Placement.Tags...),
		}
	}
	if r.SubjectTransform != nil && r.SubjectTransform.Destination != "" {
		cfg.SubjectTransform = &nats.SubjectTransformConfig{
			Source:      r.SubjectTransform.Source,
			Destination: r.SubjectTransform.Destination,
		}
	}
	if r.RePublish != nil && r.RePublish.Destination != "" {
		cfg.RePublish = &nats.RePublish{
			Source:      r.RePublish.Source,
			Destination: r.RePublish.Destination,
			HeadersOnly: r.RePublish.HeadersOnly,
		}
	}
	if r.ConsumerLimits != nil && (r.ConsumerLimits.InactiveThreshold > 0 || r.ConsumerLimits.MaxAckPending != 0) {
		cfg.ConsumerLimits = nats.StreamConsumerLimits{
			InactiveThreshold: time.Duration(r.ConsumerLimits.InactiveThreshold),
			MaxAckPending:     r.ConsumerLimits.MaxAckPending,
		}
	}
	return cfg, nil
}

func (r streamSourceRequest) toNATS() (*nats.StreamSource, error) {
	src := &nats.StreamSource{
		Name:          r.Name,
		FilterSubject: r.FilterSubject,
		OptStartSeq:   r.OptStartSeq,
	}
	if strings.TrimSpace(r.OptStartTime) != "" {
		t, err := time.Parse(time.RFC3339Nano, r.OptStartTime)
		if err != nil {
			t, err = time.Parse(time.RFC3339, r.OptStartTime)
			if err != nil {
				return nil, fmt.Errorf("optStartTime: %w", err)
			}
		}
		src.OptStartTime = &t
	}
	if r.External != nil && (r.External.APIPrefix != "" || r.External.DeliverPrefix != "") {
		src.External = &nats.ExternalStream{
			APIPrefix:     r.External.APIPrefix,
			DeliverPrefix: r.External.DeliverPrefix,
		}
	}
	return src, nil
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	maps.Copy(out, in)
	return out
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
		Metadata:           cloneStringMap(r.Metadata),
	}
	if r.DeliverPolicy != "" {
		if err := unmarshalEnum(r.DeliverPolicy, &cfg.DeliverPolicy); err != nil {
			return cfg, fmt.Errorf("deliverPolicy: %w", err)
		}
	}
	if r.AckPolicy != "" {
		if err := unmarshalEnum(r.AckPolicy, &cfg.AckPolicy); err != nil {
			return cfg, fmt.Errorf("ackPolicy: %w", err)
		}
	}
	if r.ReplayPolicy != "" {
		if err := unmarshalEnum(r.ReplayPolicy, &cfg.ReplayPolicy); err != nil {
			return cfg, fmt.Errorf("replayPolicy: %w", err)
		}
	}
	if strings.TrimSpace(r.OptStartTime) != "" {
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

// goalign:ignore
type kvBucketConfigRequest struct {
	Placement        *streamPlacementRequest `json:"placement,omitempty"`
	Metadata         map[string]string       `json:"metadata,omitempty"`
	Mirror           *streamSourceRequest    `json:"mirror,omitempty"`
	RePublish        *rePublishRequest       `json:"republish,omitempty"`
	Storage          string                  `json:"storage,omitempty"`
	Description      string                  `json:"description,omitempty"`
	Bucket           string                  `json:"bucket"`
	Sources          []streamSourceRequest   `json:"sources,omitempty"`
	TTLNs            int64                   `json:"ttlNs,omitempty"`
	Replicas         int                     `json:"replicas,omitempty"`
	MaxBytes         int64                   `json:"maxBytes,omitempty"`
	LimitMarkerTTLNs int64                   `json:"limitMarkerTTLNs,omitempty"`
	MaxValueSize     int32                   `json:"maxValueSize,omitempty"`
	History          uint8                   `json:"history,omitempty"`
	Compression      bool                    `json:"compression,omitempty"`
}

func (r kvBucketConfigRequest) toKVConfig() (nats.KeyValueConfig, error) {
	replicas := r.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	if replicas != 1 && replicas != 3 && replicas != 5 {
		return nats.KeyValueConfig{}, errors.New("replicas must be 1, 3, or 5")
	}
	history := r.History
	if history == 0 {
		history = 1
	}
	if history > 64 {
		return nats.KeyValueConfig{}, errors.New("history must be between 1 and 64")
	}
	cfg := nats.KeyValueConfig{
		Bucket:       r.Bucket,
		Description:  r.Description,
		History:      history,
		TTL:          time.Duration(r.TTLNs),
		MaxValueSize: r.MaxValueSize,
		MaxBytes:     r.MaxBytes,
		Replicas:     replicas,
		Compression:  r.Compression,
	}
	if r.Storage != "" {
		if err := unmarshalEnum(r.Storage, &cfg.Storage); err != nil {
			return cfg, fmt.Errorf("storage: %w", err)
		}
	} else {
		cfg.Storage = nats.FileStorage
	}
	if r.Placement != nil && (r.Placement.Cluster != "" || len(r.Placement.Tags) > 0) {
		cfg.Placement = &nats.Placement{
			Cluster: r.Placement.Cluster,
			Tags:    append([]string(nil), r.Placement.Tags...),
		}
	}
	if r.RePublish != nil && r.RePublish.Destination != "" {
		cfg.RePublish = &nats.RePublish{
			Source:      r.RePublish.Source,
			Destination: r.RePublish.Destination,
			HeadersOnly: r.RePublish.HeadersOnly,
		}
	}
	if r.Mirror != nil && strings.TrimSpace(r.Mirror.Name) != "" {
		src, err := r.Mirror.toNATS()
		if err != nil {
			return cfg, fmt.Errorf("mirror: %w", err)
		}
		cfg.Mirror = src
	}
	if len(r.Sources) > 0 {
		cfg.Sources = make([]*nats.StreamSource, 0, len(r.Sources))
		for i, srcReq := range r.Sources {
			if strings.TrimSpace(srcReq.Name) == "" {
				continue
			}
			src, err := srcReq.toNATS()
			if err != nil {
				return cfg, fmt.Errorf("sources[%d]: %w", i, err)
			}
			cfg.Sources = append(cfg.Sources, src)
		}
	}
	return cfg, nil
}

// goalign:ignore
type objectBucketConfigRequest struct {
	Placement   *streamPlacementRequest `json:"placement,omitempty"`
	Metadata    map[string]string       `json:"metadata,omitempty"`
	Bucket      string                  `json:"bucket"`
	Description string                  `json:"description,omitempty"`
	Storage     string                  `json:"storage,omitempty"`
	TTLNs       int64                   `json:"ttlNs,omitempty"`
	MaxBytes    int64                   `json:"maxBytes,omitempty"`
	Replicas    int                     `json:"replicas,omitempty"`
	Compression bool                    `json:"compression,omitempty"`
}

func (r objectBucketConfigRequest) toObjectConfig() (nats.ObjectStoreConfig, error) {
	replicas := r.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	if replicas != 1 && replicas != 3 && replicas != 5 {
		return nats.ObjectStoreConfig{}, errors.New("replicas must be 1, 3, or 5")
	}
	cfg := nats.ObjectStoreConfig{
		Bucket:      r.Bucket,
		Description: r.Description,
		TTL:         time.Duration(r.TTLNs),
		MaxBytes:    r.MaxBytes,
		Replicas:    replicas,
		Compression: r.Compression,
		Metadata:    r.Metadata,
	}
	if r.Storage != "" {
		if err := unmarshalEnum(r.Storage, &cfg.Storage); err != nil {
			return cfg, fmt.Errorf("storage: %w", err)
		}
	} else {
		cfg.Storage = nats.FileStorage
	}
	if r.Placement != nil && (r.Placement.Cluster != "" || len(r.Placement.Tags) > 0) {
		cfg.Placement = &nats.Placement{
			Cluster: r.Placement.Cluster,
			Tags:    append([]string(nil), r.Placement.Tags...),
		}
	}
	return cfg, nil
}

func unmarshalEnum[T any](value string, target *T) error {
	return json.Unmarshal([]byte(`"`+value+`"`), target)
}
