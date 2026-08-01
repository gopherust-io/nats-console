package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/audit"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/internal/store"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type MwHandler struct {
	authLimiter *ipRateLimiter
	authService *auth.Service
	auditWriter *audit.Writer
	cfg         config.Config
}

func New(cfg config.Config, authService *auth.Service, auditWriter *audit.Writer) *MwHandler {
	return &MwHandler{
		authLimiter: newIPRateLimiter(cfg.Auth.RateLimit, cfg.Auth.RateLimitWindow),
		cfg:         cfg,
		authService: authService,
		auditWriter: auditWriter,
	}
}

func (mw *MwHandler) ApplyRequestID(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		path := commonstrings.BytesToString(ctx.Path())
		ctx.SetUserValue(pathCtxKey, path)
		id := commonstrings.BytesToString(ctx.Request.Header.Peek(HeaderRequestID))
		if commonstrings.IsEmpty(id) {
			var b [8]byte
			_, _ = rand.Read(b[:])
			id = hex.EncodeToString(b[:])
		}
		ctx.SetUserValue(requestIDCtxKey, id)
		ctx.Response.Header.Set(HeaderRequestID, id)
		next(ctx)
	}
}

func (mw *MwHandler) ApplySecurityHeaders(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		h := &ctx.Response.Header
		h.Set(HeaderContentType, "nosniff")
		h.Set(HeaderFrameOptions, "DENY")
		h.Set(HeaderReferrerPolicy, "strict-origin-when-cross-origin")
		h.Set(HeaderPermissionPolicy, "geolocation=(), microphone=(), camera=()")
		h.Set(HeaderContentSecurityPolicy, mw.buildCSP())
		if mw.cfg.TLSEnabled() {
			h.Set(HeaderStrictTransportSecurity, "max-age=31536000; includeSubDomains")
		}
		next(ctx)
	}
}

func (mw *MwHandler) CheckBodySizeLimit(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		maxBytes := mw.cfg.HTTP.MaxRequestBodySize
		if maxBytes <= 0 {
			maxBytes = 1 << 20
		}
		if cl := ctx.Request.Header.ContentLength(); cl > 0 && int64(cl) > maxBytes {
			httpstatus.WriteErrorMessage(ctx, fasthttp.StatusRequestEntityTooLarge, httpstatus.CodeValidation, "request body too large")
			return
		}
		if int64(len(ctx.Request.Body())) > maxBytes {
			httpstatus.WriteErrorMessage(ctx, fasthttp.StatusRequestEntityTooLarge, httpstatus.CodeValidation, "request body too large")
			return
		}
		next(ctx)
	}
}

func (mw *MwHandler) ApplyMetrics(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		if !isAPIPath(requestPath(ctx)) {
			next(ctx)
			return
		}
		start := time.Now()
		next(ctx)
		metrics.ObserveHTTP(commonstrings.BytesToString(ctx.Method()), routeLabel(ctx), ctx.Response.StatusCode(), time.Since(start))
	}
}

func routeLabel(ctx *fasthttp.RequestCtx) string {
	path := requestPath(ctx)
	if strings.HasPrefix(path, "/api/v1/clusters/") {
		return "/api/v1/clusters/{clusterId}"
	}
	return path
}

func requestID(ctx *fasthttp.RequestCtx) string {
	if id, ok := ctx.UserValue("request_id").(string); ok {
		return id
	}
	return ""
}

func isAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/")
}

func (mw *MwHandler) buildCSP() string {
	origins := mw.cfg.CORSOrigins()
	connect := "'self' ws: wss:"
	if len(origins) > 0 {
		var connectSb36 strings.Builder
		for _, origin := range origins {
			connectSb36.WriteString(" " + origin)
		}
		connect += connectSb36.String()
	}
	return strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"img-src 'self' data: blob:",
		"font-src 'self' https://fonts.gstatic.com",
		"connect-src " + connect,
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"object-src 'none'",
	}, "; ")
}

func requiresAuth(path string) bool {
	return isAPIPath(path) && !isPublicPath(path)
}

func Chain(mws ...func(fasthttp.RequestHandler) fasthttp.RequestHandler) func(fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(final fasthttp.RequestHandler) fasthttp.RequestHandler {
		for _, v := range slices.Backward(mws) {
			final = v(final)
		}
		return final
	}
}

func clusterIDFromPath(path string) string {
	const prefix = "/api/v1/clusters/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	if commonstrings.IsEmpty(rest) {
		return ""
	}
	clusterID, _, _ := strings.Cut(rest, "/")
	if commonstrings.IsEmpty(clusterID) {
		return ""
	}
	if _, ok := staticClusterPathSegments[clusterID]; ok {
		return ""
	}
	if !uuidPattern.MatchString(clusterID) {
		return ""
	}
	return clusterID
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var staticClusterPathSegments = map[string]struct{}{
	"connections": {},
}

// isJetStreamResourcePath reports whether a cluster API path mutates streams,
// consumers, KV, or object stores (and thus requires CanManageJetStream).
func isJetStreamResourcePath(path string) bool {
	seg := clusterSubResource(path)
	switch seg {
	case "streams", "kv", "objects", "request-reply", "zombies", "subject-naming", "event-genome", "event-catalog", "event-wikipedia":
		return true
	default:
		return false
	}
}

// clusterSubResource extracts the first path segment following the
// clusterId in a /api/v1/clusters/{clusterId}/... path, or "" when the path
// is exactly /api/v1/clusters/{clusterId} (no sub-resource). Returns "" also
// when path does not match the cluster prefix at all; callers that need to
// distinguish "no sub-resource" from "not a cluster path" should call
// clusterIDFromPath first.
func clusterSubResource(path string) string {
	const prefix = "/api/v1/clusters/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	_, after, _ := strings.Cut(rest, "/")
	seg, _, _ := strings.Cut(after, "/")
	return seg
}

// canReadClusterPath authorizes a GET/HEAD request under
// /api/v1/clusters/{clusterId}/... . Account-scoped sub-resources (see
// accountScopedClusterSegments) accept any grant touching the cluster
// (system, account, or nats-user); cluster-wide sub-resources (JetStream
// data, monitoring, topology, live tail, etc.) require full cluster-wide
// (system-level) access via auth.CanAccessCluster.
func canReadClusterPath(user store.User, clusterID, path string) bool {
	if isAccountScopedClusterPath(path) {
		return auth.CanAccessClusterOrAccount(user, clusterID)
	}
	return auth.CanAccessCluster(user, clusterID)
}

// accountScopedClusterSegments enumerates cluster sub-resources that are
// scoped to an individual NATS account (or its users), as opposed to
// cluster-wide resources such as JetStream data, server monitoring, or
// topology. Holders of an account/nats-user grant for the cluster (see
// auth.CanAccessClusterOrAccount) may reach these without full cluster-wide
// (system-level) access; the underlying handlers are responsible for any
// further per-account narrowing.
var accountScopedClusterSegments = map[string]struct{}{
	"":                    {}, // GetCluster / bare cluster metadata: mirrors ListClusters filtering
	"connection":          {}, // single cluster connection status: mirrors ListClusterConnections filtering
	"access":              {},
	"accounts":            {},
	"nats-users":          {},
	"subject-permissions": {},
	"signing-groups":      {},
	"sharing":             {},
}

// isAccountScopedClusterPath reports whether a cluster API path targets an
// account-scoped sub-resource (see accountScopedClusterSegments) rather than
// a cluster-wide one (streams, kv, objects, topology, monitoring, etc.).
func isAccountScopedClusterPath(path string) bool {
	_, scoped := accountScopedClusterSegments[clusterSubResource(path)]
	return scoped
}
