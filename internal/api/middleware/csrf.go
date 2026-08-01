package middleware

import (
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func (mw *MwHandler) VerifyCSRF(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		if !requiresCSRF(ctx) {
			next(ctx)
			return
		}
		token := strings.BytesToString(ctx.Request.Header.Peek(HeaderCSRF))
		cookie := strings.BytesToString(ctx.Request.Header.Cookie(auth.CSRFCookie))
		if strings.IsEmpty(token) || strings.IsEmpty(cookie) || token != cookie {
			httpstatus.WriteErrorMessage(ctx, fasthttp.StatusForbidden, httpstatus.CodeCSRFInvalid, "csrf token missing or invalid")
			return
		}
		next(ctx)
	}
}

func requiresCSRF(ctx *fasthttp.RequestCtx) bool {
	method := strings.BytesToString(ctx.Method())
	if method == fasthttp.MethodGet || method == fasthttp.MethodHead || method == fasthttp.MethodOptions {
		return false
	}
	path := requestPath(ctx)
	if !isAPIPath(path) {
		return false
	}
	// Refresh is public for auth middleware but still requires CSRF when a refresh cookie is present.
	if path == pathPrefixAuthRefresh {
		return len(ctx.Request.Header.Cookie(auth.RefreshCookie)) > 0
	}
	if isPublicPath(path) {
		return false
	}
	if len(ctx.Request.Header.Cookie(auth.SessionCookie)) == 0 {
		return false
	}
	return true
}
