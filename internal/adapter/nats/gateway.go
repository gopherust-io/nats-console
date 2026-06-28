package nats

import (
	"context"
	"errors"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/internal/store"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/nats-io/nats.go"
)

type Gateway struct {
	inner *natsclient.Manager
}

var _ port.ClusterGateway = (*Gateway)(nil)

func NewGateway(inner *natsclient.Manager) *Gateway {
	return &Gateway{inner: inner}
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
		Message:    "connected",
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
}

func (g *Gateway) Close() {
	g.inner.Close()
}

func (g *Gateway) WithExecutor(ctx context.Context, clusterID string, fn func(port.JetStreamExecutor) error) error {
	client, err := g.inner.Get(ctx, clusterID)
	if err != nil {
		return mapGatewayErr(err)
	}
	return fn(&Executor{client: client})
}

func (g *Gateway) GetExecutor(ctx context.Context, clusterID string) (port.JetStreamExecutor, error) {
	client, err := g.inner.Get(ctx, clusterID)
	if err != nil {
		return nil, mapGatewayErr(err)
	}
	return &Executor{client: client}, nil
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

func (e *Executor) JetStream() nats.JetStreamContext {
	return e.client.JetStream()
}

func (e *Executor) AccountInfo(ctx context.Context) (*nats.AccountInfo, error) {
	return e.client.AccountInfo(ctx)
}

func (e *Executor) StreamNames(ctx context.Context) ([]string, error) {
	return e.client.StreamNames(ctx)
}

func (e *Executor) ListStreams(ctx context.Context, offset, limit int) ([]*nats.StreamInfo, int, error) {
	return e.client.ListStreams(ctx, offset, limit)
}

func (e *Executor) StreamInfo(ctx context.Context, name string) (*nats.StreamInfo, error) {
	return e.client.StreamInfo(ctx, name)
}

func (e *Executor) AddStream(ctx context.Context, cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	return e.client.AddStream(ctx, cfg)
}

func (e *Executor) UpdateStream(ctx context.Context, cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	return e.client.UpdateStream(ctx, cfg)
}

func (e *Executor) DeleteStream(ctx context.Context, name string) error {
	return e.client.DeleteStream(ctx, name)
}

func (e *Executor) PurgeStream(ctx context.Context, name string) error {
	return e.client.PurgeStream(ctx, name)
}

func (e *Executor) ConsumerNames(ctx context.Context, stream string) ([]string, error) {
	return e.client.ConsumerNames(ctx, stream)
}

func (e *Executor) ListConsumers(ctx context.Context, stream string, offset, limit int) ([]*nats.ConsumerInfo, int, error) {
	return e.client.ListConsumers(ctx, stream, offset, limit)
}

func (e *Executor) ConsumerInfo(ctx context.Context, stream, consumer string) (*nats.ConsumerInfo, error) {
	return e.client.ConsumerInfo(ctx, stream, consumer)
}

func (e *Executor) AddConsumer(ctx context.Context, stream string, cfg *nats.ConsumerConfig) (*nats.ConsumerInfo, error) {
	return e.client.AddConsumer(ctx, stream, cfg)
}

func (e *Executor) DeleteConsumer(ctx context.Context, stream, consumer string) error {
	return e.client.DeleteConsumer(ctx, stream, consumer)
}

func (e *Executor) GetMessage(ctx context.Context, stream string, seq uint64) (*nats.RawStreamMsg, error) {
	return e.client.GetMessage(ctx, stream, seq)
}

func (e *Executor) GetMessageNav(ctx context.Context, stream string, seq uint64, direction string) (*domain.MessageResult, error) {
	return e.client.GetMessageNav(ctx, stream, seq, direction)
}

func (e *Executor) Monitoring(ctx context.Context, path string) ([]byte, error) {
	return e.client.Monitoring(ctx, path)
}

func (e *Executor) ListKVBuckets(ctx context.Context) ([]domain.KVBucketInfo, error) {
	return e.client.ListKVBuckets(ctx)
}

func (e *Executor) CreateKVBucket(ctx context.Context, cfg *nats.KeyValueConfig) (*domain.KVBucketInfo, error) {
	return e.client.CreateKVBucket(ctx, cfg)
}

func (e *Executor) GetKVBucket(ctx context.Context, bucket string) (*domain.KVBucketInfo, error) {
	return e.client.GetKVBucket(ctx, bucket)
}

func (e *Executor) DeleteKVBucket(ctx context.Context, bucket string) error {
	return e.client.DeleteKVBucket(ctx, bucket)
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

func (e *Executor) DeleteKVEntry(ctx context.Context, bucket, key string) error {
	return e.client.DeleteKVEntry(ctx, bucket, key)
}

func (e *Executor) KVHistory(ctx context.Context, bucket, key string) ([]domain.KVEntry, error) {
	return e.client.KVHistory(ctx, bucket, key)
}

func (e *Executor) ListObjectBuckets(ctx context.Context) ([]domain.ObjectBucketInfo, error) {
	return e.client.ListObjectBuckets(ctx)
}

func (e *Executor) CreateObjectBucket(ctx context.Context, cfg *nats.ObjectStoreConfig) (*domain.ObjectBucketInfo, error) {
	return e.client.CreateObjectBucket(ctx, cfg)
}

func (e *Executor) GetObjectBucket(ctx context.Context, bucket string) (*domain.ObjectBucketInfo, error) {
	return e.client.GetObjectBucket(ctx, bucket)
}

func (e *Executor) DeleteObjectBucket(ctx context.Context, bucket string) error {
	return e.client.DeleteObjectBucket(ctx, bucket)
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

func (e *Executor) DeleteObject(ctx context.Context, bucket, name string) error {
	return e.client.DeleteObject(ctx, bucket, name)
}

func (e *Executor) Conn() *nats.Conn {
	return e.client.Conn()
}
