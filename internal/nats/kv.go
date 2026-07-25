package natsclient

import (
	"context"

	libnats "github.com/gopherust-io/nats"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/b64util"
	"github.com/nats-io/nats.go"
)

func (c *Client) ListKVBuckets(ctx context.Context) ([]domain.KVBucketInfo, error) {
	buckets, err := c.inner.KV().ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.KVBucketInfo, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, domain.KVBucketInfo{
			Bucket:  b.Bucket,
			Values:  b.Values,
			History: b.History,
		})
	}
	return out, nil
}

func (c *Client) CreateKVBucket(ctx context.Context, cfg *nats.KeyValueConfig) (*domain.KVBucketInfo, error) {
	status, err := c.inner.KV().CreateRaw(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &domain.KVBucketInfo{
		Bucket:  status.Bucket,
		Values:  status.Values,
		History: status.History,
	}, nil
}

func (c *Client) GetKVBucket(ctx context.Context, bucket string) (*domain.KVBucketInfo, error) {
	status, err := c.inner.KV().BucketInfo(ctx, bucket)
	if err != nil {
		return nil, err
	}
	return &domain.KVBucketInfo{
		Bucket:  status.Bucket,
		Values:  status.Values,
		History: status.History,
	}, nil
}

func (c *Client) DeleteKVBucket(ctx context.Context, bucket string) error {
	return c.inner.KV().Delete(ctx, bucket)
}

func (c *Client) ListKVKeys(ctx context.Context, bucket string, offset, limit int) ([]string, int, error) {
	return c.inner.KVKeys().ListKeys(ctx, bucket, offset, limit)
}

func (c *Client) GetKVEntry(ctx context.Context, bucket, key string) (*domain.KVEntry, error) {
	entry, err := c.inner.KVKeys().Get(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	return kvEntryFromLib(entry), nil
}

func (c *Client) PutKVEntry(ctx context.Context, bucket, key string, value []byte) (*domain.KVEntry, error) {
	entry, err := c.inner.KVKeys().Put(ctx, bucket, key, value)
	if err != nil {
		return nil, err
	}
	return kvEntryFromLib(entry), nil
}

func (c *Client) DeleteKVEntry(ctx context.Context, bucket, key string) error {
	return c.inner.KVKeys().DeleteKey(ctx, bucket, key)
}

func (c *Client) KVHistory(ctx context.Context, bucket, key string) ([]domain.KVEntry, error) {
	entries, err := c.inner.KVKeys().History(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	out := make([]domain.KVEntry, 0, len(entries))
	for i := range entries {
		out = append(out, *kvEntryFromLib(&entries[i]))
	}
	return out, nil
}

func kvEntryFromLib(entry *libnats.KVEntry) *domain.KVEntry {
	if entry == nil {
		return nil
	}
	return &domain.KVEntry{
		Bucket:   entry.Bucket,
		Key:      entry.Key,
		Value:    b64util.EncodeToString(entry.Value),
		Revision: entry.Revision,
		Created:  entry.Created,
	}
}
