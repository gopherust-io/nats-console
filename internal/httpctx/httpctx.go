package httpctx

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/ipset"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
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

// ClientIP resolves the caller's IP address. X-Forwarded-For is only honored
// when the direct connection (RemoteIP) is a configured trusted proxy;
// otherwise it is attacker-controlled and ignored, since honoring it
// unconditionally lets any client spoof their audit/rate-limit identity.
func ClientIP(ctx *fasthttp.RequestCtx, trustedProxies []*net.IPNet) string {
	remote := ctx.RemoteIP()
	if len(trustedProxies) > 0 && ipset.IsTrustedProxyIP(remote, trustedProxies) {
		if raw := commonstrings.BytesToString(ctx.Request.Header.Peek("X-Forwarded-For")); !commonstrings.IsEmpty(raw) {
			candidate := strings.TrimSpace(strings.Split(raw, ",")[0])
			if host, _, err := net.SplitHostPort(candidate); err == nil {
				candidate = host
			}
			if parsed := net.ParseIP(candidate); parsed != nil {
				return parsed.String()
			}
		}
	}
	if remote == nil {
		return ""
	}
	return remote.String()
}

func SetCookie(ctx *fasthttp.RequestCtx, cookie *http.Cookie) {
	var b strings.Builder
	b.WriteString(cookie.Name)
	b.WriteString("=")
	b.WriteString(cookie.Value)
	b.WriteString("; Path=")
	b.WriteString(cookie.Path)
	if cookie.HttpOnly {
		b.WriteString("; HttpOnly")
	}
	if cookie.MaxAge != 0 {
		b.WriteString("; Max-Age=")
		b.WriteString(strconv.Itoa(cookie.MaxAge))
	}
	if cookie.Secure {
		b.WriteString("; Secure")
	}
	switch cookie.SameSite {
	case http.SameSiteLaxMode:
		b.WriteString("; SameSite=Lax")
	case http.SameSiteStrictMode:
		b.WriteString("; SameSite=Strict")
	default:
	}
	ctx.Response.Header.Set("Set-Cookie", b.String())
}
