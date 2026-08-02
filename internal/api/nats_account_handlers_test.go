package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/store"
)

func TestNatsUserIDFromCtxRejectsNonUUID(t *testing.T) {
	t.Parallel()

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI("/api/v1/clusters/550e8400-e29b-41d4-a716-446655440000/nats-users/subject-permissions")
	req.Header.SetMethod(fasthttp.MethodGet)

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(req, nil, nil)
	ctx.SetUserValue("userId", "subject-permissions")
	ctx.SetUserValue("context", httpctx.FromRequest(ctx))

	id, ok := natsUserIDFromCtx(ctx)
	assert.False(t, ok)
	assert.Empty(t, id)
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
}

func TestNatsUserIDFromCtxAcceptsUUID(t *testing.T) {
	t.Parallel()

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI("/x")
	req.Header.SetMethod(fasthttp.MethodGet)

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(req, nil, nil)
	want := "550e8400-e29b-41d4-a716-446655440000"
	ctx.SetUserValue("userId", want)

	id, ok := natsUserIDFromCtx(ctx)
	require.True(t, ok)
	assert.Equal(t, want, id)
}

func TestRequireAccountAccessCrossAccountDenied(t *testing.T) {
	t.Parallel()

	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI("/api/v1/clusters/" + clusterID + "/nats-users?account=Other")
	req.Header.SetMethod(fasthttp.MethodGet)

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(req, nil, nil)
	ctx.SetUserValue("clusterId", clusterID)
	base := httpctx.FromRequest(ctx)
	user := store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceAccount,
			ResourceKey:  clusterID + ":Default",
			Role:         store.GrantObserver,
		}},
	}
	ctx.SetUserValue("context", auth.ContextWithUser(base, user))

	assert.True(t, requireAccountAccess(ctx, "Default"))
	assert.False(t, requireAccountAccess(ctx, "Other"))
	assert.Equal(t, fasthttp.StatusForbidden, ctx.Response.StatusCode())
}

func TestRequireAccountAccessDeniesWithoutActor(t *testing.T) {
	t.Parallel()

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI("/x")
	req.Header.SetMethod(fasthttp.MethodGet)

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(req, nil, nil)
	ctx.SetUserValue("context", httpctx.FromRequest(ctx))

	assert.False(t, requireAccountAccess(ctx, "Default"))
	assert.Equal(t, fasthttp.StatusUnauthorized, ctx.Response.StatusCode())
}

func TestRequireManageAccountAccessBlocksOperatorWriteOnly(t *testing.T) {
	t.Parallel()

	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI("/api/v1/clusters/" + clusterID + "/nats-users/x/assign")
	req.Header.SetMethod(fasthttp.MethodPost)

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(req, nil, nil)
	ctx.SetUserValue("clusterId", clusterID)
	base := httpctx.FromRequest(ctx)
	operator := store.User{
		ID:    "op-1",
		Roles: []string{store.RoleOperator},
		AccessRules: &store.AccessRules{
			ClusterIDs: []string{clusterID},
		},
	}
	ctx.SetUserValue("context", auth.ContextWithUser(base, operator))

	assert.False(t, requireManageAccountAccess(ctx, "Default"))
	assert.Equal(t, fasthttp.StatusForbidden, ctx.Response.StatusCode())
	// Operator still has account access for reads, but not assign/share.
	assert.True(t, auth.CanAccessAccount(operator, clusterID, "Default"))
	assert.False(t, auth.CanDownloadCreds(operator, clusterID, "Default", "any"))
}

func TestFilterNATSUsersForGrantsSingleUser(t *testing.T) {
	t.Parallel()

	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	userA := "660e8400-e29b-41d4-a716-446655440000"
	userB := "770e8400-e29b-41d4-a716-446655440000"
	holder := store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceNATSUser,
			ResourceKey:  clusterID + ":Default:" + userA,
			Role:         store.GrantCredentialDownloader,
		}},
	}
	users := []domain.NATSAccountUser{
		{ID: userA, Name: "a"},
		{ID: userB, Name: "b"},
	}
	got := filterNATSUsersForGrants(holder, clusterID, "Default", users)
	require.Len(t, got, 1)
	assert.Equal(t, userA, got[0].ID)
	assert.True(t, canViewNATSUser(holder, clusterID, "Default", userA))
	assert.False(t, canViewNATSUser(holder, clusterID, "Default", userB))
}

func TestFilterSubjectPermissionsForGrantsSingleUser(t *testing.T) {
	t.Parallel()

	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	userA := "660e8400-e29b-41d4-a716-446655440000"
	userB := "770e8400-e29b-41d4-a716-446655440000"
	holder := store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceNATSUser,
			ResourceKey:  clusterID + ":Default:" + userA,
			Role:         store.GrantCredentialDownloader,
		}},
	}
	result := domain.SubjectPermissionsResult{
		Subject: "foo",
		Publish: []domain.SubjectPermissionEntry{
			{UserID: userA, Name: "a"},
			{UserID: userB, Name: "b"},
		},
		Subscribe: []domain.SubjectPermissionEntry{
			{UserID: userB, Name: "b"},
		},
		QueueSubscribe: []domain.SubjectPermissionEntry{
			{UserID: userA, Name: "a"},
		},
	}
	got := filterSubjectPermissionsForGrants(holder, clusterID, "Default", result)
	require.Len(t, got.Publish, 1)
	assert.Equal(t, userA, got.Publish[0].UserID)
	assert.Empty(t, got.Subscribe)
	require.Len(t, got.QueueSubscribe, 1)
	assert.Equal(t, userA, got.QueueSubscribe[0].UserID)
}

func TestRequireListAllAccountNATSUsersDeniedForSingleUserGrant(t *testing.T) {
	t.Parallel()

	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	userA := "660e8400-e29b-41d4-a716-446655440000"
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI("/api/v1/clusters/" + clusterID + "/signing-groups")
	req.Header.SetMethod(fasthttp.MethodGet)

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(req, nil, nil)
	ctx.SetUserValue("clusterId", clusterID)
	base := httpctx.FromRequest(ctx)
	holder := store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceNATSUser,
			ResourceKey:  clusterID + ":Default:" + userA,
			Role:         store.GrantObserver,
		}},
	}
	ctx.SetUserValue("context", auth.ContextWithUser(base, holder))

	assert.False(t, requireListAllAccountNATSUsers(ctx, "Default"))
	assert.Equal(t, fasthttp.StatusForbidden, ctx.Response.StatusCode())
}

func TestRequireManageAccountAccessAllowsAccountAdmin(t *testing.T) {
	t.Parallel()

	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI("/x")
	req.Header.SetMethod(fasthttp.MethodPost)

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(req, nil, nil)
	ctx.SetUserValue("clusterId", clusterID)
	base := httpctx.FromRequest(ctx)
	admin := store.User{
		ID:    "admin-1",
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceAccount,
			ResourceKey:  clusterID + ":Default",
			Role:         store.GrantAdmin,
		}},
	}
	ctx.SetUserValue("context", auth.ContextWithUser(base, admin))

	assert.True(t, requireManageAccountAccess(ctx, "Default"))
}

func TestRequireMutateAccountAccess(t *testing.T) {
	t.Parallel()

	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	userA := "660e8400-e29b-41d4-a716-446655440000"

	withUser := func(u store.User) *fasthttp.RequestCtx {
		req := fasthttp.AcquireRequest()
		t.Cleanup(func() { fasthttp.ReleaseRequest(req) })
		req.SetRequestURI("/x")
		req.Header.SetMethod(fasthttp.MethodPost)
		ctx := &fasthttp.RequestCtx{}
		ctx.Init(req, nil, nil)
		ctx.SetUserValue("clusterId", clusterID)
		ctx.SetUserValue("context", auth.ContextWithUser(httpctx.FromRequest(ctx), u))
		return ctx
	}

	t.Run("deniesObserver", func(t *testing.T) {
		t.Parallel()
		ctx := withUser(store.User{
			Roles: []string{store.RoleViewer},
			Grants: []store.AccessGrant{{
				ResourceType: store.ResourceAccount,
				ResourceKey:  clusterID + ":Default",
				Role:         store.GrantObserver,
			}},
		})
		assert.False(t, requireMutateAccountAccess(ctx, "Default"))
		assert.Equal(t, fasthttp.StatusForbidden, ctx.Response.StatusCode())
	})

	t.Run("deniesNATSUserGrant", func(t *testing.T) {
		t.Parallel()
		ctx := withUser(store.User{
			Roles: []string{store.RoleViewer},
			Grants: []store.AccessGrant{{
				ResourceType: store.ResourceNATSUser,
				ResourceKey:  clusterID + ":Default:" + userA,
				Role:         store.GrantAdmin,
			}},
		})
		assert.False(t, requireMutateAccountAccess(ctx, "Default"))
		assert.Equal(t, fasthttp.StatusForbidden, ctx.Response.StatusCode())
	})

	t.Run("allowsAccountAdmin", func(t *testing.T) {
		t.Parallel()
		ctx := withUser(store.User{
			Roles: []string{store.RoleViewer},
			Grants: []store.AccessGrant{{
				ResourceType: store.ResourceAccount,
				ResourceKey:  clusterID + ":Default",
				Role:         store.GrantAdmin,
			}},
		})
		assert.True(t, requireMutateAccountAccess(ctx, "Default"))
	})

	t.Run("allowsClusterWriteOperator", func(t *testing.T) {
		t.Parallel()
		ctx := withUser(store.User{
			Roles: []string{store.RoleOperator},
			AccessRules: &store.AccessRules{
				ClusterIDs: []string{clusterID},
			},
		})
		assert.True(t, requireMutateAccountAccess(ctx, "Default"))
	})
}

func TestAccountFromCtxDefaultsAndQuery(t *testing.T) {
	t.Parallel()

	h := &NATSAccountHandler{}

	t.Run("default", func(t *testing.T) {
		t.Parallel()
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		req.SetRequestURI("/x")
		ctx := &fasthttp.RequestCtx{}
		ctx.Init(req, nil, nil)
		assert.Equal(t, "Default", h.accountFromCtx(ctx))
	})

	t.Run("query", func(t *testing.T) {
		t.Parallel()
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		req.SetRequestURI("/x?account=APP")
		ctx := &fasthttp.RequestCtx{}
		ctx.Init(req, nil, nil)
		assert.Equal(t, "APP", h.accountFromCtx(ctx))
	})

	t.Run("routeParam", func(t *testing.T) {
		t.Parallel()
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		req.SetRequestURI("/x")
		ctx := &fasthttp.RequestCtx{}
		ctx.Init(req, nil, nil)
		ctx.SetUserValue("account", "SYS")
		assert.Equal(t, "SYS", h.accountFromCtx(ctx))
	})
}
