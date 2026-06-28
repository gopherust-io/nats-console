package natsclient

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/nats-io/nats.go"
)

func (c *Client) ListKVBuckets(ctx context.Context) ([]domain.KVBucketInfo, error) {
	ch := c.js.KeyValueStoreNames()
	var names []string
	for name := range ch {
		names = append(names, name)
	}
	out := make([]domain.KVBucketInfo, 0, len(names))
	for _, name := range names {
		kv, err := c.js.KeyValue(name)
		if err != nil {
			return nil, err
		}
		status, err := kv.Status()
		if err != nil {
			return nil, err
		}
		out = append(out, domain.KVBucketInfo{
			Bucket:  status.Bucket(),
			Values:  status.Values(),
			History: status.History(),
		})
	}
	return out, nil
}

func (c *Client) CreateKVBucket(ctx context.Context, cfg *nats.KeyValueConfig) (*domain.KVBucketInfo, error) {
	kv, err := c.js.CreateKeyValue(cfg)
	if err != nil {
		return nil, err
	}
	status, err := kv.Status()
	if err != nil {
		return nil, err
	}
	return &domain.KVBucketInfo{
		Bucket:  status.Bucket(),
		Values:  status.Values(),
		History: status.History(),
	}, nil
}

func (c *Client) GetKVBucket(ctx context.Context, bucket string) (*domain.KVBucketInfo, error) {
	kv, err := c.js.KeyValue(bucket)
	if err != nil {
		return nil, err
	}
	status, err := kv.Status()
	if err != nil {
		return nil, err
	}
	return &domain.KVBucketInfo{
		Bucket:  status.Bucket(),
		Values:  status.Values(),
		History: status.History(),
	}, nil
}

func (c *Client) DeleteKVBucket(ctx context.Context, bucket string) error {
	return c.js.DeleteKeyValue(bucket)
}

func (c *Client) ListKVKeys(ctx context.Context, bucket string, offset, limit int) ([]string, int, error) {
	kv, err := c.js.KeyValue(bucket)
	if err != nil {
		return nil, 0, err
	}
	keys, err := kv.Keys()
	if errors.Is(err, nats.ErrNoKeysFound) {
		page, total := sliceStrings([]string{}, offset, limit)
		return page, total, nil
	}
	if err != nil {
		return nil, 0, err
	}
	page, total := sliceStrings(keys, offset, limit)
	return page, total, nil
}

func (c *Client) GetKVEntry(ctx context.Context, bucket, key string) (*domain.KVEntry, error) {
	kv, err := c.js.KeyValue(bucket)
	if err != nil {
		return nil, err
	}
	entry, err := kv.Get(key)
	if err != nil {
		return nil, err
	}
	return kvEntryFromNats(bucket, entry), nil
}

func (c *Client) PutKVEntry(ctx context.Context, bucket, key string, value []byte) (*domain.KVEntry, error) {
	kv, err := c.js.KeyValue(bucket)
	if err != nil {
		return nil, err
	}
	revision, err := kv.Put(key, value)
	if err != nil {
		return nil, err
	}
	entry, err := kv.Get(key)
	if err != nil {
		return &domain.KVEntry{
			Bucket:   bucket,
			Key:      key,
			Value:    base64.StdEncoding.EncodeToString(value),
			Revision: revision,
			Created:  time.Now().UTC(),
		}, nil
	}
	return kvEntryFromNats(bucket, entry), nil
}

func (c *Client) DeleteKVEntry(ctx context.Context, bucket, key string) error {
	kv, err := c.js.KeyValue(bucket)
	if err != nil {
		return err
	}
	return kv.Delete(key)
}

func (c *Client) KVHistory(ctx context.Context, bucket, key string) ([]domain.KVEntry, error) {
	kv, err := c.js.KeyValue(bucket)
	if err != nil {
		return nil, err
	}
	entries, err := kv.History(key)
	if err != nil {
		return nil, err
	}
	out := make([]domain.KVEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, *kvEntryFromNats(bucket, e))
	}
	return out, nil
}

func kvEntryFromNats(bucket string, entry nats.KeyValueEntry) *domain.KVEntry {
	return &domain.KVEntry{
		Bucket:   bucket,
		Key:      entry.Key(),
		Value:    base64.StdEncoding.EncodeToString(entry.Value()),
		Revision: entry.Revision(),
		Created:  entry.Created(),
	}
}
