package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestIsPublicPath(t *testing.T) {
	public := []string{
		"/api/health",
		"/metrics",
		"/api/openapi.yaml",
		"/api/v1/auth/config",
		"/api/v1/auth/login",
		"/api/v1/auth/logout",
		"/api/v1/auth/oidc/login",
		"/api/v1/auth/oidc/callback",
		"/api/v1/auth/oidc/google/login",
	}
	for _, path := range public {
		assert.True(t, isPublicPath(path), "%q should be public", path)
	}

	protected := []string{
		"/api/v1/clusters",
		"/api/v1/clusters/abc/streams",
		"/api/v1/auth/me",
		"/api/v1/users",
		"/api/v1/audit",
	}
	for _, path := range protected {
		assert.False(t, isPublicPath(path), "%q should not be public", path)
	}
}

func TestRequiresAuth(t *testing.T) {
	assert.True(t, requiresAuth("/api/v1/clusters"), "clusters should require auth")
	assert.False(t, requiresAuth("/api/health"), "health should not require auth")
	assert.False(t, requiresAuth("/static/app.js"), "static assets should not require auth")
}

func TestRouteLabel(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/v1/clusters/abc-123/streams")
	require.Equal(t, "/api/v1/clusters/{clusterId}", routeLabel(ctx))

	ctx.Request.SetRequestURI("/api/v1/users")
	require.Equal(t, "/api/v1/users", routeLabel(ctx))
}
