package natsclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/nats-io/nats.go"
)

type Client struct {
	nc           *nats.Conn
	js           nats.JetStreamContext
	monitoring   string
	httpClient   *http.Client
	requestTimeout time.Duration
}

func Connect(cfg config.Config) (*Client, error) {
	opts := []nats.Option{
		nats.Name("nats-consol"),
		nats.Timeout(cfg.RequestTimeout),
		nats.MaxReconnects(-1),
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
		httpClient:     &http.Client{Timeout: cfg.RequestTimeout},
		requestTimeout: cfg.RequestTimeout,
	}, nil
}

func (c *Client) Close() {
	if c.nc != nil {
		c.nc.Close()
	}
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
	var names []string
	for name := range ch {
		names = append(names, name)
	}
	return names, nil
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
	var names []string
	for name := range ch {
		names = append(names, name)
	}
	return names, nil
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

type MessageResult struct {
	Message    *nats.RawStreamMsg `json:"message"`
	Navigation MessageNavigation  `json:"navigation"`
}

type MessageNavigation struct {
	PrevSeq *uint64 `json:"prev_seq,omitempty"`
	NextSeq *uint64 `json:"next_seq,omitempty"`
}

func (c *Client) GetMessageNav(ctx context.Context, stream string, seq uint64, direction string) (*MessageResult, error) {
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

	result := &MessageResult{Message: msg}
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

func (c *Client) Monitoring(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.monitoring+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("monitoring request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("monitoring %s: status %d: %s", path, resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
