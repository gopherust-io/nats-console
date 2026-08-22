package natsclient

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	libnats "github.com/gopherust-io/nats"
	"github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/b64util"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func kvBucketInfoFromStatus(st nats.KeyValueStatus) domain.KVBucketInfo {
	cfg := st.Config()
	info := domain.KVBucketInfo{
		Bucket:       st.Bucket(),
		Description:  cfg.Description,
		Values:       st.Values(),
		Bytes:        st.Bytes(),
		History:      st.History(),
		TTLNs:        int64(cfg.TTL),
		MaxValueSize: cfg.MaxValueSize,
		MaxBytes:     cfg.MaxBytes,
		Replicas:     max(cfg.Replicas, 1),
		Compressed:   st.IsCompressed() || cfg.Compression,
		Storage:      domain.StorageTypeString(cfg.Storage),
	}
	if info.History <= 0 {
		info.History = int64(cfg.History)
	}
	if cfg.Placement != nil && (!strings.IsEmpty(cfg.Placement.Cluster) || len(cfg.Placement.Tags) > 0) {
		info.Placement = &domain.KVPlacement{
			Cluster: cfg.Placement.Cluster,
			Tags:    append([]string(nil), cfg.Placement.Tags...),
		}
	}
	if cfg.RePublish != nil && !strings.IsEmpty(cfg.RePublish.Destination) {
		info.RePublish = &domain.KVRePublish{
			Source:      cfg.RePublish.Source,
			Destination: cfg.RePublish.Destination,
			HeadersOnly: cfg.RePublish.HeadersOnly,
		}
	}
	if cfg.Mirror != nil && !strings.IsEmpty(cfg.Mirror.Name) {
		info.Mirror = &domain.KVStreamSource{
			Name:          cfg.Mirror.Name,
			FilterSubject: cfg.Mirror.FilterSubject,
		}
	}
	if len(cfg.Sources) > 0 {
		info.Sources = make([]domain.KVStreamSource, 0, len(cfg.Sources))
		for _, src := range cfg.Sources {
			if src == nil || strings.IsEmpty(src.Name) {
				continue
			}
			info.Sources = append(info.Sources, domain.KVStreamSource{
				Name:          src.Name,
				FilterSubject: src.FilterSubject,
			})
		}
	}
	if si, ok := st.(interface{ StreamInfo() *nats.StreamInfo }); ok {
		if nfo := si.StreamInfo(); nfo != nil {
			info.LimitMarkerTTL = int64(nfo.Config.SubjectDeleteMarkerTTL)
			if len(nfo.Config.Metadata) > 0 {
				info.Metadata = cloneStringMap(nfo.Config.Metadata)
			}
			if info.Compressed && nfo.Config.Compression == nats.NoCompression {
				info.Compressed = false
			}
			if nfo.Config.Compression == nats.S2Compression {
				info.Compressed = true
			}
		}
	}
	return info
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	maps.Copy(out, in)
	return out
}

func (c *Client) ListKVBuckets(ctx context.Context) ([]domain.KVBucketInfo, error) {
	buckets, err := c.natsCl.KV().ListBuckets(ctx)
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

func (c *Client) CreateKVBucket(ctx context.Context, cfg *nats.KeyValueConfig, opts domain.KVBucketWriteOpts) (*domain.KVBucketInfo, error) {
	if _, err := c.natsCl.KV().CreateRaw(ctx, cfg); err != nil {
		return nil, err
	}
	if err := c.applyKVStreamExtras(ctx, cfg, time.Duration(opts.LimitMarkerTTLNs), opts.Metadata); err != nil {
		return nil, err
	}
	return c.GetKVBucket(ctx, cfg.Bucket)
}

func (c *Client) UpdateKVBucket(ctx context.Context, cfg *nats.KeyValueConfig, opts domain.KVBucketWriteOpts) (*domain.KVBucketInfo, error) {
	if cfg == nil || strings.IsEmpty(cfg.Bucket) {
		return nil, errors.New("bucket is required")
	}
	libCfg := libnats.KeyValueConfig{
		Bucket:      cfg.Bucket,
		Description: cfg.Description,
		History:     cfg.History,
		TTL:         cfg.TTL,
		MaxBytes:    cfg.MaxBytes,
		Replicas:    cfg.Replicas,
		Compression: cfg.Compression,
	}
	if cfg.Storage != 0 {
		libCfg.Storage = cfg.Storage
	}
	if _, err := c.natsCl.KV().CreateOrUpdate(ctx, libCfg); err != nil {
		return nil, err
	}
	if err := c.applyKVStreamExtras(ctx, cfg, time.Duration(opts.LimitMarkerTTLNs), opts.Metadata); err != nil {
		return nil, err
	}
	return c.GetKVBucket(ctx, cfg.Bucket)
}

func (c *Client) applyKVStreamExtras(ctx context.Context, cfg *nats.KeyValueConfig, limitMarkerTTL time.Duration, metadata map[string]string) error {
	const kvPrefix = "KV_"
	streamName := kvPrefix + cfg.Bucket
	info, err := c.StreamInfo(ctx, streamName)
	if err != nil {
		return fmt.Errorf("kv bucket %q backing stream: %w", cfg.Bucket, err)
	}
	sc := info.Config
	sc.Description = cfg.Description
	sc.MaxAge = cfg.TTL
	sc.MaxBytes = cfg.MaxBytes
	if cfg.History > 0 {
		sc.MaxMsgsPerSubject = int64(cfg.History)
	}
	if cfg.Replicas > 0 {
		sc.Replicas = cfg.Replicas
	}
	if cfg.MaxValueSize != 0 {
		sc.MaxMsgSize = cfg.MaxValueSize
	} else {
		sc.MaxMsgSize = -1
	}
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
	sc.RePublish = cfg.RePublish
	sc.Mirror = cfg.Mirror
	sc.Sources = cfg.Sources
	sc.SubjectDeleteMarkerTTL = max(limitMarkerTTL, 0)
	sc.Metadata = cloneStringMap(metadata)
	if _, err := c.UpdateStream(ctx, &sc); err != nil {
		return fmt.Errorf("kv bucket %q apply advanced stream options: %w", cfg.Bucket, err)
	}
	return nil
}

func (c *Client) GetKVBucket(ctx context.Context, bucket string) (*domain.KVBucketInfo, error) {
	kv, err := c.natsCl.KV().Open(ctx, bucket)
	if err != nil {
		return nil, err
	}
	st, err := kv.Status()
	if err != nil {
		return nil, err
	}
	info := kvBucketInfoFromStatus(st)
	return &info, nil
}

func (c *Client) DeleteKVBucket(ctx context.Context, bucket string) error {
	return c.natsCl.KV().Delete(ctx, bucket)
}

func (c *Client) ListKVKeys(ctx context.Context, bucket string, offset, limit int) ([]string, int, error) {
	return c.natsCl.KVKeys().ListKeys(ctx, bucket, offset, limit)
}

func (c *Client) GetKVEntry(ctx context.Context, bucket, key string) (*domain.KVEntry, error) {
	entry, err := c.natsCl.KVKeys().Get(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	return kvEntryFromLib(entry), nil
}

func (c *Client) PutKVEntry(ctx context.Context, bucket, key string, value []byte) (*domain.KVEntry, error) {
	entry, err := c.natsCl.KVKeys().Put(ctx, bucket, key, value)
	if err != nil {
		return nil, err
	}
	return kvEntryFromLib(entry), nil
}

func (c *Client) DeleteKVEntry(ctx context.Context, bucket, key string) error {
	return c.natsCl.KVKeys().DeleteKey(ctx, bucket, key)
}

func (c *Client) KVHistory(ctx context.Context, bucket, key string) ([]domain.KVEntry, error) {
	entries, err := c.natsCl.KVKeys().History(ctx, bucket, key)
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
