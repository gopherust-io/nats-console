package middleware

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/config"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func largeJSON(n int) []byte {
	var b strings.Builder
	b.Grow(n + 16)
	b.WriteString(`{"data":"`)
	for b.Len() < n {
		b.WriteString("abcdefghijklmnopqrstuvwxyz0123456789")
	}
	b.WriteString(`"}`)
	// Copy: StringToBytes would alias Builder storage unsafely after return.
	return append([]byte(nil), commonstrings.StringToBytes(b.String())...)
}

func TestApplyResponseCompressionGzip(t *testing.T) {
	t.Parallel()
	mw := New(config.Config{HTTP: config.HTTPConfig{ResponseCompression: true}}, nil, nil)
	payload := largeJSON(40 << 10)
	h := mw.ApplyResponseCompression(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetContentType("application/json")
		ctx.SetBody(payload)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/v1/streams")
	ctx.Request.Header.Set(fasthttp.HeaderAcceptEncoding, "gzip")
	h(ctx)

	require.Equal(t, "gzip", commonstrings.BytesToString(ctx.Response.Header.ContentEncoding()))
	assert.Contains(t, commonstrings.BytesToString(ctx.Response.Header.Peek(fasthttp.HeaderVary)), "Accept-Encoding")
	assert.Less(t, len(ctx.Response.Body()), len(payload))
	raw, err := ctx.Response.BodyGunzip()
	require.NoError(t, err)
	assert.Equal(t, payload, raw)
}

func TestApplyResponseCompressionBrotli(t *testing.T) {
	t.Parallel()
	mw := New(config.Config{HTTP: config.HTTPConfig{ResponseCompression: true}}, nil, nil)
	payload := largeJSON(40 << 10)
	h := mw.ApplyResponseCompression(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetContentType("application/json")
		ctx.SetBody(payload)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/v1/streams")
	ctx.Request.Header.Set(fasthttp.HeaderAcceptEncoding, "br, gzip")
	h(ctx)

	require.Equal(t, "br", commonstrings.BytesToString(ctx.Response.Header.ContentEncoding()))
	raw, err := ctx.Response.BodyUnbrotli()
	require.NoError(t, err)
	assert.Equal(t, payload, raw)
}

func TestApplyResponseCompressionDisabled(t *testing.T) {
	t.Parallel()
	mw := New(config.Config{HTTP: config.HTTPConfig{ResponseCompression: false}}, nil, nil)
	payload := largeJSON(40 << 10)
	h := mw.ApplyResponseCompression(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetContentType("application/json")
		ctx.SetBody(payload)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set(fasthttp.HeaderAcceptEncoding, "gzip")
	h(ctx)

	assert.Empty(t, ctx.Response.Header.ContentEncoding())
	assert.Equal(t, payload, ctx.Response.Body())
}

func TestApplyResponseCompressionSkipsSmallBody(t *testing.T) {
	t.Parallel()
	mw := New(config.Config{HTTP: config.HTTPConfig{ResponseCompression: true}}, nil, nil)
	// Exactly 32 KiB must not compress (threshold is strictly greater than 32 KiB).
	payload := append([]byte(nil), commonstrings.StringToBytes(strings.Repeat("a", minResponseCompressBytes))...)
	h := mw.ApplyResponseCompression(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetContentType("application/json")
		ctx.SetBody(payload)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set(fasthttp.HeaderAcceptEncoding, "gzip")
	h(ctx)

	assert.Empty(t, ctx.Response.Header.ContentEncoding())
	assert.Equal(t, payload, ctx.Response.Body())
}

func TestApplyResponseCompressionSkipsSSE(t *testing.T) {
	t.Parallel()
	mw := New(config.Config{HTTP: config.HTTPConfig{ResponseCompression: true}}, nil, nil)
	payload := largeJSON(40 << 10)
	h := mw.ApplyResponseCompression(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetContentType("text/event-stream")
		ctx.SetBody(payload)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set(fasthttp.HeaderAcceptEncoding, "gzip")
	h(ctx)

	assert.Empty(t, ctx.Response.Header.ContentEncoding())
	assert.Equal(t, payload, ctx.Response.Body())
}

func TestApplyResponseCompressionSkipsWebSocketUpgrade(t *testing.T) {
	t.Parallel()
	mw := New(config.Config{HTTP: config.HTTPConfig{ResponseCompression: true}}, nil, nil)
	payload := largeJSON(40 << 10)
	h := mw.ApplyResponseCompression(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusSwitchingProtocols)
		ctx.SetContentType("application/json")
		ctx.SetBody(payload)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/v1/clusters/x/live/ws")
	ctx.Request.Header.Set(fasthttp.HeaderConnection, "Upgrade")
	ctx.Request.Header.Set(fasthttp.HeaderUpgrade, "websocket")
	ctx.Request.Header.Set(fasthttp.HeaderAcceptEncoding, "gzip")
	h(ctx)

	assert.Empty(t, ctx.Response.Header.ContentEncoding())
}

func TestApplyResponseCompressionSkipsLiveWSPath(t *testing.T) {
	t.Parallel()
	mw := New(config.Config{HTTP: config.HTTPConfig{ResponseCompression: true}}, nil, nil)
	payload := largeJSON(40 << 10)
	h := mw.ApplyResponseCompression(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetContentType("application/json")
		ctx.SetBody(payload)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/v1/clusters/abc/live/ws")
	ctx.Request.Header.Set(fasthttp.HeaderAcceptEncoding, "gzip")
	h(ctx)

	assert.Empty(t, ctx.Response.Header.ContentEncoding())
}

func TestDecompressRequestBodyGzip(t *testing.T) {
	t.Parallel()
	mw := New(config.Config{HTTP: config.HTTPConfig{MaxRequestBodySize: 1 << 20}}, nil, nil)
	plain := commonstrings.StringToBytes(`{"name":"orders","subjects":["orders.>"]}`)
	compressed := fasthttp.AppendGzipBytes(nil, plain)

	var seen []byte
	h := mw.DecompressRequestBody(func(ctx *fasthttp.RequestCtx) {
		seen = append([]byte(nil), ctx.Request.Body()...)
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/v1/clusters/x/streams")
	ctx.Request.Header.SetContentEncoding("gzip")
	ctx.Request.SetBody(compressed)
	h(ctx)

	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	assert.Equal(t, plain, seen)
	assert.Empty(t, ctx.Request.Header.ContentEncoding())
}

func TestDecompressRequestBodyOversizeUncompressed(t *testing.T) {
	t.Parallel()
	mw := New(config.Config{HTTP: config.HTTPConfig{MaxRequestBodySize: 64}}, nil, nil)
	plain := bytes.Repeat(commonstrings.StringToBytes("a"), 256)
	compressed := fasthttp.AppendGzipBytes(nil, plain)

	h := mw.DecompressRequestBody(func(ctx *fasthttp.RequestCtx) {
		t.Fatal("handler should not run")
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.Header.SetContentEncoding("gzip")
	ctx.Request.SetBody(compressed)
	h(ctx)

	require.Equal(t, fasthttp.StatusRequestEntityTooLarge, ctx.Response.StatusCode())
	assert.Contains(t, commonstrings.BytesToString(ctx.Response.Body()), "request body too large")
}

func TestDecompressRequestBodyUnsupportedEncoding(t *testing.T) {
	t.Parallel()
	mw := New(config.Config{HTTP: config.HTTPConfig{MaxRequestBodySize: 1 << 20}}, nil, nil)
	h := mw.DecompressRequestBody(func(ctx *fasthttp.RequestCtx) {
		t.Fatal("handler should not run")
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetContentEncoding("lz4")
	ctx.Request.SetBody(commonstrings.StringToBytes("nope"))
	h(ctx)

	require.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
	assert.Contains(t, commonstrings.BytesToString(ctx.Response.Body()), "unsupported content encoding")
}

func TestDecompressRequestBodyPassthrough(t *testing.T) {
	t.Parallel()
	mw := New(config.Config{HTTP: config.HTTPConfig{MaxRequestBodySize: 1 << 20}}, nil, nil)
	plain := commonstrings.StringToBytes(`{"ok":true}`)
	var seen []byte
	h := mw.DecompressRequestBody(func(ctx *fasthttp.RequestCtx) {
		seen = append([]byte(nil), ctx.Request.Body()...)
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody(plain)
	h(ctx)

	assert.Equal(t, plain, seen)
	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
}
