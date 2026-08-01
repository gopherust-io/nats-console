package middleware

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/config"
)

func TestApplyRecoverReturnsInternalError(t *testing.T) {
	t.Parallel()
	mw := New(config.Config{}, nil, nil)
	handler := mw.ApplyRecover(func(_ *fasthttp.RequestCtx) {
		panic("boom")
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/v1/clusters")
	require.NotPanics(t, func() { handler(ctx) })
	assert.Equal(t, fasthttp.StatusInternalServerError, ctx.Response.StatusCode())

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(ctx.Response.Body(), &body))
	assert.Equal(t, "internal", body.Error.Code)
	assert.Equal(t, "internal error", body.Error.Message)
}
