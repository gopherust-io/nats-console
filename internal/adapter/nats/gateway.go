package nats

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/gopherust-io/nats-consol/internal/domain"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/nats-io/nats.go"
)

type Gateway struct {
	inner *natsclient.Manager
}

var _ port.ClusterGateway = (*Gateway)(nil)

func NewGateway(inner *natsclient.Manager) *Gateway {
	return &Gateway{inner: inner}
}

func (g *Gateway) Manager() *natsclient.Manager {
	return g.inner
}

func (g *Gateway) BootstrapDefault(ctx context.Context) error {
	return g.inner.BootstrapDefaultCluster(ctx)
}

func (g *Gateway) Test(ctx context.Context, clusterID string) (domain.ClusterTestResult, error) {
	serverName, jetstream, err := g.inner.Test(ctx, clusterID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, domain.ErrNotFound) {
			return domain.ClusterTestResult{}, domain.ErrNotFound
		}
		return domain.ClusterTestResult{
			OK:      false,
			Message: err.Error(),
		}, nil
	}
	return domain.ClusterTestResult{
		OK:         true,
		Message:    "Available",
		ServerName: serverName,
		JetStream:  jetstream,
	}, nil
}

func (g *Gateway) ConnectionStatus(ctx context.Context, clusterID string) (domain.NATSConnectionStatus, error) {
	status, err := g.inner.Status(ctx, clusterID)
	if err != nil {
		return domain.NATSConnectionStatus{}, mapGatewayErr(err)
	}
	return status, nil
}

func (g *Gateway) ListConnectionStatuses(ctx context.Context) []domain.NATSConnectionStatus {
	return g.inner.ListStatuses()
}

func (g *Gateway) Evict(clusterID string) {
	g.inner.Evict(clusterID)
	g.inner.InvalidateViews(clusterID)
}

func (g *Gateway) Touch(clusterID string) {
	g.inner.Touch(clusterID)
}

func (g *Gateway) Close() {
	g.inner.Close()
}

func (g *Gateway) InvalidateViews(clusterID string) {
	g.inner.InvalidateViews(clusterID)
}

func (g *Gateway) WithExecutor(ctx context.Context, clusterID string, fn func(port.JetStreamExecutor) error) error {
	client, err := g.inner.Get(ctx, clusterID)
	if err != nil {
		return mapGatewayErr(err)
	}
	return fn(g.wrap(clusterID, client))
}

func (g *Gateway) GetExecutor(ctx context.Context, clusterID string) (port.JetStreamExecutor, error) {
	client, err := g.inner.Get(ctx, clusterID)
	if err != nil {
		return nil, mapGatewayErr(err)
	}
	return g.wrap(clusterID, client), nil
}

func (g *Gateway) wrap(clusterID string, client *natsclient.Client) port.JetStreamExecutor {
	return &cachingExecutor{
		Executor:  Executor{client: client},
		clusterID: clusterID,
		views:     g.inner.ViewCache(),
		invalidate: func() {
			g.inner.InvalidateViews(clusterID)
		},
	}
}

func mapGatewayErr(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return domain.ErrNotFound
	}
	return err
}

type Executor struct {
	client *natsclient.Client
}

type cachingExecutor struct {
	Executor

	clusterID  string
	views      *natsclient.ViewCache
	invalidate func()
	lastETag   string
}

func (e *cachingExecutor) LastETag() string {
	return e.lastETag
}

func (e *Executor) JetStream() nats.JetStreamContext {
	return e.client.JetStream()
}

func (e *Executor) AccountInfo(ctx context.Context) (*nats.AccountInfo, error) {
	return e.client.AccountInfo(ctx)
}

func (e *cachingExecutor) AccountInfo(ctx context.Context) (*nats.AccountInfo, error) {
	key := natsclient.ViewCacheKey(e.clusterID, "account")
	v, etag, err := e.views.GetOrLoad(key, func() (any, error) {
		return e.client.AccountInfo(ctx)
	})
	if err != nil {
		return nil, err
	}
	e.lastETag = etag
	info, ok := v.(*nats.AccountInfo)
	if !ok {
		return nil, fmt.Errorf("view cache: unexpected account info type %T", v)
	}
	return info, nil
}

func (e *Executor) StreamNames(ctx context.Context) ([]string, error) {
	return e.client.StreamNames(ctx)
}

func (e *Executor) ListStreams(ctx context.Context, offset, limit int) ([]*nats.StreamInfo, int, error) {
	return e.client.ListStreams(ctx, offset, limit)
}

func (e *cachingExecutor) ListStreams(ctx context.Context, offset, limit int) ([]*nats.StreamInfo, int, error) {
	key := natsclient.ViewCacheKey(e.clusterID, "streams", strconv.Itoa(offset), strconv.Itoa(limit))
	type page struct {
		Streams []*nats.StreamInfo
		Total   int
	}
	v, etag, err := e.views.GetOrLoad(key, func() (any, error) {
		streams, total, err := e.client.ListStreams(ctx, offset, limit)
		if err != nil {
			return nil, err
		}
		return page{Streams: streams, Total: total}, nil
	})
	if err != nil {
		return nil, 0, err
	}
	e.lastETag = etag
	p := v.(page)
	return p.Streams, p.Total, nil
}

func (e *Executor) StreamInfo(ctx context.Context, name string) (*nats.StreamInfo, error) {
	return e.client.StreamInfo(ctx, name)
}

func (e *Executor) AddStream(ctx context.Context, cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	return e.client.AddStream(ctx, cfg)
}

func (e *cachingExecutor) AddStream(ctx context.Context, cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	info, err := e.client.AddStream(ctx, cfg)
	if err == nil {
		e.invalidate()
	}
	return info, err
}

func (e *Executor) UpdateStream(ctx context.Context, cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	return e.client.UpdateStream(ctx, cfg)
}

func (e *cachingExecutor) UpdateStream(ctx context.Context, cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	info, err := e.client.UpdateStream(ctx, cfg)
	if err == nil {
		e.invalidate()
	}
	return info, err
}

func (e *Executor) DeleteStream(ctx context.Context, name string) error {
	return e.client.DeleteStream(ctx, name)
}

func (e *cachingExecutor) DeleteStream(ctx context.Context, name string) error {
	err := e.client.DeleteStream(ctx, name)
	if err == nil {
		e.invalidate()
	}
	return err
}

func (e *Executor) PurgeStream(ctx context.Context, name string) error {
	return e.client.PurgeStream(ctx, name)
}

func (e *cachingExecutor) PurgeStream(ctx context.Context, name string) error {
	err := e.client.PurgeStream(ctx, name)
	if err == nil {
		e.invalidate()
	}
	return err
}

func (e *Executor) ConsumerNames(ctx context.Context, stream string) ([]string, error) {
	return e.client.ConsumerNames(ctx, stream)
}

func (e *Executor) ListConsumers(ctx context.Context, stream string, offset, limit int) ([]*nats.ConsumerInfo, int, error) {
	return e.client.ListConsumers(ctx, stream, offset, limit)
}

func (e *cachingExecutor) ListConsumers(ctx context.Context, stream string, offset, limit int) ([]*nats.ConsumerInfo, int, error) {
	key := natsclient.ViewCacheKey(e.clusterID, "consumers", stream, strconv.Itoa(offset), strconv.Itoa(limit))
	type page struct {
		Consumers []*nats.ConsumerInfo
		Total     int
	}
	v, etag, err := e.views.GetOrLoad(key, func() (any, error) {
		consumers, total, err := e.client.ListConsumers(ctx, stream, offset, limit)
		if err != nil {
			return nil, err
		}
		return page{Consumers: consumers, Total: total}, nil
	})
	if err != nil {
		return nil, 0, err
	}
	e.lastETag = etag
	p := v.(page)
	return p.Consumers, p.Total, nil
}

func (e *Executor) ConsumerInfo(ctx context.Context, stream, consumer string) (*nats.ConsumerInfo, error) {
	return e.client.ConsumerInfo(ctx, stream, consumer)
}

func (e *Executor) AddConsumer(ctx context.Context, stream string, cfg *nats.ConsumerConfig) (*nats.ConsumerInfo, error) {
	return e.client.AddConsumer(ctx, stream, cfg)
}

func (e *cachingExecutor) AddConsumer(ctx context.Context, stream string, cfg *nats.ConsumerConfig) (*nats.ConsumerInfo, error) {
	info, err := e.client.AddConsumer(ctx, stream, cfg)
	if err == nil {
		e.invalidate()
	}
	return info, err
}

func (e *Executor) UpdateConsumer(ctx context.Context, stream string, cfg *nats.ConsumerConfig) (*nats.ConsumerInfo, error) {
	return e.client.UpdateConsumer(ctx, stream, cfg)
}

func (e *cachingExecutor) UpdateConsumer(ctx context.Context, stream string, cfg *nats.ConsumerConfig) (*nats.ConsumerInfo, error) {
	info, err := e.client.UpdateConsumer(ctx, stream, cfg)
	if err == nil {
		e.invalidate()
	}
	return info, err
}

func (e *Executor) DeleteConsumer(ctx context.Context, stream, consumer string) error {
	return e.client.DeleteConsumer(ctx, stream, consumer)
}

func (e *cachingExecutor) DeleteConsumer(ctx context.Context, stream, consumer string) error {
	err := e.client.DeleteConsumer(ctx, stream, consumer)
	if err == nil {
		e.invalidate()
	}
	return err
}

func (e *Executor) ReplayConsumer(ctx context.Context, stream, consumer string, req domain.ReplayConsumerRequest) (domain.ReplayConsumerResult, error) {
	return e.client.ReplayConsumer(ctx, stream, consumer, req)
}

func (e *Executor) GetMessage(ctx context.Context, stream string, seq uint64) (*nats.RawStreamMsg, error) {
	return e.client.GetMessage(ctx, stream, seq)
}

func (e *Executor) GetMessageNav(ctx context.Context, stream string, seq uint64, direction string) (*domain.MessageResult, error) {
	return e.client.GetMessageNav(ctx, stream, seq, direction)
}

func (e *Executor) PublishStreamMessage(ctx context.Context, stream string, in domain.PublishMessageRequest) (domain.PublishMessageResult, error) {
	return e.client.PublishStreamMessage(ctx, stream, in)
}

func (e *cachingExecutor) PublishStreamMessage(ctx context.Context, stream string, in domain.PublishMessageRequest) (domain.PublishMessageResult, error) {
	res, err := e.client.PublishStreamMessage(ctx, stream, in)
	if err == nil {
		e.invalidate()
	}
	return res, err
}

func (e *Executor) Monitoring(ctx context.Context, path string) ([]byte, error) {
	return e.client.Monitoring(ctx, path)
}

func (e *cachingExecutor) Monitoring(ctx context.Context, path string) ([]byte, error) {
	key := natsclient.ViewCacheKey(e.clusterID, "monitoring", path)
	v, etag, err := e.views.GetOrLoad(key, func() (any, error) {
		return e.client.Monitoring(ctx, path)
	})
	if err != nil {
		return nil, err
	}
	e.lastETag = etag
	raw, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf("view cache: unexpected monitoring type %T", v)
	}
	return raw, nil
}

func (e *Executor) ListKVBuckets(ctx context.Context) ([]domain.KVBucketInfo, error) {
	return e.client.ListKVBuckets(ctx)
}

func (e *cachingExecutor) ListKVBuckets(ctx context.Context) ([]domain.KVBucketInfo, error) {
	key := natsclient.ViewCacheKey(e.clusterID, "kv_buckets")
	v, etag, err := e.views.GetOrLoad(key, func() (any, error) {
		return e.client.ListKVBuckets(ctx)
	})
	if err != nil {
		return nil, err
	}
	e.lastETag = etag
	return v.([]domain.KVBucketInfo), nil
}

func (e *Executor) CreateKVBucket(ctx context.Context, cfg *nats.KeyValueConfig, opts domain.KVBucketWriteOpts) (*domain.KVBucketInfo, error) {
	return e.client.CreateKVBucket(ctx, cfg, opts)
}

func (e *cachingExecutor) CreateKVBucket(ctx context.Context, cfg *nats.KeyValueConfig, opts domain.KVBucketWriteOpts) (*domain.KVBucketInfo, error) {
	info, err := e.client.CreateKVBucket(ctx, cfg, opts)
	if err == nil {
		e.invalidate()
	}
	return info, err
}

func (e *Executor) UpdateKVBucket(ctx context.Context, cfg *nats.KeyValueConfig, opts domain.KVBucketWriteOpts) (*domain.KVBucketInfo, error) {
	return e.client.UpdateKVBucket(ctx, cfg, opts)
}

func (e *cachingExecutor) UpdateKVBucket(ctx context.Context, cfg *nats.KeyValueConfig, opts domain.KVBucketWriteOpts) (*domain.KVBucketInfo, error) {
	info, err := e.client.UpdateKVBucket(ctx, cfg, opts)
	if err == nil {
		e.invalidate()
	}
	return info, err
}

func (e *Executor) GetKVBucket(ctx context.Context, bucket string) (*domain.KVBucketInfo, error) {
	return e.client.GetKVBucket(ctx, bucket)
}

func (e *Executor) DeleteKVBucket(ctx context.Context, bucket string) error {
	return e.client.DeleteKVBucket(ctx, bucket)
}

func (e *cachingExecutor) DeleteKVBucket(ctx context.Context, bucket string) error {
	err := e.client.DeleteKVBucket(ctx, bucket)
	if err == nil {
		e.invalidate()
	}
	return err
}

func (e *Executor) ListKVKeys(ctx context.Context, bucket string, offset, limit int) ([]string, int, error) {
	return e.client.ListKVKeys(ctx, bucket, offset, limit)
}

func (e *Executor) GetKVEntry(ctx context.Context, bucket, key string) (*domain.KVEntry, error) {
	return e.client.GetKVEntry(ctx, bucket, key)
}

func (e *Executor) PutKVEntry(ctx context.Context, bucket, key string, value []byte) (*domain.KVEntry, error) {
	return e.client.PutKVEntry(ctx, bucket, key, value)
}

func (e *cachingExecutor) PutKVEntry(ctx context.Context, bucket, key string, value []byte) (*domain.KVEntry, error) {
	entry, err := e.client.PutKVEntry(ctx, bucket, key, value)
	if err == nil {
		e.invalidate()
	}
	return entry, err
}

func (e *Executor) DeleteKVEntry(ctx context.Context, bucket, key string) error {
	return e.client.DeleteKVEntry(ctx, bucket, key)
}

func (e *cachingExecutor) DeleteKVEntry(ctx context.Context, bucket, key string) error {
	err := e.client.DeleteKVEntry(ctx, bucket, key)
	if err == nil {
		e.invalidate()
	}
	return err
}

func (e *Executor) KVHistory(ctx context.Context, bucket, key string) ([]domain.KVEntry, error) {
	return e.client.KVHistory(ctx, bucket, key)
}

func (e *Executor) ListObjectBuckets(ctx context.Context) ([]domain.ObjectBucketInfo, error) {
	return e.client.ListObjectBuckets(ctx)
}

func (e *cachingExecutor) ListObjectBuckets(ctx context.Context) ([]domain.ObjectBucketInfo, error) {
	key := natsclient.ViewCacheKey(e.clusterID, "obj_buckets")
	v, etag, err := e.views.GetOrLoad(key, func() (any, error) {
		return e.client.ListObjectBuckets(ctx)
	})
	if err != nil {
		return nil, err
	}
	e.lastETag = etag
	return v.([]domain.ObjectBucketInfo), nil
}

func (e *Executor) CreateObjectBucket(ctx context.Context, cfg *nats.ObjectStoreConfig) (*domain.ObjectBucketInfo, error) {
	return e.client.CreateObjectBucket(ctx, cfg)
}

func (e *cachingExecutor) CreateObjectBucket(ctx context.Context, cfg *nats.ObjectStoreConfig) (*domain.ObjectBucketInfo, error) {
	info, err := e.client.CreateObjectBucket(ctx, cfg)
	if err == nil {
		e.invalidate()
	}
	return info, err
}

func (e *Executor) UpdateObjectBucket(ctx context.Context, cfg *nats.ObjectStoreConfig) (*domain.ObjectBucketInfo, error) {
	return e.client.UpdateObjectBucket(ctx, cfg)
}

func (e *cachingExecutor) UpdateObjectBucket(ctx context.Context, cfg *nats.ObjectStoreConfig) (*domain.ObjectBucketInfo, error) {
	info, err := e.client.UpdateObjectBucket(ctx, cfg)
	if err == nil {
		e.invalidate()
	}
	return info, err
}

func (e *Executor) GetObjectBucket(ctx context.Context, bucket string) (*domain.ObjectBucketInfo, error) {
	return e.client.GetObjectBucket(ctx, bucket)
}

func (e *Executor) DeleteObjectBucket(ctx context.Context, bucket string) error {
	return e.client.DeleteObjectBucket(ctx, bucket)
}

func (e *cachingExecutor) DeleteObjectBucket(ctx context.Context, bucket string) error {
	err := e.client.DeleteObjectBucket(ctx, bucket)
	if err == nil {
		e.invalidate()
	}
	return err
}

func (e *Executor) ListObjects(ctx context.Context, bucket string, offset, limit int) ([]string, int, error) {
	return e.client.ListObjects(ctx, bucket, offset, limit)
}

func (e *Executor) GetObject(ctx context.Context, bucket, name string) (*domain.ObjectInfo, error) {
	return e.client.GetObject(ctx, bucket, name)
}

func (e *Executor) PutObject(ctx context.Context, bucket, name string, data []byte) (*domain.ObjectInfo, error) {
	return e.client.PutObject(ctx, bucket, name, data)
}

func (e *cachingExecutor) PutObject(ctx context.Context, bucket, name string, data []byte) (*domain.ObjectInfo, error) {
	info, err := e.client.PutObject(ctx, bucket, name, data)
	if err == nil {
		e.invalidate()
	}
	return info, err
}

func (e *Executor) DeleteObject(ctx context.Context, bucket, name string) error {
	return e.client.DeleteObject(ctx, bucket, name)
}

func (e *cachingExecutor) DeleteObject(ctx context.Context, bucket, name string) error {
	err := e.client.DeleteObject(ctx, bucket, name)
	if err == nil {
		e.invalidate()
	}
	return err
}

func (e *Executor) Conn() *nats.Conn {
	return e.client.Conn()
}
