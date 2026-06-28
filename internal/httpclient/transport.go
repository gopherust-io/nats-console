package httpclient

import (
	"net/http"
	"time"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/quic-go/quic-go/http3"
)

func NewTransport(cfg config.Config) http.RoundTripper {
	if !cfg.HTTP3OutboundEnabled {
		return http.DefaultTransport
	}
	h3 := &http3.Transport{}
	if !cfg.HTTP3OutboundFallback {
		return h3
	}
	return &fallbackTransport{
		primary:  h3,
		fallback: http.DefaultTransport,
	}
}

func NewClient(cfg config.Config, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		return &http.Client{Transport: NewTransport(cfg)}
	}
	return &http.Client{
		Transport: NewTransport(cfg),
		Timeout:   timeout,
	}
}

type fallbackTransport struct {
	primary  http.RoundTripper
	fallback http.RoundTripper
}

func (t *fallbackTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.primary.RoundTrip(req)
	if err == nil {
		return resp, nil
	}
	if t.fallback == nil {
		return nil, err
	}
	return t.fallback.RoundTrip(req)
}
