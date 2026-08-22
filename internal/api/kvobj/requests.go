package kvobj

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// goalign:ignore
type kvBucketConfigRequest struct {
	Placement        *apikit.StreamPlacementRequest `json:"placement,omitempty"`
	Metadata         map[string]string              `json:"metadata,omitempty"`
	Mirror           *apikit.StreamSourceRequest    `json:"mirror,omitempty"`
	RePublish        *apikit.RePublishRequest       `json:"republish,omitempty"`
	Storage          string                         `json:"storage,omitempty"`
	Description      string                         `json:"description,omitempty"`
	Bucket           string                         `json:"bucket"`
	Sources          []apikit.StreamSourceRequest   `json:"sources,omitempty"`
	TTLNs            int64                          `json:"ttlNs,omitempty"`
	Replicas         int                            `json:"replicas,omitempty"`
	MaxBytes         int64                          `json:"maxBytes,omitempty"`
	LimitMarkerTTLNs int64                          `json:"limitMarkerTTLNs,omitempty"`
	MaxValueSize     int32                          `json:"maxValueSize,omitempty"`
	History          uint8                          `json:"history,omitempty"`
	Compression      bool                           `json:"compression,omitempty"`
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
	if !commonstrings.IsEmpty(r.Storage) {
		if err := apikit.UnmarshalEnum(r.Storage, &cfg.Storage); err != nil {
			return cfg, fmt.Errorf("storage: %w", err)
		}
	} else {
		cfg.Storage = nats.FileStorage
	}
	cfg.Placement = r.Placement.ToNATSPlacement()
	cfg.RePublish = r.RePublish.ToNATSRePublish()
	if r.Mirror != nil && !commonstrings.IsEmpty(strings.TrimSpace(r.Mirror.Name)) {
		src, err := r.Mirror.ToNATS()
		if err != nil {
			return cfg, fmt.Errorf("mirror: %w", err)
		}
		cfg.Mirror = src
	}
	sources, err := apikit.SourcesToNATS(r.Sources)
	if err != nil {
		return cfg, err
	}
	cfg.Sources = sources
	return cfg, nil
}

// goalign:ignore
type objectBucketConfigRequest struct {
	Placement   *apikit.StreamPlacementRequest `json:"placement,omitempty"`
	Metadata    map[string]string              `json:"metadata,omitempty"`
	Bucket      string                         `json:"bucket"`
	Description string                         `json:"description,omitempty"`
	Storage     string                         `json:"storage,omitempty"`
	TTLNs       int64                          `json:"ttlNs,omitempty"`
	MaxBytes    int64                          `json:"maxBytes,omitempty"`
	Replicas    int                            `json:"replicas,omitempty"`
	Compression bool                           `json:"compression,omitempty"`
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
	if !commonstrings.IsEmpty(r.Storage) {
		if err := apikit.UnmarshalEnum(r.Storage, &cfg.Storage); err != nil {
			return cfg, fmt.Errorf("storage: %w", err)
		}
	} else {
		cfg.Storage = nats.FileStorage
	}
	cfg.Placement = r.Placement.ToNATSPlacement()
	return cfg, nil
}
