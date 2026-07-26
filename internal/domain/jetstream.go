package domain

import (
	"time"
)

type AccountInfo struct {
	Memory    uint64        `json:"memory"`
	Storage   uint64        `json:"storage"`
	Streams   int           `json:"streams"`
	Consumers int           `json:"consumers"`
	Limits    AccountLimits `json:"limits"`
}

type AccountLimits struct {
	MaxMemory    int64 `json:"maxMemory"`
	MaxStorage   int64 `json:"maxStorage"`
	MaxStreams   int   `json:"maxStreams"`
	MaxConsumers int   `json:"maxConsumers"`
}

type StreamInfo struct {
	Created time.Time       `json:"created"`
	Config  StreamConfigDTO `json:"config"`
	State   StreamStateDTO  `json:"state"`
}

type StreamPlacementDTO struct {
	Cluster string   `json:"cluster,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

type StreamExternalDTO struct {
	APIPrefix     string `json:"api,omitempty"`
	DeliverPrefix string `json:"deliver,omitempty"`
}

type StreamSourceDTO struct {
	External      *StreamExternalDTO `json:"external,omitempty"`
	Name          string             `json:"name"`
	FilterSubject string             `json:"filterSubject,omitempty"`
	OptStartTime  string             `json:"optStartTime,omitempty"`
	OptStartSeq   uint64             `json:"optStartSeq,omitempty"`
}

type SubjectTransformDTO struct {
	Source      string `json:"src,omitempty"`
	Destination string `json:"dest"`
}

// goalign:ignore
type RePublishDTO struct {
	Source      string `json:"src,omitempty"`
	Destination string `json:"dest"`
	HeadersOnly bool   `json:"headersOnly,omitempty"`
}

type StreamConsumerLimitsDTO struct {
	InactiveThreshold int64 `json:"inactiveThreshold,omitempty"`
	MaxAckPending     int   `json:"maxAckPending,omitempty"`
}

// goalign:ignore
type StreamConfigDTO struct {
	Placement              *StreamPlacementDTO      `json:"placement,omitempty"`
	Metadata               map[string]string        `json:"metadata,omitempty"`
	ConsumerLimits         *StreamConsumerLimitsDTO `json:"consumerLimits,omitempty"`
	RePublish              *RePublishDTO            `json:"republish,omitempty"`
	SubjectTransform       *SubjectTransformDTO     `json:"subjectTransform,omitempty"`
	Mirror                 *StreamSourceDTO         `json:"mirror,omitempty"`
	Storage                string                   `json:"storage"`
	Compression            string                   `json:"compression,omitempty"`
	Name                   string                   `json:"name"`
	Description            string                   `json:"description,omitempty"`
	Retention              string                   `json:"retention"`
	Discard                string                   `json:"discard,omitempty"`
	Sources                []StreamSourceDTO        `json:"sources,omitempty"`
	Subjects               []string                 `json:"subjects,omitempty"`
	Replicas               int                      `json:"replicas,omitempty"`
	MaxConsumers           int                      `json:"maxConsumers,omitempty"`
	MaxMsgs                int64                    `json:"maxMsgs,omitempty"`
	SubjectDeleteMarkerTTL int64                    `json:"subjectDeleteMarkerTTL,omitempty"`
	FirstSeq               uint64                   `json:"firstSeq,omitempty"`
	Duplicates             int64                    `json:"duplicates,omitempty"`
	MaxMsgsPerSubject      int64                    `json:"maxMsgsPerSubject,omitempty"`
	MaxAge                 int64                    `json:"maxAge,omitempty"`
	MaxBytes               int64                    `json:"maxBytes,omitempty"`
	MaxMsgSize             int32                    `json:"maxMsgSize,omitempty"`
	AllowRollup            bool                     `json:"allowRollup"`
	DenyDelete             bool                     `json:"denyDelete"`
	DenyPurge              bool                     `json:"denyPurge"`
	DiscardNewPerSubject   bool                     `json:"discardNewPerSubject"`
	NoAck                  bool                     `json:"noAck"`
	Sealed                 bool                     `json:"sealed"`
	AllowDirect            bool                     `json:"allowDirect"`
	MirrorDirect           bool                     `json:"mirrorDirect"`
	AllowMsgTTL            bool                     `json:"allowMsgTTL"`
}

type StreamStateDTO struct {
	Messages      uint64 `json:"messages"`
	Bytes         uint64 `json:"bytes"`
	FirstSeq      uint64 `json:"firstSeq"`
	LastSeq       uint64 `json:"lastSeq"`
	ConsumerCount int    `json:"consumerCount"`
}

type ConsumerInfo struct {
	Created        time.Time         `json:"created"`
	Delivered      *SequenceInfoDTO  `json:"delivered,omitempty"`
	AckFloor       *SequenceInfoDTO  `json:"ackFloor,omitempty"`
	Name           string            `json:"name"`
	StreamName     string            `json:"streamName"`
	Config         ConsumerConfigDTO `json:"config"`
	NumPending     uint64            `json:"numPending"`
	NumAckPending  int               `json:"numAckPending"`
	NumRedelivered int               `json:"numRedelivered,omitempty"`
	NumWaiting     int               `json:"numWaiting,omitempty"`
}

// goalign:ignore
type ConsumerConfigDTO struct {
	DurableName         string            `json:"durableName,omitempty"`
	Name                string            `json:"name,omitempty"`
	Description         string            `json:"description,omitempty"`
	DeliverPolicy       string            `json:"deliverPolicy"`
	AckPolicy           string            `json:"ackPolicy"`
	FilterSubject       string            `json:"filterSubject,omitempty"`
	FilterSubjects      []string          `json:"filterSubjects,omitempty"`
	OptStartTime        string            `json:"optStartTime,omitempty"`
	ReplayPolicy        string            `json:"replayPolicy,omitempty"`
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

type SequenceInfoDTO struct {
	ConsumerSeq uint64 `json:"consumerSeq"`
	StreamSeq   uint64 `json:"streamSeq"`
}
