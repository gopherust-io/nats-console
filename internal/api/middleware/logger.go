package middleware

import (
	"time"

	"github.com/gopherust-io/tel"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func (mw *MwHandler) ApplyRequestLogger(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		path := requestPath(ctx)
		if !isAPIPath(path) {
			next(ctx)
			return
		}
		start := time.Now()
		next(ctx)
		tel.Info().
			Str("component", "http").
			Str("method", strings.BytesToString(ctx.Method())).
			Str("path", path).
			Int("status", ctx.Response.StatusCode()).
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Str("request_id", requestID(ctx)).
			Msg("request")
	}
}
