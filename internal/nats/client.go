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
	"github.com/gopherust-io/nats-consol/internal/repo"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type Client struct {
	natsCl     libnats.Client
	seqCache   *seqExistsCache
	monitoring string
}

type ConnectionHooks struct {
	OnDisconnect func(*nats.Conn, error)
	OnReconnect  func(*nats.Conn)
	OnClosed     func(*nats.Conn)
}

func Connect(ctx context.Context, cfg config.Config, hooks ConnectionHooks) (*Client, error) {
	if err := ValidateEnvConfig(cfg); err != nil {
		return nil, err
	}
	return dial(ctx, cfg, cfg.NATS.URL, cfg.NATS.CredsFile, cfg.NATS.Token, cfg.NATS.MonitoringURL, hooks)
}

func ConnectCluster(ctx context.Context, cfg config.Config, cluster repo.Cluster, hooks ConnectionHooks) (*Client, error) {
	return dial(ctx, cfg, cluster.NATSURL, cluster.CredsFilePath, cluster.Token, cluster.MonitoringURL, hooks)
}

func dial(
	ctx context.Context,
	appCfg config.Config,
	address, credsFile,
	token, monitoringURL string,
	hooks ConnectionHooks,
) (*Client, error) {
	cfg := libnats.DefaultConfig()
	cfg.Conn.Address = address
	cfg.Conn.ClientName = appCfg.ProjectName
	cfg.Conn.CredentialsFile = credsFile
	cfg.Conn.Secret = token
	if appCfg.RequestTimeout > 0 {
		cfg.Conn.ConnectTimeout = appCfg.RequestTimeout
	}
	cfg.Conn.MaxReconnect = appCfg.NATS.MaxReconnect
	cfg.Conn.ReconnectWait = appCfg.NATS.ReconnectWait
	cfg.Conn.AllowReconnect = appCfg.NATS.AllowReconnect
	// Multi-URL cluster seeds: randomize initial peer (balance), reconnect to survivors
	cfg.Conn.DontRandomize = appCfg.NATS.DontRandomize
	// single attempt by default, manager retries via cache/dial
	cfg.Conn.InitialRetryAttempts = appCfg.NATS.InitialRetryAttempts
	cfg.Conn.OnDisconnect = hooks.OnDisconnect
	cfg.Conn.OnReconnect = hooks.OnReconnect
	cfg.Conn.OnClosed = hooks.OnClosed
	cfg.Metrics.AllowMetrics = appCfg.NATS.AllowMetrics
	cfg.Metrics.AllowTracing = appCfg.NATS.AllowTracing

	tlsCfg, err := func() (libnats.ConnectionTLS, error) {
		out := libnats.ConnectionTLS{
			ServerName:         appCfg.NATS.TlsServerName,
			InsecureSkipVerify: appCfg.NATS.TlsInsecureSkipVerify,
		}
		hasMaterial := !commonstrings.IsEmpty(appCfg.NATS.TlsCAFile) ||
			!commonstrings.IsEmpty(appCfg.NATS.TlsCertFile) ||
			!commonstrings.IsEmpty(appCfg.NATS.TlsKeyFile) ||
			!commonstrings.IsEmpty(appCfg.NATS.TlsServerName) ||
			appCfg.NATS.TlsInsecureSkipVerify

		if !hasMaterial {
			return out, nil
		}
		if (commonstrings.IsEmpty(appCfg.NATS.TlsCertFile)) != (commonstrings.IsEmpty(appCfg.NATS.TlsKeyFile)) {
			return out, errors.New("NATS_TLS_CERT_FILE and NATS_TLS_KEY_FILE must be set together")
		}
		if !commonstrings.IsEmpty(appCfg.NATS.TlsCAFile) {
			ca, err := os.ReadFile(appCfg.NATS.TlsCAFile)
			if err != nil {
				return out, fmt.Errorf("read NATS_TLS_CA_FILE: %w", err)
			}
			out.CA = ca
		}
		if !commonstrings.IsEmpty(appCfg.NATS.TlsCertFile) {
			cert, err := os.ReadFile(appCfg.NATS.TlsCertFile)
			if err != nil {
				return out, fmt.Errorf("read NATS_TLS_CERT_FILE: %w", err)
			}
			key, err := os.ReadFile(appCfg.NATS.TlsKeyFile)
			if err != nil {
				return out, fmt.Errorf("read NATS_TLS_KEY_FILE: %w", err)
			}
			out.Cert = cert
			out.Key = key
		}
		return out, nil
	}()
	if err != nil {
		return nil, err
	}

	cfg.Conn.TLS = tlsCfg
	natsCl, err := libnats.NewClient(ctx, &cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	return &Client{
		natsCl:     natsCl,
		monitoring: monitoringURL,
	}, nil
}

func (c *Client) Close() error {
	if c == nil || c.natsCl == nil {
		return nil
	}
	return c.natsCl.Connector().Shutdown()
}

func (c *Client) IsAlive() bool {
	if c == nil || c.natsCl == nil {
		return false
	}
	conn := c.natsCl.Connector().Conn()
	return conn != nil && conn.IsConnected() && !conn.IsClosed()
}

func (c *Client) ServerName() string {
	if c == nil || c.natsCl == nil {
		return ""
	}
	conn := c.natsCl.Connector().Conn()
	if conn == nil || !conn.IsConnected() {
		return ""
	}
	return conn.ConnectedServerName()
}

func (c *Client) JetStream() nats.JetStreamContext {
	return c.natsCl.Connector().JetStream()
}

func (c *Client) Conn() *nats.Conn {
	return c.natsCl.Connector().Conn()
}

func (c *Client) Lib() libnats.Client {
	return c.natsCl
}

func (c *Client) AccountInfo(ctx context.Context) (*nats.AccountInfo, error) {
	return c.natsCl.Connector().AccountInfo(ctx)
}

func (c *Client) StreamNames(ctx context.Context) ([]string, error) {
	return libnats.StreamNames(ctx, c.natsCl.Streams())
}

func (c *Client) ListStreams(ctx context.Context, offset, limit int) ([]*nats.StreamInfo, int, error) {
	return c.natsCl.Streams().ListStreamsPage(ctx, offset, limit)
}

func (c *Client) StreamInfo(ctx context.Context, name string) (*nats.StreamInfo, error) {
	return c.natsCl.Streams().StreamInfo(ctx, name)
}

func (c *Client) AddStream(ctx context.Context, cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	return c.natsCl.Streams().AddStream(ctx, cfg)
}

func (c *Client) UpdateStream(ctx context.Context, cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	return c.natsCl.Streams().UpdateStream(ctx, cfg)
}

func (c *Client) DeleteStream(ctx context.Context, name string) error {
	return c.natsCl.Streams().DeleteStream(ctx, name)
}

func (c *Client) PurgeStream(ctx context.Context, name string) error {
	return c.natsCl.Streams().PurgeStream(ctx, name)
}

func (c *Client) ConsumerNames(ctx context.Context, stream string) ([]string, error) {
	return c.natsCl.Consumers().ConsumerNames(ctx, stream)
}

func (c *Client) ListConsumers(ctx context.Context, stream string, offset, limit int) ([]*nats.ConsumerInfo, int, error) {
	return c.natsCl.Consumers().ListConsumersPage(ctx, stream, offset, limit)
}

func (c *Client) ConsumerInfo(ctx context.Context, stream, consumer string) (*nats.ConsumerInfo, error) {
	return c.natsCl.Consumers().ConsumerInfo(ctx, stream, consumer)
}

func (c *Client) AddConsumer(ctx context.Context, stream string, cfg *nats.ConsumerConfig) (*nats.ConsumerInfo, error) {
	return c.natsCl.Consumers().AddConsumer(ctx, stream, cfg)
}

func (c *Client) UpdateConsumer(ctx context.Context, stream string, cfg *nats.ConsumerConfig) (*nats.ConsumerInfo, error) {
	return c.natsCl.Consumers().UpdateConsumer(ctx, stream, cfg)
}

func (c *Client) DeleteConsumer(ctx context.Context, stream, consumer string) error {
	return c.natsCl.Consumers().DeleteConsumer(ctx, stream, consumer)
}

func (c *Client) ReplayConsumer(ctx context.Context, stream, consumer string, req domain.ReplayConsumerRequest) (domain.ReplayConsumerResult, error) {
	if err := req.Validate(); err != nil {
		return domain.ReplayConsumerResult{}, err
	}

	opts, err := func() ([]libnats.ReplayOpt, error) {
		opts := make([]libnats.ReplayOpt, 0, 8)

		oneMsg := req.NormalizedFrom() == domain.ReplayFromSeq &&
			req.Seq > 0 &&
			req.UntilSeq == req.Seq &&
			req.Limit == 1

		if oneMsg {
			opts = append(opts, libnats.OneMessage(req.Seq))
		} else {
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

			if req.UntilSeq > 0 {
				opts = append(opts, libnats.UntilSeq(req.UntilSeq))
			}
			if !commonstrings.IsEmpty(strings.TrimSpace(req.UntilTime)) {
				t, err := req.ParseUntilTime()
				if err != nil {
					return nil, fmt.Errorf("untilTime: %w", err)
				}
				opts = append(opts, libnats.UntilTime(t))
			}
			if req.Limit > 0 {
				opts = append(opts, libnats.Limit(req.Limit))
			}
		}

		if strings.EqualFold(strings.TrimSpace(req.ReplayPolicy), domain.ReplayPolicyOriginal) {
			opts = append(opts, libnats.WithReplayPolicy(libnats.ReplayOriginal))
		}
		if filter := strings.TrimSpace(req.FilterSubject); !commonstrings.IsEmpty(filter) {
			opts = append(opts, libnats.WithFilterSubject(filter))
		}
		if durable := strings.TrimSpace(req.Durable); !commonstrings.IsEmpty(durable) && req.NormalizedMode() == domain.ReplayModeSidecar {
			opts = append(opts, libnats.WithReplayDurable(durable))
		}

		return opts, nil
	}()
	if err != nil {
		return domain.ReplayConsumerResult{}, err
	}

	mode := req.NormalizedMode()
	res := libnats.ReplayConsumerResult{}

	switch mode {
	case domain.ReplayModeSidecar:
		res, err = c.natsCl.Replay().CreateReplayConsumer(ctx, stream, consumer, opts...)
	default:
		res, err = c.natsCl.Replay().ResetConsumer(ctx, stream, consumer, opts...)
	}
	if err != nil {
		return domain.ReplayConsumerResult{}, err
	}

	out := domain.ReplayConsumerResult{
		Durable:  res.Durable,
		Mode:     mode,
		StartSeq: res.StartSeq,
		UntilSeq: res.UntilSeq,
		Limit:    res.Limit,
	}
	if res.StartTime != nil {
		s := res.StartTime.UTC().Format(time.RFC3339Nano)
		out.StartTime = &s
	}
	if res.UntilTime != nil {
		s := res.UntilTime.UTC().Format(time.RFC3339Nano)
		out.UntilTime = &s
	}

	return out, nil
}

func (c *Client) GetMessageRange(ctx context.Context, stream string, startSeq, endSeq uint64, limit int) (*domain.MessageRangeResult, error) {
	opts := make([]libnats.MsgRangeOpt, 0, 1)
	if limit > 0 {
		opts = append(opts, libnats.WithMaxMessages(limit))
	}
	messages, truncated, err := c.natsCl.Replay().GetMsgRange(ctx, stream, startSeq, endSeq, opts...)
	if err != nil {
		return nil, err
	}
	return storedRangeToDomain(messages, truncated), nil
}

func (c *Client) GetMessageRangeByTime(ctx context.Context, stream string, start, end time.Time, limit int) (*domain.MessageRangeResult, error) {
	opts := make([]libnats.MsgRangeOpt, 0, 1)
	if limit > 0 {
		opts = append(opts, libnats.WithMaxMessages(limit))
	}
	messages, truncated, err := c.natsCl.Replay().GetMsgRangeByTime(ctx, stream, start, end, opts...)
	if err != nil {
		return nil, err
	}
	return storedRangeToDomain(messages, truncated), nil
}

func storedRangeToDomain(msgs []*libnats.StoredMessage, truncated bool) *domain.MessageRangeResult {
	out := &domain.MessageRangeResult{
		Messages:  make([]domain.StreamMessage, 0, len(msgs)),
		Truncated: truncated,
	}
	for _, m := range msgs {
		if m == nil {
			continue
		}
		out.Messages = append(out.Messages, domain.StreamMessageFromStored(m))
	}

	return out
}

func (c *Client) GetMessage(ctx context.Context, stream string, seq uint64) (*nats.RawStreamMsg, error) {
	return c.natsCl.Streams().GetMsg(ctx, stream, seq)
}

func (c *Client) GetMessageNav(ctx context.Context, stream string, seq uint64, direction string) (*domain.MessageResult, error) {
	info, err := c.natsCl.Streams().StreamInfo(ctx, stream)
	if err != nil {
		return nil, err
	}

	const (
		next = "next"
		prev = "prev"
	)
	var msg *nats.RawStreamMsg
	switch direction {
	case next:
		msg, err = c.natsCl.Streams().GetNextMsgAfter(ctx, stream, seq)
		if err != nil {
			return nil, err
		}
	case prev:
		if seq <= info.State.FirstSeq {
			return nil, nats.ErrMsgNotFound
		}
		prevSeq, ok := c.findPrevSeq(ctx, stream, seq, info.State.FirstSeq)
		if !ok {
			return nil, nats.ErrMsgNotFound
		}
		msg, err = c.natsCl.Streams().GetMsg(ctx, stream, prevSeq)
		if err != nil {
			return nil, err
		}
	default:
		if !commonstrings.IsEmpty(direction) {
			return nil, fmt.Errorf("invalid direction %q", direction)
		}
		msg, err = c.natsCl.Streams().GetMsg(ctx, stream, seq)
		if err != nil {
			return nil, err
		}
	}

	result := &domain.MessageResult{
		Message: domain.StreamMessageFromRaw(msg),
	}

	c.ensureSeqCache().mark(stream, msg.Sequence, true)
	if prevSeq, ok := c.findPrevSeq(ctx, stream, msg.Sequence, info.State.FirstSeq); ok {
		result.Navigation.PrevSeq = &prevSeq
	}
	if nextSeq, ok := c.findNextSeq(ctx, stream, msg.Sequence, info.State.LastSeq); ok {
		result.Navigation.NextSeq = &nextSeq
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
// over the gap range (O(log gap) probes) with a per-client existence cache
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
			_, err = c.natsCl.Streams().GetMsg(ctx, stream, mid)
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
	msg, err := c.natsCl.Streams().GetNextMsgAfter(ctx, stream, seq)
	if err != nil {
		return 0, false
	}
	return msg.Sequence, true
}

func (c *Client) PublishStreamMessage(ctx context.Context, stream string, in domain.PublishMessageRequest) (domain.PublishMessageResult, error) {
	info, err := c.natsCl.Streams().StreamInfo(ctx, stream)
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

	ack, err := c.natsCl.PublishRaw(ctx, subject, data, in.Headers)
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
	return c.natsCl.Monitoring().Fetch(ctx, c.monitoring, path)
}

func (c *Client) ProbeRequest(
	ctx context.Context,
	subject string,
	format domain.RequestReplyPayloadFormat,
	payload []byte,
	timeout time.Duration,
) (*nats.Msg, time.Duration, error) {
	conn := c.Conn()
	if conn == nil || !conn.IsConnected() {
		return nil, 0, errors.New("nats connection unavailable")
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	msg := &nats.Msg{
		Subject: subject,
		Data:    payload,
		Header:  ProbeRequestHeaders(format, len(payload) > 0),
	}

	start := time.Now()
	reply, err := conn.RequestMsgWithContext(reqCtx, msg)
	if err != nil {
		return nil, 0, err
	}
	return reply, time.Since(start), nil
}
