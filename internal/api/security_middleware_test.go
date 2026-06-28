package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
)

func TestSecurityHeaders(t *testing.T) {
	cfg := config.Config{PublicBaseURL: "https://nats.example.com"}
	var called bool
	h := securityHeadersMiddleware(cfg)(func(ctx *fasthttp.RequestCtx) {
		called = true
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/health")
	h(ctx)

	require.True(t, called, "handler not called")
	assert.Equal(t, "nosniff", string(ctx.Response.Header.Peek("X-Content-Type-Options")))
	assert.Equal(t, "DENY", string(ctx.Response.Header.Peek("X-Frame-Options")))
	assert.NotEmpty(t, string(ctx.Response.Header.Peek("Strict-Transport-Security")), "expected HSTS on https public base url")
	assert.NotEmpty(t, string(ctx.Response.Header.Peek("Content-Security-Policy")), "expected CSP header")
}

func TestCSRFRequiredForSessionMutations(t *testing.T) {
	cfg := config.Config{AuthEnabled: true}
	h := csrfMiddleware(cfg)(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/v1/clusters")
	ctx.Request.Header.SetCookie(auth.SessionCookie, "session-token")
	h(ctx)
	require.Equal(t, fasthttp.StatusForbidden, ctx.Response.StatusCode())

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/v1/clusters")
	ctx.Request.Header.SetCookie(auth.SessionCookie, "session-token")
	ctx.Request.Header.SetCookie(auth.CSRFCookie, "csrf-token")
	ctx.Request.Header.Set(csrfHeader, "csrf-token")
	h(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/v1/clusters")
	ctx.Request.Header.SetCookie(auth.SessionCookie, "session-token")
	ctx.Request.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	h(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "basic auth should bypass csrf")
}

func TestAuthRateLimiter(t *testing.T) {
	rl := newIPRateLimiter(2, time.Minute)
	assert.True(t, rl.allow("1.2.3.4"), "first two requests should pass")
	assert.True(t, rl.allow("1.2.3.4"), "first two requests should pass")
	assert.False(t, rl.allow("1.2.3.4"), "third request should be blocked")
	assert.True(t, rl.allow("5.6.7.8"), "different IP should not be blocked")
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	cfg := config.Config{CORSAllowedOrigins: "https://allowed.example.com"}
	h := corsMiddleware(cfg)(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodOptions)
	ctx.Request.SetRequestURI("/api/v1/clusters")
	ctx.Request.Header.Set("Origin", "https://evil.example.com")
	h(ctx)
	require.Equal(t, fasthttp.StatusForbidden, ctx.Response.StatusCode(), "OPTIONS from unknown origin")
	assert.Empty(t, string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")), "unexpected ACAO")
}
