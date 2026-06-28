package api

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/audit"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/live"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/store"
)

type RouterDeps struct {
	Config      config.Config
	Store       *store.Store
	NATS        *natsclient.Manager
	Auth        *auth.Service
	AuditWriter *audit.Writer
}

func NewRouter(deps RouterDeps) fasthttp.RequestHandler {
	h := NewHandler(deps.Store, deps.NATS)
	authH := NewAuthHandler(deps.Auth)
	usersH := NewUsersHandler(deps.Store)
	auditH := NewAuditHandler(deps.Store)
	liveHub := live.NewHub(deps.NATS)
	r := router.New()

	r.GET("/api/health", h.Health)
	r.GET("/metrics", metricsHandler)
	r.GET("/api/openapi.yaml", openapiHandler(deps.Config.OpenAPIPath))

	r.GET("/api/v1/auth/me", authH.Me)
	r.GET("/api/v1/auth/config", authH.Config)
	r.POST("/api/v1/auth/login", authH.Login)
	r.POST("/api/v1/auth/logout", authH.Logout)
	r.GET("/api/v1/auth/oidc/login", authH.OIDCLogin)
	r.GET("/api/v1/auth/oidc/callback", authH.OIDCCallback)

	r.GET("/api/v1/users", usersH.List)
	r.PUT("/api/v1/users/{userId}/roles", usersH.SetRoles)
	r.GET("/api/v1/audit", auditH.List)

	r.GET("/api/v1/clusters", h.ListClusters)
	r.POST("/api/v1/clusters", h.CreateCluster)
	r.GET("/api/v1/clusters/{clusterId}", h.GetCluster)
	r.PUT("/api/v1/clusters/{clusterId}", h.UpdateCluster)
	r.DELETE("/api/v1/clusters/{clusterId}", h.DeleteCluster)
	r.POST("/api/v1/clusters/{clusterId}/test", h.TestCluster)

	prefix := "/api/v1/clusters/{clusterId}"
	r.GET(prefix+"/account", h.AccountInfo)
	r.GET(prefix+"/monitoring/varz", h.Varz)
	r.GET(prefix+"/monitoring/jsz", h.Jsz)

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
		metricsMiddleware,
		slogMiddleware,
		timeoutMiddleware(deps.Config.RequestTimeout),
		corsMiddleware(deps.Config),
		auditMiddleware(deps.AuditWriter),
	}

	if deps.Config.AuthEnabled {
		mws = append(mws, authMiddleware(deps.Config, deps.Auth), rbacMiddleware)
	}

	finalHandler := chain(mws...)(r.Handler)

	if deps.Config.MetricsAuthEnabled {
		return func(ctx *fasthttp.RequestCtx) {
			if string(ctx.Path()) == "/metrics" {
				if _, ok := authenticate(ctx, deps.Auth); !ok {
					ctx.SetStatusCode(fasthttp.StatusUnauthorized)
					return
				}
			}
			finalHandler(ctx)
		}
	}

	return finalHandler
}

type spaHandler struct {
	staticDir string
	index     string
}

func newSPAHandler(staticDir string) *spaHandler {
	return &spaHandler{
		staticDir: staticDir,
		index:     filepath.Join(staticDir, "index.html"),
	}
}

func (s *spaHandler) ServeHTTP(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	if path == "/" {
		fasthttp.ServeFile(ctx, s.index)
		return
	}

	if strings.HasPrefix(path, "/api/") {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}

	filePath := filepath.Join(s.staticDir, path)
	if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
		fasthttp.ServeFile(ctx, filePath)
		return
	}

	fasthttp.ServeFile(ctx, s.index)
}
