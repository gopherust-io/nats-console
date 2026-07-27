package httpctx

import (
	"context"

	"github.com/valyala/fasthttp"
)

const contextUserValueKey = "context"

func FromRequest(ctx *fasthttp.RequestCtx) context.Context {
	if ctx == nil {
		return context.Background()
	}
	if c, ok := ctx.UserValue(contextUserValueKey).(context.Context); ok && c != nil {
		return c
	}
	return context.Background()
}

func RouteParam(ctx *fasthttp.RequestCtx, key string) string {
	value, ok := ctx.UserValue(key).(string)
	if !ok {
		return ""
	}
	return value
}
