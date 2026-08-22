package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/domain"
)

func TestRequiresFreshAuthz(t *testing.T) {
	t.Parallel()
	assert.True(t, requiresFreshAuthz("/api/v1/clusters/x/nats-users/y/creds", fasthttp.MethodGet))
	assert.True(t, requiresFreshAuthz("/api/v1/clusters/x/nats-users/y/assign", fasthttp.MethodPost))
	assert.True(t, requiresFreshAuthz("/api/v1/clusters/x/nats-users/y/rotate", fasthttp.MethodPost))
	assert.True(t, requiresFreshAuthz("/api/v1/clusters/x/nats-users", fasthttp.MethodPost))
	assert.True(t, requiresFreshAuthz("/api/v1/clusters/x/signing-groups", fasthttp.MethodPut))
	assert.True(t, requiresFreshAuthz("/api/v1/clusters/x/sharing/exports", fasthttp.MethodDelete))
	assert.False(t, requiresFreshAuthz("/api/v1/clusters/x/nats-users", fasthttp.MethodGet))
	assert.True(t, requiresFreshAuthz("/api/v1/clusters/x/access", fasthttp.MethodPost))
	assert.False(t, requiresFreshAuthz("/api/v1/clusters/x/streams", fasthttp.MethodGet))
	assert.True(t, requiresFreshAuthz("/api/v1/users", fasthttp.MethodPost))
	assert.False(t, requiresFreshAuthz("/api/v1/users", fasthttp.MethodGet))
}

func TestIsPublicPath(t *testing.T) {
	public := []string{
		"/api/health",
		"/api/openapi.yaml",
		"/api/v1/schemas",
		"/api/v1/auth/config",
		"/api/v1/auth/login",
		"/api/v1/auth/logout",
		"/api/v1/auth/refresh",
		"/api/v1/auth/invite/accept",
		"/api/v1/auth/invite/abc123",
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
		"/api/v1/auth/oidc/login",
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

func TestIsLongRunningProfilePathStillWorks(t *testing.T) {
	// Keep package tests compiling alongside alerts additions.
	assert.True(t, isPprofPath("/debug/pprof"))
}

func TestIsAITimeoutPath(t *testing.T) {
	assert.True(t, isAITimeoutPath("/api/v1/assistant/config"))
	assert.True(t, isAITimeoutPath("/api/v1/clusters/abc/assistant/chat"))
	assert.True(t, isAITimeoutPath("/api/v1/clusters/abc/architecture-review/ask"))
	assert.True(t, isAITimeoutPath("/api/v1/clusters/abc/chaos-story/generate"))
	assert.False(t, isAITimeoutPath("/api/v1/clusters/abc/streams"))
	assert.False(t, isAITimeoutPath("/api/health"))
}

func TestIsLongRunningProfilePath(t *testing.T) {
	assert.True(t, isLongRunningProfilePath("/debug/pprof/profile"))
	assert.True(t, isLongRunningProfilePath("/api/v1/pprof/profile/cpu"))
	assert.False(t, isLongRunningProfilePath("/api/v1/pprof/runtime"))
	assert.False(t, isLongRunningProfilePath("/api/v1/pprof/config"))
}

func TestIsPprofPath(t *testing.T) {
	assert.True(t, isPprofPath("/debug/pprof"))
	assert.True(t, isPprofPath("/debug/pprof/heap"))
	assert.False(t, isPprofPath("/api/v1/pprof/config"))
}

func TestIsAccountScopedClusterPath(t *testing.T) {
	id := "550e8400-e29b-41d4-a716-446655440000"
	scoped := []string{
		"/api/v1/clusters/" + id,
		"/api/v1/clusters/" + id + "/connection",
		"/api/v1/clusters/" + id + "/connection/events",
		"/api/v1/clusters/" + id + "/access",
		"/api/v1/clusters/" + id + "/accounts/Default/access",
		"/api/v1/clusters/" + id + "/nats-users",
		"/api/v1/clusters/" + id + "/nats-users/u1/creds",
		"/api/v1/clusters/" + id + "/subject-permissions",
		"/api/v1/clusters/" + id + "/signing-groups",
		"/api/v1/clusters/" + id + "/sharing/exports",
	}
	for _, path := range scoped {
		assert.True(t, isAccountScopedClusterPath(path), "%q should be account-scoped", path)
	}

	clusterWide := []string{
		"/api/v1/clusters/" + id + "/streams",
		"/api/v1/clusters/" + id + "/kv/buckets",
		"/api/v1/clusters/" + id + "/objects/buckets",
		"/api/v1/clusters/" + id + "/topology",
		"/api/v1/clusters/" + id + "/monitoring/varz",
		"/api/v1/clusters/" + id + "/metrics/history",
		"/api/v1/clusters/" + id + "/live/ws",
		"/api/v1/clusters/" + id + "/account",
		"/api/v1/clusters/" + id + "/test",
	}
	for _, path := range clusterWide {
		assert.False(t, isAccountScopedClusterPath(path), "%q should be cluster-wide", path)
	}
}

func TestClusterIDFromPathMixedCase(t *testing.T) {
	t.Parallel()
	lower := "550e8400-e29b-41d4-a716-446655440000"
	upper := "550E8400-E29B-41D4-A716-446655440000"
	assert.Equal(t, lower, clusterIDFromPath("/api/v1/clusters/"+upper+"/streams"))
	assert.Equal(t, lower, clusterIDFromPath("/api/v1/clusters/"+lower+"/streams"))

	outsider := domain.User{Roles: []string{domain.RoleViewer}}
	assert.False(t, canReadClusterPath(outsider, clusterIDFromPath("/api/v1/clusters/"+upper+"/streams"), "/api/v1/clusters/"+upper+"/streams"))
}

func TestCanReadClusterPath(t *testing.T) {
	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	accountUser := domain.User{
		Roles: []string{domain.RoleViewer},
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceAccount,
			ResourceKey:  clusterID + ":Default",
			Role:         domain.GrantAdmin,
		}},
	}

	// Account grant holders may read account-scoped sub-resources...
	assert.True(t, canReadClusterPath(accountUser, clusterID, "/api/v1/clusters/"+clusterID+"/nats-users"))
	assert.True(t, canReadClusterPath(accountUser, clusterID, "/api/v1/clusters/"+clusterID))
	// ...but not cluster-wide resources like streams, topology, or monitoring.
	assert.False(t, canReadClusterPath(accountUser, clusterID, "/api/v1/clusters/"+clusterID+"/streams"))
	assert.False(t, canReadClusterPath(accountUser, clusterID, "/api/v1/clusters/"+clusterID+"/topology"))
	assert.False(t, canReadClusterPath(accountUser, clusterID, "/api/v1/clusters/"+clusterID+"/monitoring/varz"))

	systemUser := domain.User{
		Roles: []string{domain.RoleViewer},
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceSystem,
			ResourceKey:  clusterID,
			Role:         domain.GrantAdmin,
		}},
	}
	assert.True(t, canReadClusterPath(systemUser, clusterID, "/api/v1/clusters/"+clusterID+"/streams"))
	assert.True(t, canReadClusterPath(systemUser, clusterID, "/api/v1/clusters/"+clusterID+"/nats-users"))
}

func TestCanMutateClusterPathAccountAdminWithoutClusterWrite(t *testing.T) {
	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	accountAdmin := domain.User{
		Roles: []string{domain.RoleViewer},
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceAccount,
			ResourceKey:  clusterID + ":Default",
			Role:         domain.GrantAdmin,
		}},
	}

	assert.False(t, auth.CanWriteCluster(accountAdmin, clusterID))
	assert.True(t, canMutateClusterPath(accountAdmin, clusterID, "/api/v1/clusters/"+clusterID+"/nats-users"))
	assert.True(t, canMutateClusterPath(accountAdmin, clusterID, "/api/v1/clusters/"+clusterID+"/signing-groups"))
	assert.False(t, canMutateClusterPath(accountAdmin, clusterID, "/api/v1/clusters/"+clusterID+"/streams"))
}
