package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"

	"github.com/gopherust-io/nats-consol/internal/audit"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/internal/store"
)

const requestIDKey = "request_id"

type middleware func(fasthttp.RequestHandler) fasthttp.RequestHandler

func chain(mws ...middleware) middleware {
	return func(final fasthttp.RequestHandler) fasthttp.RequestHandler {
		for i := len(mws) - 1; i >= 0; i-- {
			final = mws[i](final)
		}
		return final
	}
}

func requestIDMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		id := string(ctx.Request.Header.Peek("X-Request-ID"))
		if id == "" {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		ctx.SetUserValue(requestIDKey, id)
		ctx.Response.Header.Set("X-Request-ID", id)
		next(ctx)
	}
}

func timeoutMiddleware(timeout time.Duration) middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			c, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			ctx.SetUserValue("context", c)
			next(ctx)
		}
	}
}

func slogMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		start := time.Now()
		next(ctx)
		slog.Info("request",
			"method", string(ctx.Method()),
			"path", string(ctx.Path()),
			"status", ctx.Response.StatusCode(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", requestID(ctx),
		)
	}
}

func metricsMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		start := time.Now()
		next(ctx)
		metrics.ObserveHTTP(string(ctx.Method()), routeLabel(ctx), ctx.Response.StatusCode(), time.Since(start))
	}
}

func routeLabel(ctx *fasthttp.RequestCtx) string {
	path := string(ctx.Path())
	if strings.HasPrefix(path, "/api/v1/clusters/") {
		return "/api/v1/clusters/{clusterId}"
	}
	return path
}

func corsMiddleware(cfg config.Config) middleware {
	origins := cfg.CORSOrigins()
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			origin := string(ctx.Request.Header.Peek("Origin"))
			if len(origins) == 0 {
				if origin != "" {
					ctx.Response.Header.Set("Access-Control-Allow-Origin", origin)
					ctx.Response.Header.Set("Vary", "Origin")
				}
			} else {
				for _, allowed := range origins {
					if origin == allowed {
						ctx.Response.Header.Set("Access-Control-Allow-Origin", origin)
						ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
						ctx.Response.Header.Set("Vary", "Origin")
						break
					}
				}
			}
			ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			ctx.Response.Header.Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-Request-ID")

			if ctx.IsOptions() {
				ctx.SetStatusCode(fasthttp.StatusNoContent)
				return
			}
			next(ctx)
		}
	}
}

func authMiddleware(cfg config.Config, authSvc *auth.Service) middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			path := string(ctx.Path())
			if isPublicPath(path) {
				next(ctx)
				return
			}

			user, ok := authenticate(ctx, authSvc)
			if !ok {
				ctx.Response.Header.Set("WWW-Authenticate", `Basic realm="nats-consol"`)
				ctx.SetStatusCode(fasthttp.StatusUnauthorized)
				ctx.SetBodyString("unauthorized")
				return
			}

			c := requestContext(ctx)
			c = auth.ContextWithUser(c, user)
			ctx.SetUserValue("context", c)
			next(ctx)
		}
	}
}

func rbacMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())
		if isPublicPath(path) {
			next(ctx)
			return
		}

		c := requestContext(ctx)
		user, ok := auth.UserFromContext(c)
		if !ok {
			ctx.SetStatusCode(fasthttp.StatusUnauthorized)
			return
		}
		role := store.HighestRole(user.Roles)
		method := string(ctx.Method())

		if method == fasthttp.MethodGet || method == fasthttp.MethodHead || method == fasthttp.MethodOptions {
			next(ctx)
			return
		}

		if role == store.RoleViewer {
			ctx.SetStatusCode(fasthttp.StatusForbidden)
			ctx.SetBodyString("forbidden")
			return
		}

		if method == fasthttp.MethodDelete && strings.HasPrefix(path, "/api/v1/clusters/") && !strings.Contains(strings.TrimPrefix(path, "/api/v1/clusters/"), "/") {
			if !auth.CanDeleteCluster(role) {
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				ctx.SetBodyString("forbidden")
				return
			}
		}

		if strings.HasPrefix(path, "/api/v1/users") || strings.HasPrefix(path, "/api/v1/audit") {
			if !auth.CanManageUsers(role) && strings.HasPrefix(path, "/api/v1/users") {
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				ctx.SetBodyString("forbidden")
				return
			}
			if !auth.CanViewAudit(role) && strings.HasPrefix(path, "/api/v1/audit") {
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				ctx.SetBodyString("forbidden")
				return
			}
		}

		next(ctx)
	}
}

func auditMiddleware(auditWriter *audit.Writer) middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			method := string(ctx.Method())
			path := string(ctx.Path())
			if method == fasthttp.MethodGet || method == fasthttp.MethodHead || method == fasthttp.MethodOptions || isPublicPath(path) {
				next(ctx)
				return
			}

			next(ctx)

			if ctx.Response.StatusCode() >= 400 || auditWriter == nil {
				return
			}

			c := requestContext(ctx)
			user, _ := auth.UserFromContext(c)
			resourceType, resourceName := audit.ParseResource(path)
			details := map[string]any{
				"method": method,
				"path":   path,
				"status": ctx.Response.StatusCode(),
			}

			auditWriter.Log(c, store.AuditCreate{
				Actor:        user.Username,
				Action:       audit.ActionForMethod(method),
				ClusterID:    audit.ClusterIDFromPath(path),
				ResourceType: resourceType,
				ResourceName: resourceName,
				RequestID:    requestID(ctx),
				Details:      details,
				IP:           clientIP(ctx),
			})
		}
	}
}

func authenticate(ctx *fasthttp.RequestCtx, authSvc *auth.Service) (store.User, bool) {
	if cookie := ctx.Request.Header.Cookie(auth.SessionCookie); len(cookie) > 0 {
		user, err := authSvc.ParseSession(string(cookie))
		if err == nil {
			return user, true
		}
	}

	authHeader := string(ctx.Request.Header.Peek("Authorization"))
	if strings.Contains(string(ctx.Path()), "/live/ws") {
		qAuth := string(ctx.QueryArgs().Peek("authorization"))
		if qAuth != "" {
			if !strings.HasPrefix(qAuth, "Basic ") {
				qAuth = "Basic " + qAuth
			}
			authHeader = qAuth
		}
	}

	if strings.HasPrefix(authHeader, "Basic ") {
		if !authSvc.BasicAuthEnabled() {
			return store.User{}, false
		}
		username, password, ok := auth.ParseBasicAuth(authHeader)
		if !ok {
			return store.User{}, false
		}
		user, err := authSvc.AuthenticateBasic(requestContext(ctx), username, password)
		return user, err == nil
	}

	return store.User{}, false
}

func isPublicPath(path string) bool {
	switch path {
	case "/api/health", "/metrics", "/api/openapi.yaml":
		return true
	}
	if strings.HasPrefix(path, "/api/v1/auth/") {
		return true
	}
	return false
}

func requestID(ctx *fasthttp.RequestCtx) string {
	if id, ok := ctx.UserValue(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func clientIP(ctx *fasthttp.RequestCtx) string {
	if ip := string(ctx.Request.Header.Peek("X-Forwarded-For")); ip != "" {
		if host, _, err := net.SplitHostPort(strings.TrimSpace(strings.Split(ip, ",")[0])); err == nil {
			return host
		}
		return strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	return ctx.RemoteIP().String()
}

func metricsHandler(ctx *fasthttp.RequestCtx) {
	fasthttpadaptor.NewFastHTTPHandler(promhttp.Handler())(ctx)
}

func openapiHandler(path string) fasthttp.RequestHandler {
	data, err := os.ReadFile(path)
	if err != nil {
		return func(ctx *fasthttp.RequestCtx) {
			writeError(ctx, fasthttp.StatusNotFound, err)
		}
	}
	return func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetContentType("application/yaml")
		ctx.SetBody(data)
	}
}

func writeUserJSON(ctx *fasthttp.RequestCtx, status int, user store.User) {
	writeJSON(ctx, status, map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
		"roles":    user.Roles,
	})
}

func parseJSONBody(ctx *fasthttp.RequestCtx, v any) error {
	return sonic.Unmarshal(ctx.PostBody(), v)
}
