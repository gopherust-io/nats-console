package natsclient

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
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
	seqCache   *seqExistsCache
	monitoring string
}

type ConnectionHooks struct {
	OnDisconnect func(*nats.Conn, error)
	OnReconnect  func(*nats.Conn)
	OnClosed     func(*nats.Conn)
}

func Connect(ctx context.Context, cfg config.Config, hooks ConnectionHooks) (*Client, error) {
	return dial(ctx, cfg, cfg.NATSURL, cfg.NATSCredsFile, cfg.NATSToken, cfg.MonitoringURL, hooks)
}

func ConnectCluster(ctx context.Context, cfg config.Config, cluster store.Cluster, hooks ConnectionHooks) (*Client, error) {
	return dial(ctx, cfg, cluster.NATSURL, cluster.CredsFilePath, cluster.Token, cluster.MonitoringURL, hooks)
}

func dial(
	ctx context.Context,
	appCfg config.Config,
	address, credsFile, token, monitoringURL string,
	hooks ConnectionHooks,
) (*Client, error) {
	cfg := libnats.DefaultConfig()
	cfg.Conn.Address = address
	cfg.Conn.ClientName = "nats-consol"
	cfg.Conn.CredentialsFile = credsFile
	cfg.Conn.Secret = token
	if appCfg.RequestTimeout > 0 {
		cfg.Conn.ConnectTimeout = appCfg.RequestTimeout
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

	tlsCfg, err := loadNATSTLS(appCfg)
	if err != nil {
		return nil, err
	}
	cfg.Conn.TLS = tlsCfg

	inner, err := libnats.NewClient(ctx, &cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	return &Client{
		inner:      inner,
		monitoring: monitoringURL,
	}, nil
}

func loadNATSTLS(cfg config.Config) (libnats.ConnectionTLS, error) {
	out := libnats.ConnectionTLS{
		ServerName:         cfg.NATSTlsServerName,
		InsecureSkipVerify: cfg.NATSTlsInsecureSkipVerify,
	}
	hasMaterial := cfg.NATSTlsCAFile != "" || cfg.NATSTlsCertFile != "" || cfg.NATSTlsKeyFile != "" ||
		cfg.NATSTlsServerName != "" || cfg.NATSTlsInsecureSkipVerify
	if !hasMaterial {
		return out, nil
	}
	if (cfg.NATSTlsCertFile == "") != (cfg.NATSTlsKeyFile == "") {
		return out, errors.New("NATS_TLS_CERT_FILE and NATS_TLS_KEY_FILE must be set together")
	}
	if cfg.NATSTlsCAFile != "" {
		ca, err := os.ReadFile(cfg.NATSTlsCAFile)
		if err != nil {
			return out, fmt.Errorf("read NATS_TLS_CA_FILE: %w", err)
		}
		out.CA = ca
	}
	if cfg.NATSTlsCertFile != "" {
		cert, err := os.ReadFile(cfg.NATSTlsCertFile)
		if err != nil {
			return out, fmt.Errorf("read NATS_TLS_CERT_FILE: %w", err)
		}
		key, err := os.ReadFile(cfg.NATSTlsKeyFile)
		if err != nil {
			return out, fmt.Errorf("read NATS_TLS_KEY_FILE: %w", err)
		}
		out.Cert = cert
		out.Key = key
	}
	return out, nil
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

func (c *Client) UpdateConsumer(ctx context.Context, stream string, cfg *nats.ConsumerConfig) (*nats.ConsumerInfo, error) {
	return c.inner.Consumers().UpdateConsumer(ctx, stream, cfg)
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

	var msg *nats.RawStreamMsg
	switch direction {
	case "next":
		msg, err = c.inner.Streams().GetNextMsgAfter(ctx, stream, seq)
		if err != nil {
			return nil, err
		}
	case "prev":
		if seq <= info.State.FirstSeq {
			return nil, nats.ErrMsgNotFound
		}
		prev, ok := c.findPrevSeq(ctx, stream, seq, info.State.FirstSeq)
		if !ok {
			return nil, nats.ErrMsgNotFound
		}
		msg, err = c.inner.Streams().GetMsg(ctx, stream, prev)
		if err != nil {
			return nil, err
		}
	default:
		if direction != "" {
			return nil, fmt.Errorf("invalid direction %q", direction)
		}
		msg, err = c.inner.Streams().GetMsg(ctx, stream, seq)
		if err != nil {
			return nil, err
		}
	}

	result := &domain.MessageResult{
		Message: domain.StreamMessageFromRaw(msg),
	}
	target := msg.Sequence
	c.ensureSeqCache().mark(stream, target, true)
	if prev, ok := c.findPrevSeq(ctx, stream, target, info.State.FirstSeq); ok {
		result.Navigation.PrevSeq = &prev
	}
	if next, ok := c.findNextSeq(ctx, stream, target, info.State.LastSeq); ok {
		result.Navigation.NextSeq = &next
	}
	return result, nil
}

func (c *Client) ensureSeqCache() *seqExistsCache {
	if c.seqCache == nil {
		c.seqCache = newSeqExistsCache()
	}
	return c.seqCache
}

// findPrevSeq locates the nearest existing sequence below seq using binary search
// over the gap range (O(log gap) probes) with a per-client existence cache.
func (c *Client) findPrevSeq(ctx context.Context, stream string, seq, firstSeq uint64) (uint64, bool) {
	if seq <= firstSeq {
		return 0, false
	}
	cache := c.ensureSeqCache()
	lo, hi := firstSeq, seq-1
	var found uint64
	have := false
	for lo <= hi {
		mid := lo + (hi-lo)/2
		exists, known := cache.known(stream, mid)
		var err error
		if !known {
			_, err = c.inner.Streams().GetMsg(ctx, stream, mid)
			if err == nil {
				exists = true
				cache.mark(stream, mid, true)
			} else if errors.Is(err, nats.ErrMsgNotFound) {
				exists = false
				cache.mark(stream, mid, false)
			} else {
				return 0, false
			}
		}
		if exists {
			found = mid
			have = true
			lo = mid + 1
		} else {
			if mid == firstSeq {
				break
			}
			hi = mid - 1
		}
	}
	return found, have
}

func (c *Client) findNextSeq(ctx context.Context, stream string, seq, lastSeq uint64) (uint64, bool) {
	if seq >= lastSeq {
		return 0, false
	}
	msg, err := c.inner.Streams().GetNextMsgAfter(ctx, stream, seq)
	if err != nil {
		return 0, false
	}
	return msg.Sequence, true
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
