package domain

import "time"

type KVPlacement struct {
	Cluster string   `json:"cluster,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

// goalign:ignore
type KVRePublish struct {
	Source      string `json:"src,omitempty"`
	Destination string `json:"dest"`
	HeadersOnly bool   `json:"headersOnly,omitempty"`
}

type KVStreamSource struct {
	Name          string `json:"name"`
	FilterSubject string `json:"filterSubject,omitempty"`
}

// goalign:ignore
type KVBucketInfo struct {
	Placement      *KVPlacement      `json:"placement,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Mirror         *KVStreamSource   `json:"mirror,omitempty"`
	RePublish      *KVRePublish      `json:"republish,omitempty"`
	Description    string            `json:"description,omitempty"`
	Bucket         string            `json:"bucket"`
	Storage        string            `json:"storage,omitempty"`
	Sources        []KVStreamSource  `json:"sources,omitempty"`
	MaxBytes       int64             `json:"maxBytes,omitempty"`
	Values         uint64            `json:"values"`
	LimitMarkerTTL int64             `json:"limitMarkerTTLNs,omitempty"`
	TTLNs          int64             `json:"ttlNs,omitempty"`
	History        int64             `json:"history"`
	Bytes          uint64            `json:"bytes,omitempty"`
	Replicas       int               `json:"replicas,omitempty"`
	MaxValueSize   int32             `json:"maxValueSize,omitempty"`
	Compressed     bool              `json:"compressed,omitempty"`
}

// KVBucketWriteOpts carries KV options that live on the backing stream
// (LimitMarkerTTL / Metadata) rather than classic KeyValueConfig.
type KVBucketWriteOpts struct {
	Metadata         map[string]string
	LimitMarkerTTLNs int64
}

type KVEntry struct {
	Created  time.Time `json:"created"`
	Bucket   string    `json:"bucket"`
	Key      string    `json:"key"`
	Value    string    `json:"value"`
	Revision uint64    `json:"revision"`
}
