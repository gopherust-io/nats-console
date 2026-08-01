package httpstatus_test

import (
	"errors"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
)

func TestWriteDataEnvelope(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, map[string]string{"id": "1"})

	var body map[string]any
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1", data["id"])
	assert.NotContains(t, body, "error")
	assert.NotContains(t, body, "meta")
}

func TestWriteDataMetaEnvelope(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, []string{"a", "b"}, &httpstatus.Meta{
		Total: 2, Offset: 0, Limit: 50,
	})

	var body struct {
		Data []string        `json:"data"`
		Meta httpstatus.Meta `json:"meta"`
	}
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	assert.Equal(t, []string{"a", "b"}, body.Data)
	assert.Equal(t, 2, body.Meta.Total)
	assert.Equal(t, 0, body.Meta.Offset)
	assert.Equal(t, 50, body.Meta.Limit)
}

func TestWriteDataMetaEmptyList(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, []string{}, &httpstatus.Meta{Total: 0})

	var body map[string]any
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	data, ok := body["data"].([]any)
	require.True(t, ok)
	assert.Empty(t, data)
}

func TestWriteErrorNested(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	httpstatus.WriteError(ctx, fasthttp.StatusNotFound, errors.New("stream missing"))

	var body struct {
		Error httpstatus.ErrorBody `json:"error"`
	}
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	assert.Equal(t, fasthttp.StatusNotFound, ctx.Response.StatusCode())
	assert.Equal(t, "stream missing", body.Error.Message)
	assert.Equal(t, httpstatus.CodeNotFound, body.Error.Code)
}

func TestWriteErrorBodyExtras(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	httpstatus.WriteErrorBody(ctx, fasthttp.StatusTooManyRequests, httpstatus.ErrorBody{
		Message:           "slow down",
		Code:              httpstatus.CodeRateLimit,
		Retryable:         true,
		RetryAfterSeconds: 30,
	})

	var body struct {
		Error httpstatus.ErrorBody `json:"error"`
	}
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	assert.True(t, body.Error.Retryable)
	assert.Equal(t, 30, body.Error.RetryAfterSeconds)
}

func TestCodeFromStatusMappings(t *testing.T) {
	assert.Equal(t, httpstatus.CodeGone, httpstatus.CodeFromStatus(fasthttp.StatusGone))
	assert.Equal(t, httpstatus.CodeNotFound, httpstatus.CodeFromStatus(fasthttp.StatusNotFound))
	assert.Equal(t, httpstatus.CodeConflict, httpstatus.CodeFromStatus(fasthttp.StatusConflict))
	assert.Equal(t, httpstatus.CodeValidation, httpstatus.CodeFromStatus(fasthttp.StatusBadRequest))
}

func TestWriteErrorMessageCSRFAndGone(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	httpstatus.WriteErrorMessage(ctx, fasthttp.StatusForbidden, httpstatus.CodeCSRFInvalid, "csrf token missing or invalid")
	var body struct {
		Error httpstatus.ErrorBody `json:"error"`
	}
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	assert.Equal(t, httpstatus.CodeCSRFInvalid, body.Error.Code)
	assert.Equal(t, "csrf token missing or invalid", body.Error.Message)

	ctx = &fasthttp.RequestCtx{}
	httpstatus.WriteErrorMessage(ctx, fasthttp.StatusGone, httpstatus.CodeGone, "invite expired or already used")
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	assert.Equal(t, fasthttp.StatusGone, ctx.Response.StatusCode())
	assert.Equal(t, httpstatus.CodeGone, body.Error.Code)
}
