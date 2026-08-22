package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	_ "net/http/pprof" // register on DefaultServeMux for adaptor tests
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestApplyDebugPprofDisabled(t *testing.T) {
	mw := New(config.Config{Pprof: config.PprofConfig{Enabled: false}}, nil, nil)
	var nextCalled bool
	h := mw.ApplyDebugPprof(func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/debug/pprof/")
	h(ctx)

	assert.False(t, nextCalled)
	assert.Equal(t, fasthttp.StatusNotFound, ctx.Response.StatusCode())
}

func TestApplyDebugPprofPassesThroughNonPprof(t *testing.T) {
	mw := New(config.Config{Pprof: config.PprofConfig{Enabled: true}}, nil, nil)
	var nextCalled bool
	h := mw.ApplyDebugPprof(func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/health")
	h(ctx)

	assert.True(t, nextCalled)
	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
}

func TestApplyDebugPprofAuthRequiredUnauthorized(t *testing.T) {
	mw := New(config.Config{Pprof: config.PprofConfig{Enabled: true, AuthEnabled: true}}, nil, nil)
	var nextCalled bool
	h := mw.ApplyDebugPprof(func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/debug/pprof/")
	h(ctx)

	assert.False(t, nextCalled)
	assert.Equal(t, fasthttp.StatusUnauthorized, ctx.Response.StatusCode())
}

func TestApplyDebugPprofAuthOffServesMux(t *testing.T) {
	mw := New(config.Config{Pprof: config.PprofConfig{Enabled: true, AuthEnabled: false}}, nil, nil)
	var nextCalled bool
	h := mw.ApplyDebugPprof(func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/debug/pprof/")
	h(ctx)

	assert.False(t, nextCalled)
	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
}

func TestApplyDebugPprofForbiddenForViewer(t *testing.T) {
	svc := newPprofTestAuthService(t)
	mw := New(config.Config{Pprof: config.PprofConfig{Enabled: true, AuthEnabled: true}}, svc, nil)
	h := mw.ApplyDebugPprof(func(ctx *fasthttp.RequestCtx) {
		t.Fatal("next must not run for /debug/pprof")
	})

	ctx := pprofAuthedCtx(t, mw, svc, domain.User{
		ID:       "viewer-1",
		Username: "viewer",
		Roles:    []string{domain.RoleViewer},
	})
	h(ctx)

	assert.Equal(t, fasthttp.StatusForbidden, ctx.Response.StatusCode())
}

func TestApplyDebugPprofAllowsAdmin(t *testing.T) {
	svc := newPprofTestAuthService(t)
	mw := New(config.Config{Pprof: config.PprofConfig{Enabled: true, AuthEnabled: true}}, svc, nil)
	h := mw.ApplyDebugPprof(func(ctx *fasthttp.RequestCtx) {
		t.Fatal("next must not run for /debug/pprof")
	})

	ctx := pprofAuthedCtx(t, mw, svc, domain.User{
		ID:       "admin-1",
		Username: "admin",
		Roles:    []string{domain.RoleAdmin},
	})
	h(ctx)

	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
}

func newPprofTestAuthService(t *testing.T) *auth.Service {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privPEM := commonstrings.BytesToString(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}))
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	pubPEM := commonstrings.BytesToString(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))

	svc, err := auth.NewService(config.Config{
		Auth: config.AuthConfig{
			SessionPrivateKey: privPEM,
			SessionPublicKey:  pubPEM,
			SessionTTL:        time.Hour,
		},
	}, nil)
	require.NoError(t, err)
	return svc
}

func pprofAuthedCtx(t *testing.T, mw *MwHandler, svc *auth.Service, user domain.User) *fasthttp.RequestCtx {
	t.Helper()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/debug/pprof/")
	ctx.Request.Header.Set("User-Agent", "test-agent")
	fph := mw.requestFingerprint(ctx)
	token, err := svc.CreateSession(context.Background(), user, fph)
	require.NoError(t, err)
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	return ctx
}
