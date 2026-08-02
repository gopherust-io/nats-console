package middleware

import (
	"context"
	"strings"

	"github.com/valyala/fasthttp"
)

func (mw *MwHandler) ApplyAITimeout(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		path := requestPath(ctx)
		if isLongRunningProfilePath(path) {
			next(ctx)
			return
		}
		reqTimeout := mw.cfg.RequestTimeout
		if mw.cfg.AI.RequestTimeout > 0 && isAITimeoutPath(path) {
			reqTimeout = mw.cfg.AI.RequestTimeout
		}

		ctxTimeout, cancel := context.WithTimeout(context.Background(), reqTimeout)
		defer cancel()
		ctx.SetUserValue(ctxKey, ctxTimeout)
		next(ctx)
	}
}

func isAITimeoutPath(path string) bool {
	return strings.HasPrefix(path, pathPrefixAssistant) ||
		strings.Contains(path, "/assistant/chat") ||
		strings.Contains(path, "/architecture-review/ask") ||
		strings.Contains(path, "/architecture-refactor/ask") ||
		strings.Contains(path, "/architecture-score/ask") ||
		strings.Contains(path, "/architecture-export") ||
		strings.Contains(path, "/chaos-story/generate") ||
		strings.Contains(path, "/hidden-bottlenecks/ask")
}
