package natsclient

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/nats-io/nats.go"
)

type Client struct {
	js             nats.JetStreamContext
	nc             *nats.Conn
	httpClient     *http.Client
	monitoring     string
	requestTimeout time.Duration
}

type ConnectionHooks struct {
	OnDisconnect func(*nats.Conn, error)
	OnReconnect  func(*nats.Conn)
	OnClosed     func(*nats.Conn)
}

func Connect(cfg config.Config, hooks ConnectionHooks) (*Client, error) {
	opts := []nats.Option{
		nats.Name("nats-consol"),
		nats.Timeout(cfg.RequestTimeout),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
	}

	if hooks.OnDisconnect != nil {
		opts = append(opts, nats.DisconnectErrHandler(hooks.OnDisconnect))
	}
	if hooks.OnReconnect != nil {
		opts = append(opts, nats.ReconnectHandler(hooks.OnReconnect))
	}
	if hooks.OnClosed != nil {
		opts = append(opts, nats.ClosedHandler(hooks.OnClosed))
	}

	if cfg.NATSCredsFile != "" {
		opts = append(opts, nats.UserCredentials(cfg.NATSCredsFile))
	}
	if cfg.NATSToken != "" {
		opts = append(opts, nats.Token(cfg.NATSToken))
	}

	nc, err := nats.Connect(cfg.NATSURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream context: %w", err)
	}

	return &Client{
		nc:             nc,
		js:             js,
		monitoring:     cfg.MonitoringURL,
		httpClient:     monitoringHTTPClient(cfg.RequestTimeout),
		requestTimeout: cfg.RequestTimeout,
	}, nil
}

func (c *Client) Close() {
	if c.nc != nil {
		c.nc.Close()
	}
}

func (c *Client) IsAlive() bool {
	return c.nc != nil && c.nc.IsConnected() && !c.nc.IsClosed()
}

func (c *Client) ServerName() string {
	if c.nc == nil || !c.nc.IsConnected() {
		return ""
	}
	return c.nc.ConnectedServerName()
}

func (c *Client) JetStream() nats.JetStreamContext {
	return c.js
}

func (c *Client) Conn() *nats.Conn {
	return c.nc
}

func (c *Client) AccountInfo(ctx context.Context) (*nats.AccountInfo, error) {
	info, err := c.js.AccountInfo()
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (c *Client) StreamNames(ctx context.Context) ([]string, error) {
	ch := c.js.StreamNames()
	names := make([]string, 0, 64)
	for name := range ch {
		names = append(names, name)
	}
	return names, nil
}

func sliceStrings(items []string, offset, limit int) ([]string, int) {
	total := len(items)
	if offset >= total {
		return []string{}, total
	}
	end := min(offset+limit, total)
	return items[offset:end], total
}

func (c *Client) ListStreams(ctx context.Context, offset, limit int) ([]*nats.StreamInfo, int, error) {
	names, err := c.StreamNames(ctx)
	if err != nil {
		return nil, 0, err
	}
	sort.Strings(names)
	total := len(names)
	if offset >= total {
		return []*nats.StreamInfo{}, total, nil
	}
	end := min(offset+limit, total)
	pageNames := names[offset:end]
	streams := make([]*nats.StreamInfo, 0, len(pageNames))
	for _, name := range pageNames {
		info, err := c.js.StreamInfo(name)
		if err != nil {
			return nil, total, err
		}
		streams = append(streams, info)
	}
	return streams, total, nil
}

func (c *Client) StreamInfo(ctx context.Context, name string) (*nats.StreamInfo, error) {
	return c.js.StreamInfo(name)
}

func (c *Client) AddStream(ctx context.Context, cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	return c.js.AddStream(cfg)
}

func (c *Client) UpdateStream(ctx context.Context, cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	return c.js.UpdateStream(cfg)
}

func (c *Client) DeleteStream(ctx context.Context, name string) error {
	return c.js.DeleteStream(name)
}

func (c *Client) PurgeStream(ctx context.Context, name string) error {
	return c.js.PurgeStream(name)
}

func (c *Client) ConsumerNames(ctx context.Context, stream string) ([]string, error) {
	ch := c.js.ConsumerNames(stream)
	names := make([]string, 0, 64)
	for name := range ch {
		names = append(names, name)
	}
	return names, nil
}

func (c *Client) ListConsumers(ctx context.Context, stream string, offset, limit int) ([]*nats.ConsumerInfo, int, error) {
	names, err := c.ConsumerNames(ctx, stream)
	if err != nil {
		return nil, 0, err
	}
	sort.Strings(names)
	total := len(names)
	if offset >= total {
		return []*nats.ConsumerInfo{}, total, nil
	}
	end := min(offset+limit, total)
	pageNames := names[offset:end]
	consumers := make([]*nats.ConsumerInfo, 0, len(pageNames))
	for _, name := range pageNames {
		info, err := c.js.ConsumerInfo(stream, name)
		if err != nil {
			return nil, total, err
		}
		consumers = append(consumers, info)
	}
	return consumers, total, nil
}

func (c *Client) ConsumerInfo(ctx context.Context, stream, consumer string) (*nats.ConsumerInfo, error) {
	return c.js.ConsumerInfo(stream, consumer)
}

func (c *Client) AddConsumer(ctx context.Context, stream string, cfg *nats.ConsumerConfig) (*nats.ConsumerInfo, error) {
	return c.js.AddConsumer(stream, cfg)
}

func (c *Client) DeleteConsumer(ctx context.Context, stream, consumer string) error {
	return c.js.DeleteConsumer(stream, consumer)
}

func (c *Client) GetMessage(ctx context.Context, stream string, seq uint64) (*nats.RawStreamMsg, error) {
	return c.js.GetMsg(stream, seq)
}

func (c *Client) GetMessageNav(ctx context.Context, stream string, seq uint64, direction string) (*domain.MessageResult, error) {
	info, err := c.js.StreamInfo(stream)
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

	msg, err := c.js.GetMsg(stream, target)
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
	info, err := c.js.StreamInfo(stream)
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

	msg := &nats.Msg{Subject: subject, Data: data}
	for k, v := range in.Headers {
		if msg.Header == nil {
			msg.Header = nats.Header{}
		}
		msg.Header.Set(k, v)
	}

	ack, err := c.js.PublishMsg(msg)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.monitoring+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("monitoring request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("monitoring %s: status %d: %s", path, resp.StatusCode, string(body))
	}

	body, err := readBodyPooled(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
