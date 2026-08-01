package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
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
