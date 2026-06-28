package live

import (
	"context"
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestHubConfigDefaults(t *testing.T) {
	t.Parallel()

	h := NewHub(nil, config.Config{})
	assert.Equal(t, defaultLiveWSMaxMessages, h.liveWSMaxMessages())
	assert.Equal(t, defaultLiveWSIdleTimeout, h.liveWSIdleTimeout())
	assert.Equal(t, defaultLiveWSRateLimit, h.liveWSRateLimit())

	custom := NewHub(nil, config.Config{
		LiveWSMaxMessages: 42,
		LiveWSIdleTimeout: 2 * time.Minute,
		LiveWSRateLimit:   50 * time.Millisecond,
	})
	assert.Equal(t, 42, custom.liveWSMaxMessages())
	assert.Equal(t, 2*time.Minute, custom.liveWSIdleTimeout())
	assert.Equal(t, 50*time.Millisecond, custom.liveWSRateLimit())
}

func TestRequestContext(t *testing.T) {
	t.Parallel()

	type ctxKey struct{}
	bg := context.WithValue(context.Background(), ctxKey{}, "scoped")
	ctx := &fasthttp.RequestCtx{}
	ctx.SetUserValue("context", bg)
	require.Equal(t, bg, requestContext(ctx))

	empty := &fasthttp.RequestCtx{}
	require.Equal(t, context.Background(), requestContext(empty))
}
