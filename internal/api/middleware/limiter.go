package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/ipset"
)

func (mw *MwHandler) VerifyAuthRateLimit(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		path := requestPath(ctx)
		if path != pathPrefixAuthLogin {
			next(ctx)
			return
		}

		trustedProxies := ipset.ParseTrustedProxies(mw.cfg.TrustedProxyList())
		if !mw.authLimiter.allow(httpctx.ClientIP(ctx, trustedProxies)) {
			retryAfter := max(int(mw.cfg.Auth.RateLimitWindow.Seconds()), 1)
			ctx.Response.Header.Set("Retry-After", strconv.Itoa(retryAfter))
			httpstatus.WriteErrorBody(ctx, fasthttp.StatusTooManyRequests, httpstatus.ErrorBody{
				Message:           "rate limit exceeded",
				Code:              httpstatus.CodeRateLimit,
				Retryable:         true,
				RetryAfterSeconds: retryAfter,
			})
			return
		}
		next(ctx)
	}
}

const (
	ipRateLimiterPurgeEvery   = 256
	ipRateLimiterMaxStaleKeys = 512
)

type ipRateLimiter struct {
	events     map[string][]time.Time
	limit      int
	window     time.Duration
	allowCount int
	mu         sync.Mutex
}

func newIPRateLimiter(limit int, window time.Duration) *ipRateLimiter {
	return &ipRateLimiter{
		events: make(map[string][]time.Time),
		limit:  limit,
		window: window,
	}
}

func (rl *ipRateLimiter) allow(key string) bool {
	if rl.limit <= 0 {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)
	times := rl.events[key]
	n := 0
	for _, t := range times {
		if t.After(cutoff) {
			times[n] = t
			n++
		}
	}
	times = times[:n]
	if len(times) >= rl.limit {
		rl.events[key] = times
		return false
	}
	rl.events[key] = append(times, now)

	rl.allowCount++
	if rl.allowCount%ipRateLimiterPurgeEvery == 0 || len(rl.events) > ipRateLimiterMaxStaleKeys {
		rl.purgeStaleLocked(cutoff)
	}
	return true
}

func (rl *ipRateLimiter) purgeStaleLocked(cutoff time.Time) {
	for key, times := range rl.events {
		n := 0
		for _, t := range times {
			if t.After(cutoff) {
				times[n] = t
				n++
			}
		}
		if n == 0 {
			delete(rl.events, key)
			continue
		}
		rl.events[key] = times[:n]
	}
}
