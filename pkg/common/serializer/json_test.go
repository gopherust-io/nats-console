package serializer_test

import (
	"errors"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestWriteJSONUsesCamelCaseTags(t *testing.T) {
	cluster := domain.Cluster{
		ID:        "id-1",
		Name:      "default",
		NATSURL:   "nats://localhost:4222",
		IsDefault: true,
	}
	raw, err := sonic.Marshal(cluster)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, sonic.Unmarshal(raw, &decoded))
	assert.Equal(t, "nats://localhost:4222", decoded["natsUrl"], "expected natsUrl")
	assert.NotContains(t, decoded, "nats_url", "unexpected snake_case key in response")
}

func TestUnmarshalRequestAcceptsCamelCase(t *testing.T) {
	body := commonstrings.StringToBytes(`{"natsUrl":"nats://localhost:4222","isDefault":true}`)
	var req struct {
		NATSURL   string `json:"natsUrl"`
		IsDefault bool   `json:"isDefault"`
	}
	require.NoError(t, serializer.Unmarshal(body, &req))
	assert.Equal(t, "nats://localhost:4222", req.NATSURL)
	assert.True(t, req.IsDefault)
}

func TestStreamInfoDTOUsesCamelCase(t *testing.T) {
	info := domain.StreamInfo{
		Config: domain.StreamConfigDTO{
			Name:      "ORDERS",
			Retention: "limits",
			Storage:   "file",
		},
		State: domain.StreamStateDTO{
			FirstSeq:      1,
			LastSeq:       10,
			ConsumerCount: 2,
		},
	}
	raw, err := sonic.Marshal(info)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, sonic.Unmarshal(raw, &decoded))
	state := decoded["state"].(map[string]any)
	assert.Equal(t, float64(1), state["firstSeq"])
	assert.NotContains(t, state, "first_seq")
}

func TestWriteErrorIncludesCodeFromStatus(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	httpstatus.WriteError(ctx, fasthttp.StatusNotFound, errors.New("stream missing"))

	assert.Equal(t, fasthttp.StatusNotFound, ctx.Response.StatusCode())
	var body map[string]any
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	errObj := body["error"].(map[string]any)
	assert.Equal(t, "stream missing", errObj["message"])
	assert.Equal(t, httpstatus.CodeNotFound, errObj["code"])
}

func TestWriteErrorCodeUsesExplicitCode(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	httpstatus.WriteErrorCode(ctx, fasthttp.StatusConflict, httpstatus.CodeConflict, errors.New("already exists"))

	assert.Equal(t, fasthttp.StatusConflict, ctx.Response.StatusCode())
	var body map[string]any
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	errObj := body["error"].(map[string]any)
	assert.Equal(t, "already exists", errObj["message"])
	assert.Equal(t, httpstatus.CodeConflict, errObj["code"])
}

func TestWriteErrorMessage(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	httpstatus.WriteErrorMessage(ctx, fasthttp.StatusUnauthorized, httpstatus.CodeUnauthorized, "unauthorized")

	assert.Equal(t, fasthttp.StatusUnauthorized, ctx.Response.StatusCode())
	var body map[string]any
	require.NoError(t, sonic.Unmarshal(ctx.Response.Body(), &body))
	errObj := body["error"].(map[string]any)
	assert.Equal(t, "unauthorized", errObj["message"])
	assert.Equal(t, httpstatus.CodeUnauthorized, errObj["code"])
}

func TestMarshalNilSliceAsEmptyArray(t *testing.T) {
	raw, err := serializer.Marshal(struct {
		Items []string `json:"items"`
	}{Items: nil})
	require.NoError(t, err)
	assert.JSONEq(t, `{"items":[]}`, string(raw))
}

func TestMarshalOmitemptyNilMapStillOmitted(t *testing.T) {
	raw, err := serializer.Marshal(struct {
		Headers map[string]string `json:"headers,omitempty"`
		Name    string            `json:"name"`
	}{Name: "x"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"x"}`, string(raw))
}

func TestMarshalRequestReplySnapshotOptionals(t *testing.T) {
	lat := 12.5
	ok := true
	snap := domain.RequestReplySnapshot{
		Patterns: []domain.RequestReplyPattern{{
			Subject:        "orders.status",
			ProbeLatencyMs: &lat,
			ProbeOk:        &ok,
		}},
		Connections: []domain.RequestReplyConnection{},
	}
	raw, err := serializer.Marshal(snap)
	require.NoError(t, err)
	var decoded domain.RequestReplySnapshot
	require.NoError(t, serializer.Unmarshal(raw, &decoded))
	require.Len(t, decoded.Patterns, 1)
	require.NotNil(t, decoded.Patterns[0].ProbeLatencyMs)
	assert.InDelta(t, 12.5, *decoded.Patterns[0].ProbeLatencyMs, 0.001)
	require.NotNil(t, decoded.Patterns[0].ProbeOk)
	assert.True(t, *decoded.Patterns[0].ProbeOk)
	assert.Nil(t, decoded.MedianRttMs)
}

func TestCodeFromStatus(t *testing.T) {
	assert.Equal(t, httpstatus.CodeForbidden, httpstatus.CodeFromStatus(fasthttp.StatusForbidden))
	assert.Equal(t, httpstatus.CodeTimeout, httpstatus.CodeFromStatus(fasthttp.StatusGatewayTimeout))
	assert.Equal(t, httpstatus.CodeUnavailable, httpstatus.CodeFromStatus(fasthttp.StatusBadGateway))
	assert.Equal(t, httpstatus.CodeRateLimit, httpstatus.CodeFromStatus(fasthttp.StatusTooManyRequests))
	assert.Equal(t, httpstatus.CodeInternal, httpstatus.CodeFromStatus(fasthttp.StatusInternalServerError))
	assert.Equal(t, httpstatus.CodeGone, httpstatus.CodeFromStatus(fasthttp.StatusGone))
}
