package httpclient

import (
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Parallel()
	c := NewClient(config.Config{}, 0)
	require.NotNil(t, c)
	assert.Equal(t, time.Duration(0), c.ReadTimeout)
	assert.Equal(t, time.Duration(0), c.WriteTimeout)

	c = NewClient(config.Config{}, time.Second)
	require.NotNil(t, c)
	assert.Equal(t, time.Second, c.ReadTimeout)
	assert.Equal(t, time.Second, c.WriteTimeout)
	assert.Equal(t, time.Second, c.MaxConnWaitTimeout)
}
