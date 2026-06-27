package api

import (
	"encoding/base64"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/live"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/store"
)

func NewRouter(cfg config.Config, st *store.Store, nats *natsclient.Manager) fasthttp.RequestHandler {
	h := NewHandler(st, nats)
	liveHub := live.NewHub(nats)
	r := router.New()

	r.GET("/api/health", h.Health)

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

	if cfg.StaticDir != "" {
		spa := newSPAHandler(cfg.StaticDir)
		r.NotFound = spa.ServeHTTP
	}

	mws := []middleware{logMiddleware, corsMiddleware}
	if cfg.AuthEnabled {
		mws = append(mws, authMiddleware(cfg.AdminUsername, cfg.AdminPassword))
	}

	return chain(mws...)(r.Handler)
}

type middleware func(fasthttp.RequestHandler) fasthttp.RequestHandler

func chain(mws ...middleware) middleware {
	return func(final fasthttp.RequestHandler) fasthttp.RequestHandler {
		for i := len(mws) - 1; i >= 0; i-- {
			final = mws[i](final)
		}
		return final
	}
}

func logMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		start := time.Now()
		next(ctx)
		log.Printf("%s %s %d %s",
			string(ctx.Method()),
			string(ctx.Path()),
			ctx.Response.StatusCode(),
			time.Since(start),
		)
	}
}

func corsMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
		ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		ctx.Response.Header.Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type")
		ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")

		if ctx.IsOptions() {
			ctx.SetStatusCode(fasthttp.StatusNoContent)
			return
		}

		next(ctx)
	}
}

func authMiddleware(username, password string) middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			if string(ctx.Path()) == "/api/health" {
				next(ctx)
				return
			}

			if !validBasicAuth(ctx, username, password) && !validWSAuth(ctx, username, password) {
				ctx.Response.Header.Set("WWW-Authenticate", `Basic realm="nats-consol"`)
				ctx.SetStatusCode(fasthttp.StatusUnauthorized)
				ctx.SetBodyString("unauthorized")
				return
			}

			next(ctx)
		}
	}
}

func validWSAuth(ctx *fasthttp.RequestCtx, username, password string) bool {
	if !strings.Contains(string(ctx.Path()), "/live/ws") {
		return false
	}
	auth := string(ctx.QueryArgs().Peek("authorization"))
	if auth == "" {
		return false
	}
	if !strings.HasPrefix(auth, "Basic ") {
		auth = "Basic " + auth
	}
	ctx.Request.Header.Set("Authorization", auth)
	return validBasicAuth(ctx, username, password)
}

func validBasicAuth(ctx *fasthttp.RequestCtx, username, password string) bool {
	auth := string(ctx.Request.Header.Peek("Authorization"))
	if !strings.HasPrefix(auth, "Basic ") {
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		return false
	}

	user, pass, ok := strings.Cut(string(decoded), ":")
	return ok && user == username && pass == password
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

	filePath := filepath.Join(s.staticDir, path)
	if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
		fasthttp.ServeFile(ctx, filePath)
		return
	}

	fasthttp.ServeFile(ctx, s.index)
}
