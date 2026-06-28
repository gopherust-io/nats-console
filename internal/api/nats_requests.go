package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

type streamConfigRequest struct {
	Name      string   `json:"name"`
	Retention string   `json:"retention,omitempty"`
	Storage   string   `json:"storage,omitempty"`
	Subjects  []string `json:"subjects,omitempty"`
	MaxMsgs   int64    `json:"maxMsgs,omitempty"`
	MaxBytes  int64    `json:"maxBytes,omitempty"`
	MaxAge    int64    `json:"maxAge,omitempty"`
}

func (r streamConfigRequest) toNATS() (nats.StreamConfig, error) {
	cfg := nats.StreamConfig{
		Name:     r.Name,
		Subjects: r.Subjects,
		MaxMsgs:  r.MaxMsgs,
		MaxBytes: r.MaxBytes,
		MaxAge:   time.Duration(r.MaxAge),
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
	}
	return cfg, nil
}

type consumerConfigRequest struct {
	DurableName    string   `json:"durableName,omitempty"`
	Name           string   `json:"name,omitempty"`
	DeliverPolicy  string   `json:"deliverPolicy,omitempty"`
	AckPolicy      string   `json:"ackPolicy,omitempty"`
	FilterSubject  string   `json:"filterSubject,omitempty"`
	FilterSubjects []string `json:"filterSubjects,omitempty"`
}

func (r consumerConfigRequest) toNATS() (nats.ConsumerConfig, error) {
	cfg := nats.ConsumerConfig{
		Durable:        r.DurableName,
		Name:           r.Name,
		FilterSubject:  r.FilterSubject,
		FilterSubjects: r.FilterSubjects,
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
	return cfg, nil
}

type bucketCreateRequest struct {
	Bucket string `json:"bucket"`
}

func (r bucketCreateRequest) toKVConfig() nats.KeyValueConfig {
	return nats.KeyValueConfig{Bucket: r.Bucket}
}

func (r bucketCreateRequest) toObjectConfig() nats.ObjectStoreConfig {
	return nats.ObjectStoreConfig{Bucket: r.Bucket}
}

func unmarshalEnum[T any](value string, target *T) error {
	return json.Unmarshal([]byte(`"`+value+`"`), target)
}
