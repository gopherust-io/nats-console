package httpclient

import (
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/config"
)

// NewClient returns a fasthttp client with read/write timeouts matching timeout.
func NewClient(_ config.Config, timeout time.Duration) *fasthttp.Client {
	c := &fasthttp.Client{}
	if timeout > 0 {
		c.ReadTimeout = timeout
		c.WriteTimeout = timeout
		c.MaxConnWaitTimeout = timeout
	}
	return c
}
