package httpctx

import (
	"context"

	"github.com/valyala/fasthttp"
)

const contextUserValueKey = "context"

// FromRequest returns the context stored on the fasthttp request, or Background.
func FromRequest(ctx *fasthttp.RequestCtx) context.Context {
	if ctx == nil {
		return context.Background()
	}
	if c, ok := ctx.UserValue(contextUserValueKey).(context.Context); ok && c != nil {
		return c
	}
	return context.Background()
}
