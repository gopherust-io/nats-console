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

type StreamConfigDTO struct {
	Name      string   `json:"name"`
	Retention string   `json:"retention"`
	Storage   string   `json:"storage"`
	Subjects  []string `json:"subjects,omitempty"`
	MaxMsgs   int64    `json:"maxMsgs,omitempty"`
	MaxBytes  int64    `json:"maxBytes,omitempty"`
	MaxAge    int64    `json:"maxAge,omitempty"`
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

type ConsumerConfigDTO struct {
	DurableName    string   `json:"durableName,omitempty"`
	Name           string   `json:"name,omitempty"`
	DeliverPolicy  string   `json:"deliverPolicy"`
	AckPolicy      string   `json:"ackPolicy"`
	FilterSubject  string   `json:"filterSubject,omitempty"`
	FilterSubjects []string `json:"filterSubjects,omitempty"`
}

type SequenceInfoDTO struct {
	ConsumerSeq uint64 `json:"consumerSeq"`
	StreamSeq   uint64 `json:"streamSeq"`
}
