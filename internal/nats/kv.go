package natsclient

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/nats-io/nats.go"
)

type KVBucketInfo struct {
	Bucket  string `json:"bucket"`
	Values  uint64 `json:"values"`
	History int64  `json:"history"`
}

type KVEntry struct {
	Bucket   string    `json:"bucket"`
	Key      string    `json:"key"`
	Value    string    `json:"value"`
	Revision uint64    `json:"revision"`
	Created  time.Time `json:"created"`
}

func (c *Client) ListKVBuckets(ctx context.Context) ([]KVBucketInfo, error) {
	ch := c.js.KeyValueStoreNames()
	var names []string
	for name := range ch {
		names = append(names, name)
	}
	out := make([]KVBucketInfo, 0, len(names))
	for _, name := range names {
		kv, err := c.js.KeyValue(name)
		if err != nil {
			return nil, err
		}
		status, err := kv.Status()
		if err != nil {
			return nil, err
		}
		out = append(out, KVBucketInfo{
			Bucket:  status.Bucket(),
			Values:  status.Values(),
			History: status.History(),
		})
	}
	return out, nil
}

func (c *Client) CreateKVBucket(ctx context.Context, cfg *nats.KeyValueConfig) (*KVBucketInfo, error) {
	kv, err := c.js.CreateKeyValue(cfg)
	if err != nil {
		return nil, err
	}
	status, err := kv.Status()
	if err != nil {
		return nil, err
	}
	return &KVBucketInfo{
		Bucket:  status.Bucket(),
		Values:  status.Values(),
		History: status.History(),
	}, nil
}

func (c *Client) GetKVBucket(ctx context.Context, bucket string) (*KVBucketInfo, error) {
	kv, err := c.js.KeyValue(bucket)
	if err != nil {
		return nil, err
	}
	status, err := kv.Status()
	if err != nil {
		return nil, err
	}
	return &KVBucketInfo{
		Bucket:  status.Bucket(),
		Values:  status.Values(),
		History: status.History(),
	}, nil
}

func (c *Client) DeleteKVBucket(ctx context.Context, bucket string) error {
	return c.js.DeleteKeyValue(bucket)
}

func (c *Client) ListKVKeys(ctx context.Context, bucket string) ([]string, error) {
	kv, err := c.js.KeyValue(bucket)
	if err != nil {
		return nil, err
	}
	keys, err := kv.Keys()
	if err == nats.ErrNoKeysFound {
		return []string{}, nil
	}
	return keys, err
}

func (c *Client) GetKVEntry(ctx context.Context, bucket, key string) (*KVEntry, error) {
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

func (c *Client) PutKVEntry(ctx context.Context, bucket, key string, value []byte) (*KVEntry, error) {
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
		return &KVEntry{
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

func (c *Client) KVHistory(ctx context.Context, bucket, key string) ([]KVEntry, error) {
	kv, err := c.js.KeyValue(bucket)
	if err != nil {
		return nil, err
	}
	entries, err := kv.History(key)
	if err != nil {
		return nil, err
	}
	out := make([]KVEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, *kvEntryFromNats(bucket, e))
	}
	return out, nil
}

func kvEntryFromNats(bucket string, entry nats.KeyValueEntry) *KVEntry {
	return &KVEntry{
		Bucket:   bucket,
		Key:      entry.Key(),
		Value:    base64.StdEncoding.EncodeToString(entry.Value()),
		Revision: entry.Revision(),
		Created:  entry.Created(),
	}
}
