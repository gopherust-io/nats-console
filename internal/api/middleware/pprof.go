package middleware

import (
	"errors"
	"net/http"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ApplyDebugPprof short-circuits /debug/pprof before the API middleware chain.
// net/http/pprof registers on DefaultServeMux; enablement and optional auth are
// gated by cfg.Pprof separately from /api/v1/pprof/*.
func (mw *MwHandler) ApplyDebugPprof(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	pprofHandler := fasthttpadaptor.NewFastHTTPHandler(http.DefaultServeMux)
	return func(ctx *fasthttp.RequestCtx) {
		if !isPprofPath(requestPath(ctx)) {
			next(ctx)
			return
		}
		if !mw.cfg.Pprof.Enabled {
			httpstatus.WriteError(ctx, fasthttp.StatusNotFound, errors.New("not found"))
			return
		}
		if mw.cfg.Pprof.AuthEnabled && !mw.authorizeDebugPprof(ctx) {
			return
		}
		pprofHandler(ctx)
	}
}

func (mw *MwHandler) authorizeDebugPprof(ctx *fasthttp.RequestCtx) bool {
	user, ok := mw.Authenticate(ctx)
	if !ok || commonstrings.IsEmpty(user.ID) {
		httpstatus.WriteUnauthorized(ctx)
		return false
	}
	loaded, err := mw.authService.LoadUserForSession(httpctx.FromRequest(ctx), user)
	if err != nil {
		httpstatus.WriteUnauthorized(ctx)
		return false
	}
	if !auth.CanViewProfiling(loaded) {
		httpstatus.WriteForbidden(ctx)
		return false
	}
	return true
}
