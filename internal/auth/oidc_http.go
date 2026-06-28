package auth

import (
	"net/http"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/httpclient"
)

func hostRewriteHTTPClient(publicHost, internalHost string, cfg config.Config) *http.Client {
	return &http.Client{
		Transport: &hostRewriteTransport{
			base:         httpclient.NewTransport(cfg),
			publicHost:   publicHost,
			internalHost: internalHost,
		},
	}
}

type hostRewriteTransport struct {
	base         http.RoundTripper
	publicHost   string
	internalHost string
}

func (t *hostRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	if req.URL.Host != t.publicHost {
		return base.RoundTrip(req)
	}
	cloned := req.Clone(req.Context())
	u := *cloned.URL
	cloned.URL = &u
	cloned.URL.Host = t.internalHost
	cloned.Host = t.internalHost
	return base.RoundTrip(cloned)
}
