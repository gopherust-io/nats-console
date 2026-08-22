package middleware

import (
	"bytes"
	"errors"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const (
	// minResponseCompressBytes: compress only when the body is strictly larger than 32 KB
	minResponseCompressBytes = 32 << 10
	defaultMaxBytes          = 1 << 20
)

var (
	strEventStream = commonstrings.StringToBytes("text/event-stream")
	strUpgrade     = commonstrings.StringToBytes("Upgrade")
	strLiveWS      = "/live/ws"
)

// ApplyResponseCompression compresses response bodies (br → gzip → deflate/zstd)
// at best-speed levels. Skips WebSocket upgrades and SSE streams.
func (mw *MwHandler) ApplyResponseCompression(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	if !mw.cfg.HTTP.ResponseCompression {
		return next
	}
	compress := fasthttp.CompressHandlerBrotliLevel(
		func(*fasthttp.RequestCtx) {},
		fasthttp.CompressBrotliBestSpeed,
		fasthttp.CompressBestSpeed,
	)
	return func(ctx *fasthttp.RequestCtx) {
		if skipResponseCompressionBefore(ctx) {
			next(ctx)
			return
		}
		next(ctx)
		if skipResponseCompressionAfter(ctx) {
			return
		}
		compress(ctx)
	}
}

// DecompressRequestBody expands gzip/br/deflate/zstd request bodies before
// handlers and size checks see them. The uncompressed size is capped by
// HTTP.MaxRequestBodySize.
func (mw *MwHandler) DecompressRequestBody(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		enc := commonstrings.BytesToString(ctx.Request.Header.ContentEncoding())
		if commonstrings.IsEmpty(enc) {
			next(ctx)
			return
		}

		maxBytes := mw.cfg.HTTP.MaxRequestBodySize
		if maxBytes <= 0 {
			maxBytes = defaultMaxBytes
		}
		if cl := ctx.Request.Header.ContentLength(); cl > 0 && cl > maxBytes {
			httpstatus.WriteErrorMessage(ctx, fasthttp.StatusRequestEntityTooLarge, httpstatus.CodeValidation, "request body too large")
			return
		}
		if len(ctx.Request.Body()) > maxBytes {
			httpstatus.WriteErrorMessage(ctx, fasthttp.StatusRequestEntityTooLarge, httpstatus.CodeValidation, "request body too large")
			return
		}

		body, err := ctx.Request.BodyUncompressedWithLimit(maxBytes)
		if err != nil {
			if errors.Is(err, fasthttp.ErrBodyTooLarge) {
				httpstatus.WriteErrorMessage(ctx, fasthttp.StatusRequestEntityTooLarge, httpstatus.CodeValidation, "request body too large")
				return
			}
			if errors.Is(err, fasthttp.ErrContentEncodingUnsupported) {
				httpstatus.WriteErrorMessage(ctx, fasthttp.StatusBadRequest, httpstatus.CodeValidation, "unsupported content encoding")
				return
			}
			httpstatus.WriteErrorMessage(ctx, fasthttp.StatusBadRequest, httpstatus.CodeValidation, "invalid compressed request body")
			return
		}

		ctx.Request.SetBody(body)
		ctx.Request.Header.Del(fasthttp.HeaderContentEncoding)
		ctx.Request.Header.SetContentLength(len(body))
		next(ctx)
	}
}

func skipResponseCompressionBefore(ctx *fasthttp.RequestCtx) bool {
	if ctx.Request.Header.ConnectionUpgrade() {
		return true
	}
	if bytes.Contains(ctx.Request.Header.Peek(fasthttp.HeaderConnection), strUpgrade) {
		return true
	}
	path := requestPath(ctx)
	return strings.HasSuffix(path, strLiveWS)
}

func skipResponseCompressionAfter(ctx *fasthttp.RequestCtx) bool {
	status := ctx.Response.StatusCode()
	if status == fasthttp.StatusNoContent || status == fasthttp.StatusNotModified {
		return true
	}
	if !commonstrings.IsEmpty(commonstrings.BytesToString(ctx.Response.Header.ContentEncoding())) {
		return true
	}
	if bytes.HasPrefix(ctx.Response.Header.ContentType(), strEventStream) {
		return true
	}
	// Do not call Body() on streams — it would consume the reader.
	if ctx.Response.IsBodyStream() {
		return true
	}
	n := len(ctx.Response.Body())
	return n == 0 || n <= minResponseCompressBytes
}
