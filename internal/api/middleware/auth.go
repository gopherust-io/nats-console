package middleware

import (
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/ipset"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func (mw *MwHandler) VerifyAuth(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		path := requestPath(ctx)
		if !requiresAuth(path) {
			next(ctx)
			return
		}

		user, ok := mw.Authenticate(ctx)
		if !ok {
			httpstatus.WriteUnauthorized(ctx)
			return
		}

		if commonstrings.IsEmpty(user.ID) {
			httpstatus.WriteUnauthorized(ctx)
			return
		}

		c := httpctx.FromRequest(ctx)
		var (
			loaded domain.User
			err    error
		)
		if requiresFreshAuthz(path, commonstrings.BytesToString(ctx.Method())) {
			loaded, err = mw.authService.LoadUserFresh(c, user.ID)
		} else {
			loaded, err = mw.authService.LoadUserForSession(c, user)
		}
		if err != nil {
			httpstatus.WriteUnauthorized(ctx)
			return
		}
		user = loaded

		c = auth.ContextWithUser(c, user)
		ctx.SetUserValue(ctxKey, c)
		next(ctx)
	}
}

// requiresFreshAuthz forces a DB reload of grants/roles for credential and
// access-management paths so multi-replica cache lag cannot authorize revoked rights.
func requiresFreshAuthz(path, method string) bool {
	if strings.Contains(path, "/creds") ||
		strings.Contains(path, "/assign") ||
		(strings.Contains(path, "/nats-users/") && strings.HasSuffix(path, "/rotate")) ||
		strings.Contains(path, "/mint-jwt") ||
		strings.Contains(path, "/access/") ||
		strings.HasSuffix(path, "/access") ||
		strings.Contains(path, "/grants") {
		return true
	}
	mutating := method != fasthttp.MethodGet && method != fasthttp.MethodHead && method != fasthttp.MethodOptions
	if mutating {
		if strings.HasPrefix(path, "/api/v1/users") || strings.HasPrefix(path, "/api/v1/people") {
			return true
		}
		// NATS account control-plane mutations after revoke must not use stale grants.
		if strings.Contains(path, "/nats-users") ||
			strings.Contains(path, "/signing-groups") ||
			strings.Contains(path, "/sharing") {
			return true
		}
	}
	return false
}

func (mw *MwHandler) requestFingerprint(ctx *fasthttp.RequestCtx) string {
	trusted := ipset.ParseTrustedProxies(mw.cfg.TrustedProxyList())
	ip := httpctx.ClientIP(ctx, trusted)
	ua := commonstrings.BytesToString(ctx.Request.Header.UserAgent())
	return auth.DeviceFingerprint(ua, ip)
}

func (mw *MwHandler) Authenticate(ctx *fasthttp.RequestCtx) (domain.User, bool) {
	fph := mw.requestFingerprint(ctx)

	if cookie := ctx.Request.Header.Cookie(auth.SessionCookie); len(cookie) > 0 {
		user, err := mw.authService.ParseSession(httpctx.FromRequest(ctx), commonstrings.BytesToString(cookie), fph)
		if err == nil {
			return user, true
		}
	}

	authHeader := commonstrings.BytesToString(ctx.Request.Header.Peek(HeaderAuthorization))

	if after, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
		token := strings.TrimSpace(after)
		if commonstrings.IsEmpty(token) {
			return domain.User{}, false
		}
		user, err := mw.authService.ParseSession(httpctx.FromRequest(ctx), token, fph)
		return user, err == nil
	}

	if strings.HasPrefix(authHeader, "Basic ") {
		username, password, ok := auth.ParseBasicAuth(authHeader)
		if !ok {
			return domain.User{}, false
		}
		user, err := mw.authService.AuthenticateBasic(httpctx.FromRequest(ctx), username, password)
		return user, err == nil
	}

	return domain.User{}, false
}

func (mw *MwHandler) VerifyRBAC(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		path := requestPath(ctx)
		if !requiresAuth(path) {
			next(ctx)
			return
		}

		c := httpctx.FromRequest(ctx)
		user, ok := auth.UserFromContext(c)
		if !ok {
			httpstatus.WriteUnauthorized(ctx)
			return
		}
		method := commonstrings.BytesToString(ctx.Method())

		if strings.HasPrefix(path, "/api/v1/admin") && !user.IsRoot {
			httpstatus.WriteForbidden(ctx)
			return
		}

		if method == fasthttp.MethodGet || method == fasthttp.MethodHead || method == fasthttp.MethodOptions {
			if strings.HasPrefix(path, "/api/v1/audit") && !auth.CanViewAudit(user) {
				httpstatus.WriteForbidden(ctx)
				return
			}
			if (strings.HasPrefix(path, "/api/v1/users") || strings.HasPrefix(path, "/api/v1/people")) && !auth.CanManageUsers(user) {
				httpstatus.WriteForbidden(ctx)
				return
			}
			if strings.HasPrefix(path, "/api/v1/alert-rules") && !auth.CanManageAlertRules(user) {
				httpstatus.WriteForbidden(ctx)
				return
			}
			if strings.HasPrefix(path, "/api/v1/pprof") && !auth.CanViewProfiling(user) {
				httpstatus.WriteForbidden(ctx)
				return
			}
			if clusterID := clusterIDFromPath(path); !commonstrings.IsEmpty(clusterID) && !canReadClusterPath(user, clusterID, path) {
				httpstatus.WriteForbidden(ctx)
				return
			}
			next(ctx)
			return
		}

		// Acknowledge is allowed for any authenticated user with cluster access (checked in handler).
		if method == fasthttp.MethodPost && strings.HasPrefix(path, "/api/v1/alerts/") && strings.HasSuffix(path, "/acknowledge") {
			next(ctx)
			return
		}

		if method == fasthttp.MethodDelete && strings.HasPrefix(path, "/api/v1/clusters/") && !strings.Contains(strings.TrimPrefix(path, "/api/v1/clusters/"), "/") {
			if !auth.CanDeleteCluster(user) {
				httpstatus.WriteForbidden(ctx)
				return
			}
		}

		if clusterID := clusterIDFromPath(path); !commonstrings.IsEmpty(clusterID) {
			if !canMutateClusterPath(user, clusterID, path) {
				httpstatus.WriteForbidden(ctx)
				return
			}
		} else if strings.HasPrefix(path, "/api/v1/alert-rules") {
			if !auth.CanManageAlertRules(user) {
				httpstatus.WriteForbidden(ctx)
				return
			}
		} else if !auth.CanWrite(user) {
			httpstatus.WriteForbidden(ctx)
			return
		}

		if strings.HasPrefix(path, "/api/v1/users") || strings.HasPrefix(path, "/api/v1/people") {
			if !auth.CanManageUsers(user) {
				httpstatus.WriteForbidden(ctx)
				return
			}
		}
		if strings.HasPrefix(path, "/api/v1/audit") {
			if !auth.CanViewAudit(user) {
				httpstatus.WriteForbidden(ctx)
				return
			}
		}

		next(ctx)
	}
}
