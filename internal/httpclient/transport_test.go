package httpclient

import (
	"net/http"
	"testing"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNewTransportOutboundDisabled(t *testing.T) {
	t.Parallel()
	rt := NewTransport(config.Config{HTTP3OutboundEnabled: false})
	_, ok := rt.(*http.Transport)
	assert.True(t, ok, "expected default transport when outbound h3 disabled")
}

func TestNewTransportFallback(t *testing.T) {
	t.Parallel()
	rt := NewTransport(config.Config{
		HTTP3OutboundEnabled:  true,
		HTTP3OutboundFallback: true,
	})
	_, ok := rt.(*fallbackTransport)
	assert.True(t, ok)
}
