package apikit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/config"
)

func TestParsePaginationParams(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Pagination: config.PaginationConfig{
			DefaultLimit: 100,
			MaxLimit:     500,
		},
	}

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		req.SetRequestURI("/x")
		ctx := &fasthttp.RequestCtx{}
		ctx.Init(req, nil, nil)

		offset, limit := ParsePaginationParams(ctx, cfg)
		assert.Equal(t, 0, offset)
		assert.Equal(t, 100, limit)
	})

	t.Run("explicit and clamp", func(t *testing.T) {
		t.Parallel()
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		req.SetRequestURI("/x?offset=10&limit=9999")
		ctx := &fasthttp.RequestCtx{}
		ctx.Init(req, nil, nil)

		offset, limit := ParsePaginationParams(ctx, cfg)
		assert.Equal(t, 10, offset)
		assert.Equal(t, 500, limit)
	})

	t.Run("negative offset", func(t *testing.T) {
		t.Parallel()
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		req.SetRequestURI("/x?offset=-5&limit=25")
		ctx := &fasthttp.RequestCtx{}
		ctx.Init(req, nil, nil)

		offset, limit := ParsePaginationParams(ctx, cfg)
		assert.Equal(t, 0, offset)
		assert.Equal(t, 25, limit)
	})
}

func TestKeysAndObjectsPage(t *testing.T) {
	t.Parallel()

	keys := KeysPage(nil, 0, 0, 100)
	assert.Empty(t, keys.Data)
	assert.Equal(t, 0, keys.Meta.Total)
	assert.Equal(t, 100, keys.Meta.Limit)

	objects := ObjectsPage([]string{"a.txt"}, 1, 5, 50)
	assert.Equal(t, []string{"a.txt"}, objects.Data)
	assert.Equal(t, 1, objects.Meta.Total)
	assert.Equal(t, 5, objects.Meta.Offset)
	assert.Equal(t, 50, objects.Meta.Limit)
}
