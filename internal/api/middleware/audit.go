package middleware

import (
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/audit"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/ipset"
	"github.com/gopherust-io/nats-consol/internal/repo"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func (mw *MwHandler) VerifyAudit(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		method := strings.BytesToString(ctx.Method())
		path := requestPath(ctx)
		if method == fasthttp.MethodGet || method == fasthttp.MethodHead || method == fasthttp.MethodOptions || isPublicPath(path) {
			next(ctx)
			return
		}

		next(ctx)

		if ctx.Response.StatusCode() >= fasthttp.StatusBadRequest || mw.auditWriter == nil {
			return
		}

		ctxReq := httpctx.FromRequest(ctx)
		user, _ := auth.UserFromContext(ctxReq)
		resourceType, resourceName := audit.ParseResource(path)
		trustedProxies := ipset.ParseTrustedProxies(mw.cfg.TrustedProxyList())

		details := repo.AuditRequestDetails{
			Method: method,
			Path:   path,
			Status: ctx.Response.StatusCode(),
		}

		mw.auditWriter.Log(repo.AuditCreate{
			Actor:        user.Username,
			Action:       audit.ActionForMethod(method),
			ClusterID:    audit.ClusterIDFromPath(path),
			ResourceType: resourceType,
			ResourceName: resourceName,
			RequestID:    requestID(ctx),
			Details:      details,
			IP:           httpctx.ClientIP(ctx, trustedProxies),
		})
	}
}
