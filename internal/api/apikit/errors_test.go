package apikit

import (
	"errors"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
)

func TestWriteAPIError(t *testing.T) {
	cases := []struct {
		err     error
		name    string
		code    string
		message string
		status  int
	}{
		{name: "not found", err: domain.ErrNotFound, status: fasthttp.StatusNotFound, code: httpstatus.CodeNotFound, message: domain.ErrNotFound.Error()},
		{name: "alert not found", err: domain.ErrAlertNotFound, status: fasthttp.StatusNotFound, code: httpstatus.CodeNotFound, message: domain.ErrAlertNotFound.Error()},
		{name: "catalog not found", err: domain.ErrEventCatalogEntryNotFound, status: fasthttp.StatusNotFound, code: httpstatus.CodeNotFound, message: domain.ErrEventCatalogEntryNotFound.Error()},
		{name: "forbidden", err: domain.ErrForbidden, status: fasthttp.StatusForbidden, code: httpstatus.CodeForbidden, message: domain.ErrForbidden.Error()},
		{name: "root protected", err: domain.ErrRootProtected, status: fasthttp.StatusForbidden, code: httpstatus.CodeForbidden, message: domain.ErrRootProtected.Error()},
		{name: "cannot escalate", err: domain.ErrCannotEscalate, status: fasthttp.StatusForbidden, code: httpstatus.CodeForbidden, message: domain.ErrCannotEscalate.Error()},
		{name: "validation", err: domain.ErrInvalidInput, status: fasthttp.StatusBadRequest, code: httpstatus.CodeValidation, message: domain.ErrInvalidInput.Error()},
		{name: "conflict", err: domain.ErrConflict, status: fasthttp.StatusConflict, code: httpstatus.CodeConflict, message: domain.ErrConflict.Error()},
		{name: "root exists", err: domain.ErrRootExists, status: fasthttp.StatusConflict, code: httpstatus.CodeConflict, message: domain.ErrRootExists.Error()},
		{name: "signing protected", err: domain.ErrSigningGroupProtected, status: fasthttp.StatusConflict, code: httpstatus.CodeConflict, message: domain.ErrSigningGroupProtected.Error()},
		{name: "signing in use", err: domain.ErrSigningGroupInUse, status: fasthttp.StatusConflict, code: httpstatus.CodeConflict, message: domain.ErrSigningGroupInUse.Error()},
		{name: "internal sanitized", err: errors.New("boom sql detail"), status: fasthttp.StatusInternalServerError, code: httpstatus.CodeInternal, message: "internal error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &fasthttp.RequestCtx{}
			WriteAPIError(ctx, tc.err)
			assert.Equal(t, tc.status, ctx.Response.StatusCode())
			var body map[string]any
			require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
			errObj := body["error"].(map[string]any)
			assert.Equal(t, tc.code, errObj["code"])
			assert.Equal(t, tc.message, errObj["message"])
		})
	}
}

func TestWriteNATSErrorStableMessages(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	WriteNATSError(ctx, fasthttp.StatusBadGateway, errors.New("nats: connection refused raw detail"))
	assert.Equal(t, fasthttp.StatusBadGateway, ctx.Response.StatusCode())
	var body map[string]any
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	errObj := body["error"].(map[string]any)
	assert.Equal(t, httpstatus.CodeUnavailable, errObj["code"])
	assert.Equal(t, "NATS is unavailable", errObj["message"])
	assert.Equal(t, true, errObj["retryable"])
	assert.EqualValues(t, natsRetryAfterSeconds, errObj["retryAfterSeconds"])

	ctx = &fasthttp.RequestCtx{}
	WriteNATSError(ctx, fasthttp.StatusGatewayTimeout, errors.New("deadline exceeded"))
	assert.Equal(t, fasthttp.StatusGatewayTimeout, ctx.Response.StatusCode())
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	errObj = body["error"].(map[string]any)
	assert.Equal(t, httpstatus.CodeTimeout, errObj["code"])
	assert.Equal(t, "NATS request timed out", errObj["message"])
	assert.Equal(t, true, errObj["retryable"])

	ctx = &fasthttp.RequestCtx{}
	WriteNATSError(ctx, fasthttp.StatusNotFound, errors.New("stream not found"))
	assert.Equal(t, fasthttp.StatusNotFound, ctx.Response.StatusCode())
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	errObj = body["error"].(map[string]any)
	assert.Equal(t, httpstatus.CodeNotFound, errObj["code"])
	assert.Equal(t, domain.ErrNotFound.Error(), errObj["message"])
	_, hasRetry := errObj["retryable"]
	assert.False(t, hasRetry)

	ctx = &fasthttp.RequestCtx{}
	WriteNATSError(ctx, fasthttp.StatusBadRequest, errors.New("invalid subject"))
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	errObj = body["error"].(map[string]any)
	assert.Equal(t, "invalid subject", errObj["message"])
}

func TestWriteAPIErrorNil(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	WriteAPIError(ctx, nil)
	assert.Equal(t, fasthttp.StatusInternalServerError, ctx.Response.StatusCode())
	var body map[string]any
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	errObj := body["error"].(map[string]any)
	assert.Equal(t, httpstatus.CodeInternal, errObj["code"])
	assert.Equal(t, "internal error", errObj["message"])
}

func TestCodeFromStatusGone(t *testing.T) {
	assert.Equal(t, httpstatus.CodeGone, httpstatus.CodeFromStatus(fasthttp.StatusGone))
}
