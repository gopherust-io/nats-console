package api

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/audit"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/live"
)

type RouterDeps struct {
	Services    *app.Services
	AuditWriter *audit.Writer
	Config      config.Config
}

func NewRouter(deps RouterDeps) fasthttp.RequestHandler {
	h := NewHandler(deps.Services, deps.Config)
	authH := NewAuthHandler(deps.Services.Auth, deps.Config)
	assistantH := NewAssistantHandler(deps.Services.Assistant)
	usersH := NewUsersHandler(deps.Services, deps.Config)
	auditH := NewAuditHandler(deps.Services, deps.Config)
	liveHub := live.NewHub(deps.Services.JetStream, deps.Config)
	r := router.New()

	r.GET("/api/health", h.Health)
	r.GET("/metrics", promMetricsHandler)
	r.GET("/api/openapi.yaml", openapiHandler(deps.Config.OpenAPIPath))

	r.GET("/api/v1/auth/me", authH.Me)
	r.GET("/api/v1/auth/config", authH.Config)
	r.GET("/api/v1/assistant/config", assistantH.Config)
	r.POST("/api/v1/auth/login", authH.Login)
	r.POST("/api/v1/auth/logout", authH.Logout)
	r.GET("/api/v1/auth/oidc/login", authH.OIDCLogin)
	r.GET("/api/v1/auth/oidc/callback", authH.OIDCCallback)
	r.GET("/api/v1/auth/oidc/{provider}/login", authH.SSOProviderLogin)
	r.GET("/api/v1/auth/oidc/{provider}/callback", authH.SSOProviderCallback)

	r.GET("/api/v1/users", usersH.List)
	r.POST("/api/v1/users", usersH.Create)
	r.PUT("/api/v1/users/{userId}", usersH.Update)
	r.DELETE("/api/v1/users/{userId}", usersH.Delete)
	r.PUT("/api/v1/users/{userId}/roles", usersH.SetRoles)
	r.GET("/api/v1/audit", auditH.List)

	r.GET("/api/v1/pprof/config", h.PprofConfig)
	r.GET("/api/v1/pprof/continuous", h.PprofContinuous)
	r.GET("/api/v1/pprof/runtime", h.PprofRuntime)
	r.GET("/api/v1/pprof/profile/{profile}/download", h.PprofProfileDownload)
	r.GET("/api/v1/pprof/profile/{profile}", h.PprofProfileSummary)

	r.GET("/api/v1/clusters", h.ListClusters)
	r.POST("/api/v1/clusters", h.CreateCluster)
	r.GET("/api/v1/clusters/connections", h.ListClusterConnections)
	r.GET("/api/v1/clusters/{clusterId}", h.GetCluster)
	r.PUT("/api/v1/clusters/{clusterId}", h.UpdateCluster)
	r.DELETE("/api/v1/clusters/{clusterId}", h.DeleteCluster)
	r.POST("/api/v1/clusters/{clusterId}/test", h.TestCluster)
	r.GET("/api/v1/clusters/{clusterId}/connection", h.GetClusterConnection)

	prefix := "/api/v1/clusters/{clusterId}"
	r.GET(prefix+"/account", h.AccountInfo)
	r.GET(prefix+"/monitoring/varz", h.Varz)
	r.GET(prefix+"/monitoring/jsz", h.Jsz)
	r.GET(prefix+"/monitoring/{endpoint}", h.Monitoring)
	r.GET(prefix+"/supercluster", h.Supercluster)

	r.GET(prefix+"/streams", h.ListStreams)
	r.POST(prefix+"/streams", h.CreateStream)
	r.GET(prefix+"/streams/{name}", h.GetStream)
	r.PUT(prefix+"/streams/{name}", h.UpdateStream)
	r.DELETE(prefix+"/streams/{name}", h.DeleteStream)
	r.POST(prefix+"/streams/{name}/purge", h.PurgeStream)
	r.GET(prefix+"/streams/{name}/consumers", h.ListConsumers)
	r.POST(prefix+"/streams/{name}/consumers", h.CreateConsumer)
	r.GET(prefix+"/streams/{name}/consumers/{consumer}", h.GetConsumer)
	r.DELETE(prefix+"/streams/{name}/consumers/{consumer}", h.DeleteConsumer)
	r.GET(prefix+"/streams/{name}/messages", h.GetMessage)

	r.GET(prefix+"/live/ws", liveHub.Handle)
	r.POST(prefix+"/assistant/chat", assistantH.Chat)

	r.GET(prefix+"/kv/buckets", h.ListKVBuckets)
	r.POST(prefix+"/kv/buckets", h.CreateKVBucket)
	r.GET(prefix+"/kv/buckets/{bucket}", h.GetKVBucket)
	r.DELETE(prefix+"/kv/buckets/{bucket}", h.DeleteKVBucket)
	r.GET(prefix+"/kv/buckets/{bucket}/keys", h.ListKVKeys)
	r.GET(prefix+"/kv/buckets/{bucket}/keys/{key}", h.GetKVEntry)
	r.PUT(prefix+"/kv/buckets/{bucket}/keys/{key}", h.PutKVEntry)
	r.DELETE(prefix+"/kv/buckets/{bucket}/keys/{key}", h.DeleteKVEntry)
	r.GET(prefix+"/kv/buckets/{bucket}/keys/{key}/history", h.KVHistory)

	r.GET(prefix+"/objects/buckets", h.ListObjectBuckets)
	r.POST(prefix+"/objects/buckets", h.CreateObjectBucket)
	r.GET(prefix+"/objects/buckets/{bucket}", h.GetObjectBucket)
	r.DELETE(prefix+"/objects/buckets/{bucket}", h.DeleteObjectBucket)
	r.GET(prefix+"/objects/buckets/{bucket}/objects", h.ListObjects)
	r.GET(prefix+"/objects/buckets/{bucket}/objects/{objectName}", h.GetObject)
	r.PUT(prefix+"/objects/buckets/{bucket}/objects/{objectName}", h.PutObject)
	r.DELETE(prefix+"/objects/buckets/{bucket}/objects/{objectName}", h.DeleteObject)

	if deps.Config.StaticDir != "" {
		spa := newSPAHandler(deps.Config.StaticDir)
		r.NotFound = spa.ServeHTTP
	}

	mws := []middleware{
		requestIDMiddleware,
		securityHeadersMiddleware(deps.Config),
		bodySizeLimitMiddleware(deps.Config.MaxBodyBytes()),
		authRateLimitMiddleware(deps.Config),
		metricsMiddleware,
		requestLogMiddleware,
		timeoutMiddleware(deps.Config.RequestTimeout),
		corsMiddleware(deps.Config),
		auditMiddleware(deps.AuditWriter),
	}

	if deps.Config.AuthEnabled {
		mws = append(mws, csrfMiddleware(deps.Config), authMiddleware(deps.Config, deps.Services.Auth), rbacMiddleware)
	}

	finalHandler := chain(mws...)(r.Handler)

	return func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())
		if strings.HasPrefix(path, pprofPathPrefix) {
			if !deps.Config.PprofEnabled || deps.Config.IsProduction() {
				ctx.SetStatusCode(fasthttp.StatusNotFound)
				return
			}
			if deps.Config.PprofAuthEnabled {
				user, ok := authenticate(ctx, deps.Services.Auth)
				if !ok {
					ctx.SetStatusCode(fasthttp.StatusUnauthorized)
					return
				}
				if user.ID != "" {
					loaded, err := deps.Services.Auth.LoadUser(context.Background(), user.ID)
					if err != nil {
						ctx.SetStatusCode(fasthttp.StatusUnauthorized)
						return
					}
					user = loaded
				}
				if !auth.CanViewProfiling(user) {
					ctx.SetStatusCode(fasthttp.StatusForbidden)
					ctx.SetBodyString("forbidden")
					return
				}
			}
			serveStdPprof(ctx)
			return
		}
		if deps.Config.MetricsAuthEnabled && path == "/metrics" {
			user, ok := authenticate(ctx, deps.Services.Auth)
			if !ok {
				ctx.SetStatusCode(fasthttp.StatusUnauthorized)
				return
			}
			if user.ID != "" {
				loaded, err := deps.Services.Auth.LoadUser(context.Background(), user.ID)
				if err != nil {
					ctx.SetStatusCode(fasthttp.StatusUnauthorized)
					return
				}
				user = loaded
			}
			if !auth.CanViewMetrics(user) {
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				ctx.SetBodyString("forbidden")
				return
			}
		}
		finalHandler(ctx)
	}
}

type spaHandler struct {
	staticDir string
	index     string
}

func newSPAHandler(staticDir string) *spaHandler {
	absDir, err := filepath.Abs(staticDir)
	if err != nil {
		absDir = staticDir
	}
	return &spaHandler{
		staticDir: absDir,
		index:     filepath.Join(absDir, "index.html"),
	}
}

// safeStaticFilePath resolves a URL path under rootDir, rejecting traversal outside the root.
func safeStaticFilePath(rootDir, urlPath string) (string, bool) {
	cleaned := path.Clean("/" + urlPath)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" || cleaned == "." || strings.Contains(cleaned, "..") {
		return "", false
	}

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", false
	}

	candidate := filepath.Join(absRoot, filepath.FromSlash(cleaned))
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", false
	}

	rel, err := filepath.Rel(absRoot, absCandidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}

	return absCandidate, true
}

func (s *spaHandler) ServeHTTP(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	if path == "/" {
		fasthttp.ServeFile(ctx, s.index)
		return
	}

	if isAPIPath(path) {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}

	filePath, ok := safeStaticFilePath(s.staticDir, path)
	if ok {
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			fasthttp.ServeFile(ctx, filePath)
			return
		}
	}

	fasthttp.ServeFile(ctx, s.index)
}
