package natsclient

import (
	"context"
	"errors"
	"fmt"

	libnats "github.com/gopherust-io/nats"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/b64util"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/nats-io/nats.go"
)

func (c *Client) ListObjectBuckets(ctx context.Context) ([]domain.ObjectBucketInfo, error) {
	buckets, err := c.inner.Objects().ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ObjectBucketInfo, 0, len(buckets))
	for _, b := range buckets {
		// List metadata only — avoid N+1 GetObjectBucket/StreamInfo calls.
		out = append(out, domain.ObjectBucketInfo{
			Bucket:      b.Bucket,
			Description: b.Description,
			Size:        b.Size,
		})
	}
	return out, nil
}

func (c *Client) CreateObjectBucket(ctx context.Context, cfg *nats.ObjectStoreConfig) (*domain.ObjectBucketInfo, error) {
	if _, err := c.inner.Objects().CreateRaw(ctx, cfg); err != nil {
		return nil, err
	}
	return c.GetObjectBucket(ctx, cfg.Bucket)
}

func (c *Client) UpdateObjectBucket(ctx context.Context, cfg *nats.ObjectStoreConfig) (*domain.ObjectBucketInfo, error) {
	if cfg == nil || strings.IsEmpty(cfg.Bucket) {
		return nil, errors.New("bucket is required")
	}
	if err := c.applyObjectStreamConfig(ctx, cfg); err != nil {
		return nil, err
	}
	return c.GetObjectBucket(ctx, cfg.Bucket)
}

func (c *Client) applyObjectStreamConfig(ctx context.Context, cfg *nats.ObjectStoreConfig) error {
	streamName := "OBJ_" + cfg.Bucket
	info, err := c.StreamInfo(ctx, streamName)
	if err != nil {
		return fmt.Errorf("object bucket %q backing stream: %w", cfg.Bucket, err)
	}
	sc := info.Config
	sc.Description = cfg.Description
	sc.MaxAge = cfg.TTL
	maxBytes := cfg.MaxBytes
	if maxBytes == 0 {
		maxBytes = -1
	}
	sc.MaxBytes = maxBytes
	if cfg.Storage != 0 {
		sc.Storage = cfg.Storage
	}
	replicas := cfg.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	sc.Replicas = replicas
	if cfg.Compression {
		sc.Compression = nats.S2Compression
	} else {
		sc.Compression = nats.NoCompression
	}
	if cfg.Placement != nil && (!strings.IsEmpty(cfg.Placement.Cluster) || len(cfg.Placement.Tags) > 0) {
		sc.Placement = &nats.Placement{
			Cluster: cfg.Placement.Cluster,
			Tags:    append([]string(nil), cfg.Placement.Tags...),
		}
	} else {
		sc.Placement = nil
	}
	sc.Metadata = cloneStringMap(cfg.Metadata)
	// Preserve Object Store stream semantics.
	sc.Discard = nats.DiscardNew
	sc.AllowRollup = true
	sc.AllowDirect = true
	if _, err := c.UpdateStream(ctx, &sc); err != nil {
		return fmt.Errorf("object bucket %q apply stream config: %w", cfg.Bucket, err)
	}
	return nil
}

func (c *Client) GetObjectBucket(ctx context.Context, bucket string) (*domain.ObjectBucketInfo, error) {
	status, err := c.inner.Objects().BucketInfo(ctx, bucket)
	if err != nil {
		return nil, err
	}
	info := domain.ObjectBucketInfo{
		Bucket:      status.Bucket,
		Description: status.Description,
		Size:        status.Size,
	}
	streamName := "OBJ_" + bucket
	si, serr := c.StreamInfo(ctx, streamName)
	if serr != nil {
		return &info, nil
	}
	sc := si.Config
	info.Description = sc.Description
	info.TTLNs = int64(sc.MaxAge)
	info.MaxBytes = sc.MaxBytes
	info.Replicas = sc.Replicas
	if info.Replicas <= 0 {
		info.Replicas = 1
	}
	info.Storage = domain.StorageTypeString(sc.Storage)
	info.Compressed = sc.Compression == nats.S2Compression
	info.Sealed = sc.Sealed
	info.Metadata = cloneStringMap(sc.Metadata)
	if sc.Placement != nil && (!strings.IsEmpty(sc.Placement.Cluster) || len(sc.Placement.Tags) > 0) {
		info.Placement = &domain.ObjectPlacement{
			Cluster: sc.Placement.Cluster,
			Tags:    append([]string(nil), sc.Placement.Tags...),
		}
	}
	return &info, nil
}

func (c *Client) DeleteObjectBucket(ctx context.Context, bucket string) error {
	return c.inner.Objects().Delete(ctx, bucket)
}

func (c *Client) ListObjects(ctx context.Context, bucket string, offset, limit int) ([]string, int, error) {
	return c.inner.Objects().ListObjects(ctx, bucket, offset, limit)
}

func (c *Client) GetObject(ctx context.Context, bucket, name string) (*domain.ObjectInfo, error) {
	entry, err := c.inner.Objects().Get(ctx, bucket, name)
	if err != nil {
		return nil, err
	}
	return objectInfoFromLib(entry), nil
}

func (c *Client) PutObject(ctx context.Context, bucket, name string, data []byte) (*domain.ObjectInfo, error) {
	entry, err := c.inner.Objects().Put(ctx, bucket, name, data)
	if err != nil {
		return nil, err
	}
	return objectInfoFromLib(entry), nil
}

func (c *Client) DeleteObject(ctx context.Context, bucket, name string) error {
	return c.inner.Objects().DeleteObject(ctx, bucket, name)
}

func objectInfoFromLib(entry *libnats.ObjectEntry) *domain.ObjectInfo {
	if entry == nil {
		return nil
	}
	return &domain.ObjectInfo{
		Bucket:   entry.Bucket,
		Name:     entry.Name,
		Size:     entry.Size,
		Data:     b64util.EncodeToString(entry.Data),
		Modified: entry.Modified,
	}
}
