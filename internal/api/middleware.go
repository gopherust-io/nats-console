package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"

	"github.com/gopherust-io/nats-consol/internal/audit"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/log"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
)

const (
	requestIDKey = "request_id"
	pathKey      = "path"
)

func isAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/")
}

func requiresAuth(path string) bool {
	return isAPIPath(path) && !isPublicPath(path)
}

type middleware func(fasthttp.RequestHandler) fasthttp.RequestHandler

func chain(mws ...middleware) middleware {
	return func(final fasthttp.RequestHandler) fasthttp.RequestHandler {
		for _, v := range slices.Backward(mws) {
			final = v(final)
		}
		return final
	}
}

func requestIDMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())
		ctx.SetUserValue(pathKey, path)
		id := string(ctx.Request.Header.Peek("X-Request-ID"))
		if id == "" {
			var b [8]byte
			_, _ = rand.Read(b[:])
			id = hex.EncodeToString(b[:])
		}
		ctx.SetUserValue(requestIDKey, id)
		ctx.Response.Header.Set("X-Request-ID", id)
		next(ctx)
	}
}

func requestPath(ctx *fasthttp.RequestCtx) string {
	if path, ok := ctx.UserValue(pathKey).(string); ok && path != "" {
		return path
	}
	return string(ctx.Path())
}

func timeoutMiddleware(timeout time.Duration) middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			if isLongRunningProfilePath(requestPath(ctx)) {
				next(ctx)
				return
			}
			c, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			ctx.SetUserValue("context", c)
			next(ctx)
		}
	}
}

func requestLogMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		path := requestPath(ctx)
		if !isAPIPath(path) {
			next(ctx)
			return
		}
		start := time.Now()
		next(ctx)
		log.Info().
			Str("component", "http").
			Str("method", string(ctx.Method())).
			Str("path", path).
			Int("status", ctx.Response.StatusCode()).
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Str("request_id", requestID(ctx)).
			Msg("request")
	}
}

func metricsMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		if !isAPIPath(requestPath(ctx)) {
			next(ctx)
			return
		}
		start := time.Now()
		next(ctx)
		metrics.ObserveHTTP(string(ctx.Method()), routeLabel(ctx), ctx.Response.StatusCode(), time.Since(start))
	}
}

func routeLabel(ctx *fasthttp.RequestCtx) string {
	path := requestPath(ctx)
	if strings.HasPrefix(path, "/api/v1/clusters/") {
		return "/api/v1/clusters/{clusterId}"
	}
	return path
}

func corsMiddleware(cfg config.Config) middleware {
	allowed := make(map[string]struct{}, len(cfg.CORSOrigins()))
	for _, origin := range cfg.CORSOrigins() {
		allowed[origin] = struct{}{}
	}
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			origin := string(ctx.Request.Header.Peek("Origin"))
			if origin != "" {
				if _, ok := allowed[origin]; ok {
					ctx.Response.Header.Set("Access-Control-Allow-Origin", origin)
					ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
					ctx.Response.Header.Set("Vary", "Origin")
				}
			}
			ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			ctx.Response.Header.Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-Request-ID, X-CSRF-Token")

			if ctx.IsOptions() {
				if origin != "" {
					if _, ok := allowed[origin]; ok {
						ctx.SetStatusCode(fasthttp.StatusNoContent)
						return
					}
				}
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				return
			}
			next(ctx)
		}
	}
}

func authMiddleware(cfg config.Config, authSvc *auth.Service) middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			path := requestPath(ctx)
			if !requiresAuth(path) {
				next(ctx)
				return
			}

			user, ok := authenticate(ctx, authSvc)
			if !ok {
				ctx.SetStatusCode(fasthttp.StatusUnauthorized)
				ctx.SetContentType("application/json")
				ctx.SetBodyString(`{"error":"unauthorized"}`)
				return
			}

			if user.ID != "" {
				loaded, err := authSvc.LoadUser(requestContext(ctx), user.ID)
				if err != nil {
					ctx.SetStatusCode(fasthttp.StatusUnauthorized)
					ctx.SetContentType("application/json")
					ctx.SetBodyString(`{"error":"unauthorized"}`)
					return
				}
				user = loaded
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
		path := requestPath(ctx)
		if !requiresAuth(path) {
			next(ctx)
			return
		}

		c := requestContext(ctx)
		user, ok := auth.UserFromContext(c)
		if !ok {
			ctx.SetStatusCode(fasthttp.StatusUnauthorized)
			return
		}
		method := string(ctx.Method())

		if strings.HasPrefix(path, "/api/v1/admin") && !user.IsRoot {
			ctx.SetStatusCode(fasthttp.StatusForbidden)
			ctx.SetBodyString("forbidden")
			return
		}

		if method == fasthttp.MethodGet || method == fasthttp.MethodHead || method == fasthttp.MethodOptions {
			if strings.HasPrefix(path, "/api/v1/audit") && !auth.CanViewAudit(user) {
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				ctx.SetBodyString("forbidden")
				return
			}
			if strings.HasPrefix(path, "/api/v1/users") && !auth.CanManageUsers(user) {
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				ctx.SetBodyString("forbidden")
				return
			}
			if strings.HasPrefix(path, "/api/v1/pprof") && !auth.CanViewProfiling(user) {
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				ctx.SetBodyString("forbidden")
				return
			}
			if strings.Contains(path, "/resolver/export") && !auth.CanViewProfiling(user) {
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				ctx.SetBodyString("forbidden")
				return
			}
			if clusterID := clusterIDFromPath(path); clusterID != "" && !auth.CanAccessCluster(user, clusterID) {
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				ctx.SetBodyString("forbidden")
				return
			}
			next(ctx)
			return
		}

		if !auth.CanWrite(user) {
			ctx.SetStatusCode(fasthttp.StatusForbidden)
			ctx.SetBodyString("forbidden")
			return
		}

		if method == fasthttp.MethodPost && path == "/api/v1/clusters" && !auth.CanCreateCluster(user) {
			ctx.SetStatusCode(fasthttp.StatusForbidden)
			ctx.SetBodyString("forbidden")
			return
		}

		if method == fasthttp.MethodDelete && strings.HasPrefix(path, "/api/v1/clusters/") && !strings.Contains(strings.TrimPrefix(path, "/api/v1/clusters/"), "/") {
			if !auth.CanDeleteCluster(user) {
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				ctx.SetBodyString("forbidden")
				return
			}
		}

		if strings.HasPrefix(path, "/api/v1/users") {
			if !auth.CanManageUsers(user) {
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				ctx.SetBodyString("forbidden")
				return
			}
		}
		if strings.HasPrefix(path, "/api/v1/audit") {
			if !auth.CanViewAudit(user) {
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				ctx.SetBodyString("forbidden")
				return
			}
		}

		if clusterID := clusterIDFromPath(path); clusterID != "" && !auth.CanAccessCluster(user, clusterID) {
			ctx.SetStatusCode(fasthttp.StatusForbidden)
			ctx.SetBodyString("forbidden")
			return
		}

		next(ctx)
	}
}

func auditMiddleware(auditWriter *audit.Writer) middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			method := string(ctx.Method())
	path := requestPath(ctx)
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
			details := store.AuditRequestDetails{
				Method: method,
				Path:   path,
				Status: ctx.Response.StatusCode(),
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
	if strings.Contains(requestPath(ctx), "/live/ws") {
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
	case "/api/health", "/metrics", "/api/openapi.yaml",
		"/api/v1/auth/config", "/api/v1/auth/login", "/api/v1/auth/logout":
		return true
	default:
		return strings.HasPrefix(path, "/api/v1/auth/oidc/")
	}
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

var promMetricsHandler = fasthttpadaptor.NewFastHTTPHandler(promhttp.Handler())

func openapiHandler(path string) fasthttp.RequestHandler {
	// Path comes from server config (OPENAPI_PATH), not user input.
	data, err := os.ReadFile(path) //nolint:gosec // G304: trusted config path
	if err != nil {
		return func(ctx *fasthttp.RequestCtx) {
			serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
		}
	}
	return func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetContentType("application/yaml")
		ctx.SetBody(data)
	}
}

func writeUserJSON(ctx *fasthttp.RequestCtx, status int, user store.User) {
	serializer.WriteJSON(ctx, status, toUserResponse(user))
}

func parseJSONBody(ctx *fasthttp.RequestCtx, v any) error {
	return serializer.UnmarshalRequest(ctx.PostBody(), v)
}
