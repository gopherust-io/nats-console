package natsclient

import (
	"context"

	libnats "github.com/gopherust-io/nats"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/b64util"
	"github.com/nats-io/nats.go"
)

func (c *Client) ListObjectBuckets(ctx context.Context) ([]domain.ObjectBucketInfo, error) {
	buckets, err := c.inner.Objects().ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ObjectBucketInfo, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, domain.ObjectBucketInfo{
			Bucket:      b.Bucket,
			Description: b.Description,
			Size:        b.Size,
		})
	}
	return out, nil
}

func (c *Client) CreateObjectBucket(ctx context.Context, cfg *nats.ObjectStoreConfig) (*domain.ObjectBucketInfo, error) {
	status, err := c.inner.Objects().CreateRaw(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &domain.ObjectBucketInfo{
		Bucket:      status.Bucket,
		Description: status.Description,
		Size:        status.Size,
	}, nil
}

func (c *Client) GetObjectBucket(ctx context.Context, bucket string) (*domain.ObjectBucketInfo, error) {
	status, err := c.inner.Objects().BucketInfo(ctx, bucket)
	if err != nil {
		return nil, err
	}
	return &domain.ObjectBucketInfo{
		Bucket:      status.Bucket,
		Description: status.Description,
		Size:        status.Size,
	}, nil
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
