package api

import (
	"time"

	"github.com/valyala/fasthttp"
)

// refreshSSEWriteDeadline extends the connection write deadline so long-lived
// EventSource streams are not killed by the server HTTP.WriteTimeout.
func refreshSSEWriteDeadline(ctx *fasthttp.RequestCtx) {
	if ctx == nil {
		return
	}
	c := ctx.Conn()
	if c == nil {
		return
	}
	_ = c.SetWriteDeadline(time.Now().Add(2 * time.Minute))
}
