package port

import (
	"context"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/nats-io/nats.go"
)

type JetStreamExecutor interface {
	JetStream() nats.JetStreamContext
	AccountInfo(ctx context.Context) (*nats.AccountInfo, error)
	StreamNames(ctx context.Context) ([]string, error)
	ListStreams(ctx context.Context, offset, limit int) ([]*nats.StreamInfo, int, error)
	StreamInfo(ctx context.Context, name string) (*nats.StreamInfo, error)
	AddStream(ctx context.Context, cfg *nats.StreamConfig) (*nats.StreamInfo, error)
	UpdateStream(ctx context.Context, cfg *nats.StreamConfig) (*nats.StreamInfo, error)
	DeleteStream(ctx context.Context, name string) error
	PurgeStream(ctx context.Context, name string) error
	ConsumerNames(ctx context.Context, stream string) ([]string, error)
	ListConsumers(ctx context.Context, stream string, offset, limit int) ([]*nats.ConsumerInfo, int, error)
	ConsumerInfo(ctx context.Context, stream, consumer string) (*nats.ConsumerInfo, error)
	AddConsumer(ctx context.Context, stream string, cfg *nats.ConsumerConfig) (*nats.ConsumerInfo, error)
	DeleteConsumer(ctx context.Context, stream, consumer string) error
	GetMessage(ctx context.Context, stream string, seq uint64) (*nats.RawStreamMsg, error)
	GetMessageNav(ctx context.Context, stream string, seq uint64, direction string) (*domain.MessageResult, error)
	Monitoring(ctx context.Context, path string) ([]byte, error)
	ListKVBuckets(ctx context.Context) ([]domain.KVBucketInfo, error)
	CreateKVBucket(ctx context.Context, cfg *nats.KeyValueConfig) (*domain.KVBucketInfo, error)
	GetKVBucket(ctx context.Context, bucket string) (*domain.KVBucketInfo, error)
	DeleteKVBucket(ctx context.Context, bucket string) error
	ListKVKeys(ctx context.Context, bucket string, offset, limit int) ([]string, int, error)
	GetKVEntry(ctx context.Context, bucket, key string) (*domain.KVEntry, error)
	PutKVEntry(ctx context.Context, bucket, key string, value []byte) (*domain.KVEntry, error)
	DeleteKVEntry(ctx context.Context, bucket, key string) error
	KVHistory(ctx context.Context, bucket, key string) ([]domain.KVEntry, error)
	ListObjectBuckets(ctx context.Context) ([]domain.ObjectBucketInfo, error)
	CreateObjectBucket(ctx context.Context, cfg *nats.ObjectStoreConfig) (*domain.ObjectBucketInfo, error)
	GetObjectBucket(ctx context.Context, bucket string) (*domain.ObjectBucketInfo, error)
	DeleteObjectBucket(ctx context.Context, bucket string) error
	ListObjects(ctx context.Context, bucket string, offset, limit int) ([]string, int, error)
	GetObject(ctx context.Context, bucket, name string) (*domain.ObjectInfo, error)
	PutObject(ctx context.Context, bucket, name string, data []byte) (*domain.ObjectInfo, error)
	DeleteObject(ctx context.Context, bucket, name string) error
	Conn() *nats.Conn
}

type ClusterGateway interface {
	BootstrapDefault(ctx context.Context) error
	Test(ctx context.Context, clusterID string) (domain.ClusterTestResult, error)
	ConnectionStatus(ctx context.Context, clusterID string) (domain.NATSConnectionStatus, error)
	ListConnectionStatuses(ctx context.Context) []domain.NATSConnectionStatus
	Evict(clusterID string)
	Close()
	WithExecutor(ctx context.Context, clusterID string, fn func(JetStreamExecutor) error) error
	GetExecutor(ctx context.Context, clusterID string) (JetStreamExecutor, error)
}
