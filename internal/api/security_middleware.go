package api

import (
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
)

const csrfHeader = "X-CSRF-Token"

func securityHeadersMiddleware(cfg config.Config) middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			h := &ctx.Response.Header
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			h.Set("Content-Security-Policy", buildCSP(cfg))
			if cfg.TLSEnabled() {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next(ctx)
		}
	}
}

func buildCSP(cfg config.Config) string {
	connect := "'self' ws: wss:"
	if len(cfg.CORSOrigins()) > 0 {
		var connectSb36 strings.Builder
		for _, origin := range cfg.CORSOrigins() {
			connectSb36.WriteString(" " + origin)
		}
		connect += connectSb36.String()
	}
	return strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob:",
		"font-src 'self'",
		"connect-src " + connect,
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"object-src 'none'",
	}, "; ")
}

func bodySizeLimitMiddleware(maxBytes int) middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			if cl := ctx.Request.Header.ContentLength(); cl > 0 && int64(cl) > int64(maxBytes) {
				ctx.SetStatusCode(fasthttp.StatusRequestEntityTooLarge)
				ctx.SetBodyString(`{"error":"request body too large"}`)
				return
			}
			if len(ctx.Request.Body()) > maxBytes {
				ctx.SetStatusCode(fasthttp.StatusRequestEntityTooLarge)
				ctx.SetBodyString(`{"error":"request body too large"}`)
				return
			}
			next(ctx)
		}
	}
}

func csrfMiddleware(cfg config.Config) middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			if !cfg.AuthEnabled || !requiresCSRF(ctx) {
				next(ctx)
				return
			}
			token := string(ctx.Request.Header.Peek(csrfHeader))
			cookie := string(ctx.Request.Header.Cookie(auth.CSRFCookie))
			if token == "" || cookie == "" || token != cookie {
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				ctx.SetContentType("application/json")
				ctx.SetBodyString(`{"error":"csrf token missing or invalid"}`)
				return
			}
			next(ctx)
		}
	}
}

func requiresCSRF(ctx *fasthttp.RequestCtx) bool {
	method := string(ctx.Method())
	if method == fasthttp.MethodGet || method == fasthttp.MethodHead || method == fasthttp.MethodOptions {
		return false
	}
	path := string(ctx.Path())
	if !isAPIPath(path) || isPublicPath(path) {
		return false
	}
	if len(ctx.Request.Header.Cookie(auth.SessionCookie)) == 0 {
		return false
	}
	if strings.HasPrefix(string(ctx.Request.Header.Peek("Authorization")), "Basic ") {
		return false
	}
	return true
}

type ipRateLimiter struct {
	events map[string][]time.Time
	limit  int
	window time.Duration
	mu     sync.Mutex
}

func newIPRateLimiter(limit int, window time.Duration) *ipRateLimiter {
	return &ipRateLimiter{
		events: make(map[string][]time.Time),
		limit:  limit,
		window: window,
	}
}

func (rl *ipRateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)
	times := rl.events[key]
	n := 0
	for _, t := range times {
		if t.After(cutoff) {
			times[n] = t
			n++
		}
	}
	times = times[:n]
	if len(times) >= rl.limit {
		rl.events[key] = times
		return false
	}
	rl.events[key] = append(times, now)
	return true
}

func authRateLimitMiddleware(cfg config.Config) middleware {
	limiter := newIPRateLimiter(cfg.AuthRateLimitPerWindow(), cfg.AuthRateLimitDuration())
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			path := string(ctx.Path())
			if !isAuthRateLimitPath(path) {
				next(ctx)
				return
			}
			if !limiter.allow(clientIP(ctx)) {
				ctx.SetStatusCode(fasthttp.StatusTooManyRequests)
				ctx.SetContentType("application/json")
				ctx.SetBodyString(`{"error":"rate limit exceeded"}`)
				return
			}
			next(ctx)
		}
	}
}

func isAuthRateLimitPath(path string) bool {
	switch path {
	case "/api/v1/auth/login":
		return true
	default:
		return strings.HasPrefix(path, "/api/v1/auth/oidc/") &&
			(strings.HasSuffix(path, "/login") || strings.HasSuffix(path, "/callback"))
	}
}
