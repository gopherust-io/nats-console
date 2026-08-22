package nats

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/internal/repo"
)

type Gateway struct {
	manager *natsclient.Manager
}

var _ port.ClusterGateway = (*Gateway)(nil)

func NewGateway(manager *natsclient.Manager) *Gateway {
	return &Gateway{manager: manager}
}

func (g *Gateway) Manager() *natsclient.Manager {
	return g.manager
}

func (g *Gateway) BootstrapDefault(ctx context.Context) error {
	return g.manager.BootstrapDefaultCluster(ctx)
}

func (g *Gateway) Test(ctx context.Context, clusterID string) (domain.ClusterTestResult, error) {
	serverName, js, err := g.manager.Test(ctx, clusterID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) || errors.Is(err, domain.ErrNotFound) {
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
		JetStream:  js,
	}, nil
}

func (g *Gateway) ConnectionStatus(ctx context.Context, clusterID string) (domain.NATSConnectionStatus, error) {
	status, err := g.manager.Status(ctx, clusterID)
	if err != nil {
		return domain.NATSConnectionStatus{}, mapGatewayErr(err)
	}
	return status, nil
}

func (g *Gateway) ListConnectionStatuses(ctx context.Context) []domain.NATSConnectionStatus {
	return g.manager.ListStatuses()
}

func (g *Gateway) SubscribeConnectionStatus(clusterID string) (updates <-chan domain.NATSConnectionStatus, latest domain.NATSConnectionStatus, unsubscribe func()) {
	return g.manager.SubscribeStatus(clusterID)
}

func (g *Gateway) Evict(clusterID string) {
	g.manager.Evict(clusterID)
	g.manager.InvalidateViews(clusterID)
}

func (g *Gateway) Touch(clusterID string) {
	g.manager.Touch(clusterID)
}

func (g *Gateway) Stop() {
	g.manager.Stop()
}

func (g *Gateway) InvalidateViews(clusterID string) {
	g.manager.InvalidateViews(clusterID)
}

func (g *Gateway) WithExecutor(ctx context.Context, clusterID string, fn func(port.JetStreamExecutor) error) error {
	ctx, cancel := g.scopedContext(ctx)
	defer cancel()

	session, err := g.manager.Session(ctx, clusterID)
	if err != nil {
		metrics.IncNATSExecutorError(clusterID)
		return mapGatewayErr(err)
	}
	defer func() { _ = session.Close() }()

	client, err := session.Client()
	if err != nil {
		metrics.IncNATSExecutorError(clusterID)
		return mapGatewayErr(err)
	}

	if err := fn(g.wrap(clusterID, client)); err != nil {
		metrics.IncNATSExecutorError(clusterID)
		return err
	}
	return nil
}

func (g *Gateway) GetExecutor(ctx context.Context, clusterID string) (port.JetStreamExecutor, error) {
	ctx, cancel := g.scopedContext(ctx)
	defer cancel()

	session, err := g.manager.Session(ctx, clusterID)
	if err != nil {
		metrics.IncNATSExecutorError(clusterID)
		return nil, mapGatewayErr(err)
	}

	client, err := session.Client()
	if err != nil {
		_ = session.Close()
		metrics.IncNATSExecutorError(clusterID)
		return nil, mapGatewayErr(err)
	}

	return g.wrap(clusterID, client), nil
}

// Session returns a live cluster session handle (session fabric). Prefer
// WithExecutor / GetExecutor for JetStream work; use Session when callers need
// Healthy probes or explicit lifecycle beyond a single executor callback.
func (g *Gateway) Session(ctx context.Context, clusterID string) (*natsclient.Session, error) {
	ctx, cancel := g.scopedContext(ctx)
	defer cancel()

	session, err := g.manager.Session(ctx, clusterID)
	if err != nil {
		return nil, mapGatewayErr(err)
	}
	return session, nil
}

func (g *Gateway) scopedContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	timeout := g.manager.RequestTimeout()
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func (g *Gateway) wrap(clusterID string, client *natsclient.Client) port.JetStreamExecutor {
	return &cachingExecutor{
		Executor{client: client},
		clusterID,
		g.manager.ViewCache(),
		func() {
			g.manager.InvalidateViews(clusterID)
		},
		"",
	}
}

func mapGatewayErr(err error) error {
	if errors.Is(err, repo.ErrNotFound) {
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

func (e *Executor) GetMessageRange(ctx context.Context, stream string, startSeq, endSeq uint64, limit int) (*domain.MessageRangeResult, error) {
	return e.client.GetMessageRange(ctx, stream, startSeq, endSeq, limit)
}

func (e *Executor) GetMessageRangeByTime(ctx context.Context, stream string, start, end time.Time, limit int) (*domain.MessageRangeResult, error) {
	return e.client.GetMessageRangeByTime(ctx, stream, start, end, limit)
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

func (e *Executor) DeleteMessage(ctx context.Context, stream string, seq uint64) error {
	return e.client.DeleteMessage(ctx, stream, seq)
}

func (e *cachingExecutor) DeleteMessage(ctx context.Context, stream string, seq uint64) error {
	err := e.client.DeleteMessage(ctx, stream, seq)
	if err == nil {
		e.invalidate()
	}
	return err
}

func (e *Executor) ListDLQMessages(ctx context.Context, stream string, startSeq uint64, limit int) (*domain.DLQListResult, error) {
	return e.client.ListDLQMessages(ctx, stream, startSeq, limit)
}

func (e *Executor) RetryDLQMessages(ctx context.Context, stream string, req domain.DLQRetryRequest) (*domain.DLQRetryResult, error) {
	return e.client.RetryDLQMessages(ctx, stream, req)
}

func (e *cachingExecutor) RetryDLQMessages(ctx context.Context, stream string, req domain.DLQRetryRequest) (*domain.DLQRetryResult, error) {
	res, err := e.client.RetryDLQMessages(ctx, stream, req)
	if err == nil && res != nil && res.Retried > 0 {
		e.invalidate()
	}
	return res, err
}

func (e *Executor) CaptureIncidentCapsule(ctx context.Context, stream string, req domain.IncidentCapsuleCaptureRequest) (*domain.IncidentCapsuleDetail, error) {
	return e.client.CaptureIncidentCapsule(ctx, stream, req)
}

func (e *cachingExecutor) CaptureIncidentCapsule(ctx context.Context, stream string, req domain.IncidentCapsuleCaptureRequest) (*domain.IncidentCapsuleDetail, error) {
	detail, err := e.client.CaptureIncidentCapsule(ctx, stream, req)
	if err == nil {
		e.invalidate()
	}
	return detail, err
}

func (e *Executor) CaptureIncidentCapsuleFromDLQ(ctx context.Context, dlqStream string, seq uint64) (*domain.IncidentCapsuleDetail, error) {
	return e.client.CaptureIncidentCapsuleFromDLQ(ctx, dlqStream, seq)
}

func (e *cachingExecutor) CaptureIncidentCapsuleFromDLQ(ctx context.Context, dlqStream string, seq uint64) (*domain.IncidentCapsuleDetail, error) {
	detail, err := e.client.CaptureIncidentCapsuleFromDLQ(ctx, dlqStream, seq)
	if err == nil {
		e.invalidate()
	}
	return detail, err
}

func (e *Executor) ListIncidentCapsules(ctx context.Context, stream, consumer string) ([]domain.IncidentCapsuleSummary, error) {
	return e.client.ListIncidentCapsules(ctx, stream, consumer)
}

func (e *Executor) LoadIncidentCapsule(ctx context.Context, id, bucket string) (*domain.IncidentCapsuleDetail, error) {
	return e.client.LoadIncidentCapsule(ctx, id, bucket)
}

func (e *Executor) PreviewIncidentCapsule(ctx context.Context, id, bucket string) (*domain.IncidentCapsuleDryRun, error) {
	return e.client.PreviewIncidentCapsule(ctx, id, bucket)
}

func (e *Executor) Monitoring(ctx context.Context, path string) ([]byte, error) {
	return e.client.Monitoring(ctx, path)
}

func (e *Executor) ProbeRequest(ctx context.Context, subject string, format domain.RequestReplyPayloadFormat, payload []byte, timeout time.Duration) (*nats.Msg, time.Duration, error) {
	return e.client.ProbeRequest(ctx, subject, format, payload, timeout)
}

func (e *cachingExecutor) Monitoring(ctx context.Context, path string) ([]byte, error) {
	key := natsclient.ViewCacheKey(e.clusterID, "monitoring", path)
	v, etag, err := e.views.GetOrLoad(key, func() (any, error) {
		raw, err := e.client.Monitoring(ctx, path)
		if err != nil {
			metrics.IncNATSMonitoringProxyError(e.clusterID)
			return nil, err
		}
		return raw, nil
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

func (e *cachingExecutor) ProbeRequest(ctx context.Context, subject string, format domain.RequestReplyPayloadFormat, payload []byte, timeout time.Duration) (*nats.Msg, time.Duration, error) {
	return e.client.ProbeRequest(ctx, subject, format, payload, timeout)
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
