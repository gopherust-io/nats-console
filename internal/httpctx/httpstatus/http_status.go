package httpstatus

import (
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const jsonContentType = "application/json"

type Meta struct {
	Total  int `json:"total"`
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
}

// goalign:ignore // JSON DTO; trailing bool padding is unavoidable
type ErrorBody struct {
	Message           string `json:"message"`
	Code              string `json:"code"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`
	Retryable         bool   `json:"retryable,omitempty"`
}

// Envelope is the universal JSON response shape for /api endpoints.
type Envelope struct {
	Data  any        `json:"data,omitempty"`
	Meta  *Meta      `json:"meta,omitempty"`
	Error *ErrorBody `json:"error,omitempty"`
}

func WriteError(ctx *fasthttp.RequestCtx, status int, err error) {
	WriteErrorCode(ctx, status, CodeFromStatus(status), err)
}

func WriteErrorCode(ctx *fasthttp.RequestCtx, status int, code string, err error) {
	msg := "request failed"
	if err != nil {
		if text := err.Error(); !strings.IsEmpty(text) {
			msg = text
		}
	}
	WriteErrorMessage(ctx, status, code, msg)
}

func CodeFromStatus(status int) string {
	switch status {
	case fasthttp.StatusNotFound:
		return CodeNotFound
	case fasthttp.StatusForbidden:
		return CodeForbidden
	case fasthttp.StatusUnauthorized:
		return CodeUnauthorized
	case fasthttp.StatusBadRequest, fasthttp.StatusUnprocessableEntity:
		return CodeValidation
	case fasthttp.StatusConflict:
		return CodeConflict
	case fasthttp.StatusGone:
		return CodeGone
	case fasthttp.StatusRequestTimeout, fasthttp.StatusGatewayTimeout:
		return CodeTimeout
	case fasthttp.StatusServiceUnavailable, fasthttp.StatusBadGateway:
		return CodeUnavailable
	case fasthttp.StatusTooManyRequests:
		return CodeRateLimit
	default:
		if status >= fasthttp.StatusBadRequest && status < fasthttp.StatusInternalServerError {
			return CodeValidation
		}
		return CodeInternal
	}
}

func WriteErrorMessage(ctx *fasthttp.RequestCtx, status int, code, message string) {
	if strings.IsEmpty(message) {
		message = "request failed"
	}
	if strings.IsEmpty(code) {
		code = CodeFromStatus(status)
	}
	WriteErrorBody(ctx, status, ErrorBody{Message: message, Code: code})
}

func WriteErrorBody(ctx *fasthttp.RequestCtx, status int, body ErrorBody) {
	if strings.IsEmpty(body.Message) {
		body.Message = "request failed"
	}
	if strings.IsEmpty(body.Code) {
		body.Code = CodeFromStatus(status)
	}
	writeEnvelope(ctx, status, Envelope{Error: &body}, "")
}

func WriteData(ctx *fasthttp.RequestCtx, status int, data any) {
	WriteDataMeta(ctx, status, data, nil)
}

func WriteDataMeta(ctx *fasthttp.RequestCtx, status int, data any, meta *Meta) {
	writeEnvelope(ctx, status, Envelope{Data: data, Meta: meta}, "")
}

func WriteDataWithETag(ctx *fasthttp.RequestCtx, status int, data any, etag string) {
	WriteDataMetaWithETag(ctx, status, data, nil, etag)
}

func WriteDataMetaWithETag(ctx *fasthttp.RequestCtx, status int, data any, meta *Meta, etag string) {
	writeEnvelope(ctx, status, Envelope{Data: data, Meta: meta}, etag)
}

func CheckIfNoneMatch(ctx *fasthttp.RequestCtx, etag string) bool {
	if strings.IsEmpty(etag) {
		return false
	}
	inm := strings.BytesToString(ctx.Request.Header.Peek("If-None-Match"))
	return !strings.IsEmpty(inm) && inm == etag
}

// WriteRawJSONWithETag writes raw JSON bytes without an envelope (monitoring proxy).
func WriteRawJSONWithETag(ctx *fasthttp.RequestCtx, data []byte, etag string) {
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType(jsonContentType)
	if !strings.IsEmpty(etag) {
		ctx.Response.Header.Set("ETag", etag)
		ctx.Response.Header.Set("Cache-Control", "private, max-age=0, must-revalidate")
	}
	ctx.SetBody(data)
}

// WriteJSON is kept for rare non-envelope passthrough; prefer WriteData for API responses.
func WriteJSON(ctx *fasthttp.RequestCtx, status int, v any) {
	WriteJSONWithETag(ctx, status, v, "")
}

// WriteJSONWithETag marshals v as-is (no envelope). Prefer WriteDataWithETag for API responses.
func WriteJSONWithETag(ctx *fasthttp.RequestCtx, status int, v any, etag string) {
	data, err := serializer.Marshal(v)
	if err != nil {
		WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	ctx.SetStatusCode(status)
	ctx.SetContentType(jsonContentType)
	if !strings.IsEmpty(etag) {
		ctx.Response.Header.Set("ETag", etag)
		ctx.Response.Header.Set("Cache-Control", "private, max-age=0, must-revalidate")
	}
	ctx.SetBody(data)
}

func WriteForbidden(ctx *fasthttp.RequestCtx) {
	WriteErrorMessage(ctx, fasthttp.StatusForbidden, CodeForbidden, "forbidden")
}

func WriteUnauthorized(ctx *fasthttp.RequestCtx) {
	WriteErrorMessage(ctx, fasthttp.StatusUnauthorized, CodeUnauthorized, "unauthorized")
}

func writeEnvelope(ctx *fasthttp.RequestCtx, status int, env Envelope, etag string) {
	data, err := serializer.Marshal(env)
	if err != nil {
		// Avoid recursion if marshal of an error envelope fails.
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.SetContentType(jsonContentType)
		ctx.SetBodyString(`{"error":{"message":"response encode failed","code":"internal"}}`)
		return
	}
	ctx.SetStatusCode(status)
	ctx.SetContentType(jsonContentType)
	if !strings.IsEmpty(etag) {
		ctx.Response.Header.Set("ETag", etag)
		ctx.Response.Header.Set("Cache-Control", "private, max-age=0, must-revalidate")
	}
	ctx.SetBody(data)
}
