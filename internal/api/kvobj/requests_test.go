package kvobj

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
)

func TestKVBucketConfigRequestToKVConfig(t *testing.T) {
	t.Parallel()

	cfg, err := (kvBucketConfigRequest{
		Bucket:       "CONFIG",
		Description:  "feature flags",
		Storage:      "memory",
		TTLNs:        int64(time.Hour),
		MaxBytes:     1 << 20,
		MaxValueSize: 4096,
		History:      10,
		Replicas:     3,
		Compression:  true,
		Placement: &apikit.StreamPlacementRequest{
			Cluster: "east",
			Tags:    []string{"ssd"},
		},
		RePublish: &apikit.RePublishRequest{
			Source:      "kv.CONFIG.>",
			Destination: "mirror.>",
			HeadersOnly: true,
		},
	}).toKVConfig()
	require.NoError(t, err)
	assert.Equal(t, "CONFIG", cfg.Bucket)
	assert.Equal(t, "feature flags", cfg.Description)
	assert.Equal(t, nats.MemoryStorage, cfg.Storage)
	assert.Equal(t, time.Hour, cfg.TTL)
	assert.Equal(t, int64(1<<20), cfg.MaxBytes)
	assert.Equal(t, int32(4096), cfg.MaxValueSize)
	assert.Equal(t, uint8(10), cfg.History)
	assert.Equal(t, 3, cfg.Replicas)
	assert.True(t, cfg.Compression)
	require.NotNil(t, cfg.Placement)
	assert.Equal(t, "east", cfg.Placement.Cluster)
	require.NotNil(t, cfg.RePublish)
	assert.Equal(t, "mirror.>", cfg.RePublish.Destination)
	assert.True(t, cfg.RePublish.HeadersOnly)
}

func TestKVBucketConfigRequestDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := (kvBucketConfigRequest{Bucket: "B"}).toKVConfig()
	require.NoError(t, err)
	assert.Equal(t, nats.FileStorage, cfg.Storage)
	assert.Equal(t, 1, cfg.Replicas)
	assert.Equal(t, uint8(1), cfg.History)
	assert.Nil(t, cfg.Placement)
	assert.False(t, cfg.Compression)
}

func TestKVBucketConfigRequestInvalidReplicas(t *testing.T) {
	t.Parallel()

	_, err := (kvBucketConfigRequest{Bucket: "B", Replicas: 2}).toKVConfig()
	require.Error(t, err)
}

func TestKVBucketConfigRequestInvalidHistory(t *testing.T) {
	t.Parallel()

	_, err := (kvBucketConfigRequest{Bucket: "B", History: 65}).toKVConfig()
	require.Error(t, err)
}

func TestObjectBucketConfigRequestToObjectConfig(t *testing.T) {
	t.Parallel()

	cfg, err := (objectBucketConfigRequest{
		Bucket:      "ARTIFACTS",
		Description: "build artifacts",
		Storage:     "memory",
		TTLNs:       int64(time.Hour),
		MaxBytes:    1 << 30,
		Replicas:    3,
		Compression: true,
		Placement: &apikit.StreamPlacementRequest{
			Cluster: "east",
			Tags:    []string{"ssd"},
		},
		Metadata: map[string]string{"owner": "ops"},
	}).toObjectConfig()
	require.NoError(t, err)
	assert.Equal(t, "ARTIFACTS", cfg.Bucket)
	assert.Equal(t, "build artifacts", cfg.Description)
	assert.Equal(t, nats.MemoryStorage, cfg.Storage)
	assert.Equal(t, time.Hour, cfg.TTL)
	assert.Equal(t, int64(1<<30), cfg.MaxBytes)
	assert.Equal(t, 3, cfg.Replicas)
	assert.True(t, cfg.Compression)
	require.NotNil(t, cfg.Placement)
	assert.Equal(t, "east", cfg.Placement.Cluster)
	assert.Equal(t, []string{"ssd"}, cfg.Placement.Tags)
	assert.Equal(t, "ops", cfg.Metadata["owner"])
}

func TestObjectBucketConfigRequestDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := (objectBucketConfigRequest{Bucket: "B"}).toObjectConfig()
	require.NoError(t, err)
	assert.Equal(t, nats.FileStorage, cfg.Storage)
	assert.Equal(t, 1, cfg.Replicas)
	assert.Nil(t, cfg.Placement)
	assert.False(t, cfg.Compression)
}

func TestObjectBucketConfigRequestInvalidReplicas(t *testing.T) {
	t.Parallel()

	_, err := (objectBucketConfigRequest{Bucket: "B", Replicas: 2}).toObjectConfig()
	require.Error(t, err)
}
