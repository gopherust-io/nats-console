package serializer

import (
	"github.com/bytedance/sonic"
	"github.com/valyala/fasthttp"
)

const (
	jsonContentType = "application/json"
)

func marshalJSON(v any) ([]byte, error) {
	return sonic.ConfigDefault.Marshal(v)
}

func WriteJSON(ctx *fasthttp.RequestCtx, status int, v any) {
	WriteJSONWithETag(ctx, status, v, "")
}

func WriteJSONWithETag(ctx *fasthttp.RequestCtx, status int, v any, etag string) {
	data, err := marshalJSON(v)
	if err != nil {
		WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	ctx.SetStatusCode(status)
	ctx.SetContentType(jsonContentType)
	if etag != "" {
		ctx.Response.Header.Set("ETag", etag)
		ctx.Response.Header.Set("Cache-Control", "private, max-age=0, must-revalidate")
	}
	ctx.SetBody(data)
}

func WriteRawJSON(ctx *fasthttp.RequestCtx, data []byte) {
	WriteRawJSONWithETag(ctx, data, "")
}

func WriteRawJSONWithETag(ctx *fasthttp.RequestCtx, data []byte, etag string) {
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType(jsonContentType)
	if etag != "" {
		ctx.Response.Header.Set("ETag", etag)
		ctx.Response.Header.Set("Cache-Control", "private, max-age=0, must-revalidate")
	}
	ctx.SetBody(data)
}

// CheckIfNoneMatch returns true when the client already has etag (should 304).
func CheckIfNoneMatch(ctx *fasthttp.RequestCtx, etag string) bool {
	if etag == "" {
		return false
	}
	inm := string(ctx.Request.Header.Peek("If-None-Match"))
	return inm != "" && inm == etag
}

type errorResponse struct {
	Error string `json:"error"`
}

func WriteError(ctx *fasthttp.RequestCtx, status int, err error) {
	WriteJSON(ctx, status, errorResponse{Error: err.Error()})
}

func UnmarshalRequest(body []byte, v any) error {
	return sonic.Unmarshal(body, v)
}
