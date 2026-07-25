package natsclient

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	libnats "github.com/gopherust-io/nats"
	"github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
)

// Client wraps gopherust-io/nats with console-specific helpers (monitoring URL, domain mapping).
type Client struct {
	inner      libnats.Client
	monitoring string
}

type ConnectionHooks struct {
	OnDisconnect func(*nats.Conn, error)
	OnReconnect  func(*nats.Conn)
	OnClosed     func(*nats.Conn)
}

func Connect(ctx context.Context, cfg config.Config, hooks ConnectionHooks) (*Client, error) {
	return dial(ctx, cfg.NATSURL, cfg.NATSCredsFile, cfg.NATSToken, cfg.MonitoringURL, cfg.RequestTimeout, hooks)
}

func ConnectCluster(ctx context.Context, cluster store.Cluster, timeout time.Duration, hooks ConnectionHooks) (*Client, error) {
	return dial(ctx, cluster.NATSURL, cluster.CredsFilePath, cluster.Token, cluster.MonitoringURL, timeout, hooks)
}

func dial(
	ctx context.Context,
	address, credsFile, token, monitoringURL string,
	timeout time.Duration,
	hooks ConnectionHooks,
) (*Client, error) {
	cfg := libnats.DefaultConfig()
	cfg.Conn.Address = address
	cfg.Conn.ClientName = "nats-consol"
	cfg.Conn.CredentialsFile = credsFile
	cfg.Conn.Secret = token
	if timeout > 0 {
		cfg.Conn.ConnectTimeout = timeout
	}
	cfg.Conn.MaxReconnect = -1
	cfg.Conn.ReconnectWait = 2 * time.Second
	cfg.Conn.AllowReconnect = true
	cfg.Conn.InitialRetryAttempts = 0 // single attempt; Manager retries via cache/dial
	cfg.Conn.OnDisconnect = hooks.OnDisconnect
	cfg.Conn.OnReconnect = hooks.OnReconnect
	cfg.Conn.OnClosed = hooks.OnClosed
	cfg.Metrics.AllowMetrics = true
	cfg.Metrics.AllowTracing = true

	inner, err := libnats.NewClient(ctx, &cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	return &Client{
		inner:      inner,
		monitoring: monitoringURL,
	}, nil
}

func (c *Client) Close() {
	if c == nil || c.inner == nil {
		return
	}
	_ = c.inner.Connector().Shutdown()
}

func (c *Client) IsAlive() bool {
	if c == nil || c.inner == nil {
		return false
	}
	conn := c.inner.Connector().Conn()
	return conn != nil && conn.IsConnected() && !conn.IsClosed()
}

func (c *Client) ServerName() string {
	if c == nil || c.inner == nil {
		return ""
	}
	conn := c.inner.Connector().Conn()
	if conn == nil || !conn.IsConnected() {
		return ""
	}
	return conn.ConnectedServerName()
}

func (c *Client) JetStream() nats.JetStreamContext {
	return c.inner.Connector().JetStream()
}

func (c *Client) Conn() *nats.Conn {
	return c.inner.Connector().Conn()
}

func (c *Client) Lib() libnats.Client {
	return c.inner
}

func (c *Client) AccountInfo(ctx context.Context) (*nats.AccountInfo, error) {
	return c.inner.Connector().AccountInfo(ctx)
}

func (c *Client) StreamNames(ctx context.Context) ([]string, error) {
	return libnats.StreamNames(ctx, c.inner.Streams())
}

func (c *Client) ListStreams(ctx context.Context, offset, limit int) ([]*nats.StreamInfo, int, error) {
	return c.inner.Streams().ListStreamsPage(ctx, offset, limit)
}

func (c *Client) StreamInfo(ctx context.Context, name string) (*nats.StreamInfo, error) {
	return c.inner.Streams().StreamInfo(ctx, name)
}

func (c *Client) AddStream(ctx context.Context, cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	return c.inner.Streams().AddStream(ctx, cfg)
}

func (c *Client) UpdateStream(ctx context.Context, cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	return c.inner.Streams().UpdateStream(ctx, cfg)
}

func (c *Client) DeleteStream(ctx context.Context, name string) error {
	return c.inner.Streams().DeleteStream(ctx, name)
}

func (c *Client) PurgeStream(ctx context.Context, name string) error {
	return c.inner.Streams().PurgeStream(ctx, name)
}

func (c *Client) ConsumerNames(ctx context.Context, stream string) ([]string, error) {
	return c.inner.Consumers().ConsumerNames(ctx, stream)
}

func (c *Client) ListConsumers(ctx context.Context, stream string, offset, limit int) ([]*nats.ConsumerInfo, int, error) {
	return c.inner.Consumers().ListConsumersPage(ctx, stream, offset, limit)
}

func (c *Client) ConsumerInfo(ctx context.Context, stream, consumer string) (*nats.ConsumerInfo, error) {
	return c.inner.Consumers().ConsumerInfo(ctx, stream, consumer)
}

func (c *Client) AddConsumer(ctx context.Context, stream string, cfg *nats.ConsumerConfig) (*nats.ConsumerInfo, error) {
	return c.inner.Consumers().AddConsumer(ctx, stream, cfg)
}

func (c *Client) DeleteConsumer(ctx context.Context, stream, consumer string) error {
	return c.inner.Consumers().DeleteConsumer(ctx, stream, consumer)
}

func (c *Client) ReplayConsumer(ctx context.Context, stream, consumer string, req domain.ReplayConsumerRequest) (domain.ReplayConsumerResult, error) {
	if err := req.Validate(); err != nil {
		return domain.ReplayConsumerResult{}, err
	}

	opts, err := replayOptsFromRequest(req)
	if err != nil {
		return domain.ReplayConsumerResult{}, err
	}

	mode := req.NormalizedMode()
	switch mode {
	case domain.ReplayModeSidecar:
		durable, err := c.inner.Replay().CreateReplayConsumer(ctx, stream, consumer, opts...)
		if err != nil {
			return domain.ReplayConsumerResult{}, err
		}
		return domain.ReplayConsumerResult{Durable: durable, Mode: mode}, nil
	default:
		if err := c.inner.Replay().ResetConsumer(ctx, stream, consumer, opts...); err != nil {
			return domain.ReplayConsumerResult{}, err
		}
		return domain.ReplayConsumerResult{Durable: consumer, Mode: mode}, nil
	}
}

func replayOptsFromRequest(req domain.ReplayConsumerRequest) ([]libnats.ReplayOpt, error) {
	opts := make([]libnats.ReplayOpt, 0, 4)
	switch req.NormalizedFrom() {
	case domain.ReplayFromSeq:
		opts = append(opts, libnats.FromSeq(req.Seq))
	case domain.ReplayFromTime:
		t, err := req.ParseTime()
		if err != nil {
			return nil, fmt.Errorf("time: %w", err)
		}
		opts = append(opts, libnats.FromTime(t))
	case domain.ReplayFromBeginning:
		opts = append(opts, libnats.FromBeginning())
	case domain.ReplayFromNew:
		opts = append(opts, libnats.FromNew())
	}

	if strings.EqualFold(strings.TrimSpace(req.ReplayPolicy), domain.ReplayPolicyOriginal) {
		opts = append(opts, libnats.WithReplayPolicy(libnats.ReplayOriginal))
	}

	if filter := strings.TrimSpace(req.FilterSubject); filter != "" {
		opts = append(opts, libnats.WithFilterSubject(filter))
	}

	if durable := strings.TrimSpace(req.Durable); durable != "" && req.NormalizedMode() == domain.ReplayModeSidecar {
		opts = append(opts, libnats.WithReplayDurable(durable))
	}

	return opts, nil
}

func (c *Client) GetMessage(ctx context.Context, stream string, seq uint64) (*nats.RawStreamMsg, error) {
	return c.inner.Streams().GetMsg(ctx, stream, seq)
}

func (c *Client) GetMessageNav(ctx context.Context, stream string, seq uint64, direction string) (*domain.MessageResult, error) {
	info, err := c.inner.Streams().StreamInfo(ctx, stream)
	if err != nil {
		return nil, err
	}

	target := seq
	switch direction {
	case "next":
		target = seq + 1
	case "prev":
		if seq > 0 {
			target = seq - 1
		}
	}

	if target < info.State.FirstSeq || target > info.State.LastSeq {
		return nil, nats.ErrMsgNotFound
	}

	msg, err := c.inner.Streams().GetMsg(ctx, stream, target)
	if err != nil {
		return nil, err
	}

	result := &domain.MessageResult{
		Message: domain.StreamMessageFromRaw(msg),
	}
	if target > info.State.FirstSeq {
		prev := target - 1
		result.Navigation.PrevSeq = &prev
	}
	if target < info.State.LastSeq {
		next := target + 1
		result.Navigation.NextSeq = &next
	}
	return result, nil
}

func (c *Client) PublishStreamMessage(ctx context.Context, stream string, in domain.PublishMessageRequest) (domain.PublishMessageResult, error) {
	info, err := c.inner.Streams().StreamInfo(ctx, stream)
	if err != nil {
		return domain.PublishMessageResult{}, err
	}

	data, err := base64.StdEncoding.DecodeString(in.Data)
	if err != nil {
		return domain.PublishMessageResult{}, fmt.Errorf("decode data: %w", err)
	}

	subject, err := ResolvePublishSubject(in.Subject, info.Config.Subjects)
	if err != nil {
		return domain.PublishMessageResult{}, err
	}

	ack, err := c.inner.PublishRaw(ctx, subject, data, in.Headers)
	if err != nil {
		return domain.PublishMessageResult{}, err
	}

	return domain.PublishMessageResult{
		Stream:  stream,
		Subject: subject,
		Seq:     ack.Sequence,
	}, nil
}

func (c *Client) Monitoring(ctx context.Context, path string) ([]byte, error) {
	return c.inner.Monitoring().Fetch(ctx, c.monitoring, path)
}
