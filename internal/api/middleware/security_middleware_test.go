package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/store"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestSecurityHeaders(t *testing.T) {
	cfg := config.Config{PublicBaseURL: "https://nats.example.com"}
	mw := New(cfg, nil, nil)
	var called bool
	h := mw.ApplySecurityHeaders(func(ctx *fasthttp.RequestCtx) {
		called = true
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/health")
	h(ctx)

	require.True(t, called, "handler not called")
	assert.Equal(t, "nosniff", commonstrings.BytesToString(ctx.Response.Header.Peek("X-Content-Type-Options")))
	assert.Equal(t, "DENY", commonstrings.BytesToString(ctx.Response.Header.Peek("X-Frame-Options")))
	assert.NotEmpty(t, commonstrings.BytesToString(ctx.Response.Header.Peek("Strict-Transport-Security")), "expected HSTS on https public base url")
	assert.NotEmpty(t, commonstrings.BytesToString(ctx.Response.Header.Peek("Content-Security-Policy")), "expected CSP header")
}

func TestCSRFRequiredForSessionMutations(t *testing.T) {
	cfg := config.Config{}
	mw := New(cfg, nil, nil)
	h := mw.VerifyCSRF(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/v1/clusters")
	ctx.Request.Header.SetCookie(auth.SessionCookie, "session-token")
	h(ctx)
	require.Equal(t, fasthttp.StatusForbidden, ctx.Response.StatusCode())
	assert.Contains(t, string(ctx.Response.Body()), `"code":"csrf_invalid"`)

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/v1/clusters")
	ctx.Request.Header.SetCookie(auth.SessionCookie, "session-token")
	ctx.Request.Header.SetCookie(auth.CSRFCookie, "csrf-token")
	ctx.Request.Header.Set(HeaderCSRF, "csrf-token")
	h(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/v1/clusters")
	ctx.Request.Header.SetCookie(auth.SessionCookie, "session-token")
	ctx.Request.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	h(ctx)
	require.Equal(t, fasthttp.StatusForbidden, ctx.Response.StatusCode(), "session cookie must still require CSRF even with Basic header")

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/v1/clusters")
	ctx.Request.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	h(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "Basic-only (no session cookie) skips CSRF")

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/v1/clusters")
	ctx.Request.Header.Set("Authorization", "Bearer some.jwt.token")
	h(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "Bearer-only (no session cookie) skips CSRF")
}

func TestAuthenticateAcceptsBearerRS256(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}))
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))

	svc, err := auth.NewService(config.Config{
		Auth: config.AuthConfig{
			SessionPrivateKey: privPEM,
			SessionPublicKey:  pubPEM,
			SessionTTL:        time.Hour,
		},
	}, nil)
	require.NoError(t, err)

	mw := New(config.Config{}, svc, nil)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/v1/clusters")
	ctx.Request.Header.Set("User-Agent", "test-agent")
	ctx.Request.Header.Set("Authorization", "Bearer placeholder")

	fph := mw.requestFingerprint(ctx)
	token, err := svc.CreateSession(context.Background(), store.User{
		ID:       "user-1",
		Username: "alice",
		Roles:    []string{store.RoleAdmin},
	}, fph)
	require.NoError(t, err)
	ctx.Request.Header.Set("Authorization", "Bearer "+token)

	user, ok := mw.Authenticate(ctx)
	require.True(t, ok)
	assert.Equal(t, "user-1", user.ID)
	assert.Equal(t, "alice", user.Username)
}

func TestAuthRateLimiter(t *testing.T) {
	rl := newIPRateLimiter(2, time.Minute)
	assert.True(t, rl.allow("1.2.3.4"), "first two requests should pass")
	assert.True(t, rl.allow("1.2.3.4"), "first two requests should pass")
	assert.False(t, rl.allow("1.2.3.4"), "third request should be blocked")
	assert.True(t, rl.allow("5.6.7.8"), "different IP should not be blocked")
}

func TestAuthRateLimitResponseBody(t *testing.T) {
	cfg := config.Config{Auth: config.AuthConfig{
		RateLimit:       1,
		RateLimitWindow: time.Minute,
	}}
	mw := New(cfg, nil, nil)
	h := mw.VerifyAuthRateLimit(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	okCtx := &fasthttp.RequestCtx{}
	okCtx.Request.Header.SetMethod(fasthttp.MethodPost)
	okCtx.Request.SetRequestURI("/api/v1/auth/login")
	okCtx.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP("10.0.0.9"), Port: 1234})
	h(okCtx)
	require.Equal(t, fasthttp.StatusOK, okCtx.Response.StatusCode())

	blocked := &fasthttp.RequestCtx{}
	blocked.Request.Header.SetMethod(fasthttp.MethodPost)
	blocked.Request.SetRequestURI("/api/v1/auth/login")
	blocked.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP("10.0.0.9"), Port: 1235})
	h(blocked)
	require.Equal(t, fasthttp.StatusTooManyRequests, blocked.Response.StatusCode())
	assert.Equal(t, "60", string(blocked.Response.Header.Peek("Retry-After")))
	assert.Contains(t, string(blocked.Response.Body()), `"code":"rate_limit"`)
	assert.Contains(t, string(blocked.Response.Body()), `"retryable":true`)
	assert.Contains(t, string(blocked.Response.Body()), `"retryAfterSeconds":60`)
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	cfg := config.Config{CORSAllowedOrigins: "https://allowed.example.com"}
	mw := New(cfg, nil, nil)
	h := mw.VerifyCors(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodOptions)
	ctx.Request.SetRequestURI("/api/v1/clusters")
	ctx.Request.Header.Set("Origin", "https://evil.example.com")
	h(ctx)
	require.Equal(t, fasthttp.StatusForbidden, ctx.Response.StatusCode(), "OPTIONS from unknown origin")
	assert.Empty(t, commonstrings.BytesToString(ctx.Response.Header.Peek("Access-Control-Allow-Origin")), "unexpected ACAO")
}
