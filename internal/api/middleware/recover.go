package middleware

import (
	"runtime/debug"

	"github.com/gopherust-io/tel"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func (mw *MwHandler) ApplyRecover(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			tel.Error().
				Str("component", "http").
				Any("panic", rec).
				Bytes("stack", debug.Stack()).
				Str("method", commonstrings.BytesToString(ctx.Method())).
				Str("path", requestPath(ctx)).
				Str("request_id", requestID(ctx)).
				Msg("http handler panic")
			httpstatus.WriteErrorMessage(ctx, fasthttp.StatusInternalServerError, httpstatus.CodeInternal, "internal error")
		}()
		next(ctx)
	}
}
