package domain

import "time"

type ObjectPlacement struct {
	Cluster string   `json:"cluster,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

// goalign:ignore
type ObjectBucketInfo struct {
	Placement   *ObjectPlacement  `json:"placement,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Bucket      string            `json:"bucket"`
	Description string            `json:"description,omitempty"`
	Storage     string            `json:"storage,omitempty"`
	Size        uint64            `json:"size"`
	TTLNs       int64             `json:"ttlNs,omitempty"`
	MaxBytes    int64             `json:"maxBytes,omitempty"`
	Replicas    int               `json:"replicas,omitempty"`
	Compressed  bool              `json:"compressed,omitempty"`
	Sealed      bool              `json:"sealed,omitempty"`
}

type ObjectInfo struct {
	Modified time.Time `json:"modified"`
	Bucket   string    `json:"bucket"`
	Name     string    `json:"name"`
	Data     string    `json:"data"`
	Size     uint64    `json:"size"`
}
