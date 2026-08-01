package middleware

import (
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func (mw *MwHandler) VerifyCors(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	origins := mw.cfg.CORSOrigins()
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}

	return func(ctx *fasthttp.RequestCtx) {
		origin := strings.BytesToString(ctx.Request.Header.Peek(HeaderOrigin))
		if !strings.IsEmpty(origin) {
			if _, ok := allowed[origin]; ok {
				ctx.Response.Header.Set(HeaderAccessControlAllowOrigin, origin)
				ctx.Response.Header.Set(HeaderAccessControlAllowCredentials, "true")
				ctx.Response.Header.Set(HeaderVary, HeaderOrigin)
			}
		}
		ctx.Response.Header.Set(HeaderAccessControlAllowMethods, "GET, POST, PUT, DELETE, OPTIONS")
		ctx.Response.Header.Set(HeaderAccessControlAllowMethods, "Accept, Authorization, Content-Type, X-Request-ID, X-CSRF-Token")

		if ctx.IsOptions() {
			if !strings.IsEmpty(origin) {
				if _, ok := allowed[origin]; ok {
					ctx.SetStatusCode(fasthttp.StatusNoContent)
					return
				}
			}
			httpstatus.WriteForbidden(ctx)
			return
		}
		next(ctx)
	}
}
