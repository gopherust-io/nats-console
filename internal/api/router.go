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
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/internal/store"
)

type RouterDeps struct {
	Services    *app.Services
	AuditWriter *audit.Writer
	Store       *store.Store
	SnapshotHub *snapshot.Hub
	Config      config.Config
}

func NewRouter(deps RouterDeps) fasthttp.RequestHandler {
	h := NewHandler(deps.Services, deps.Config, deps.SnapshotHub)
	authH := NewAuthHandler(deps.Services.Auth, deps.Config)
	assistantH := NewAssistantHandler(deps.Services.Assistant)
	usersH := NewUsersHandler(deps.Services, deps.Config)
	auditH := NewAuditHandler(deps.Services, deps.Config)
	adminH := NewAdminHandler(deps.Store)
	natsAccountH := NewNATSAccountHandler(deps.Store, deps.Services.Auth, deps.Config)
	accessH := NewAccessHandler(deps.Store, deps.Services.Auth, deps.Config)
	metricsH := NewMetricsHistoryHandler(deps.Store)
	alertsH := NewAlertsHandler(deps.Store, deps.Config)
	liveHub := live.NewHub(deps.Services.JetStream, deps.Config)
	r := router.New()

	r.GET("/api/health", h.Health)
	r.GET("/api/openapi.yaml", openapiHandler(deps.Config.OpenAPIPath))

	r.GET("/api/v1/auth/me", authH.Me)
	r.GET("/api/v1/auth/config", authH.Config)
	r.GET("/api/v1/assistant/config", assistantH.Config)
	r.POST("/api/v1/auth/login", authH.Login)
	r.POST("/api/v1/auth/logout", authH.Logout)
	r.GET("/api/v1/auth/invite/{token}", accessH.GetInvite)
	r.POST("/api/v1/auth/invite/accept", accessH.AcceptInvite)

	r.GET("/api/v1/users", usersH.List)
	r.POST("/api/v1/users", usersH.Create)
	r.PUT("/api/v1/users/{userId}", usersH.Update)
	r.DELETE("/api/v1/users/{userId}", usersH.Delete)
	r.PUT("/api/v1/users/{userId}/roles", usersH.SetRoles)
	r.GET("/api/v1/people", usersH.List)
	r.POST("/api/v1/people", usersH.Create)
	r.POST("/api/v1/people/invite", accessH.InvitePerson)
	r.GET("/api/v1/audit", auditH.List)
	r.POST("/api/v1/admin/rotate-encryption-key", adminH.RotateEncryptionKey)

	r.GET("/api/v1/alerts", alertsH.List)
	r.GET("/api/v1/alerts/open-summary", alertsH.OpenSummary)
	r.GET("/api/v1/alerts/{alertId}", alertsH.Get)
	r.POST("/api/v1/alerts/{alertId}/acknowledge", alertsH.Acknowledge)
	r.GET("/api/v1/alert-rules", alertsH.ListRules)
	r.POST("/api/v1/alert-rules", alertsH.CreateRule)
	r.GET("/api/v1/alert-rules/metrics", alertsH.Metrics)
	r.GET("/api/v1/alert-rules/{ruleId}", alertsH.GetRule)
	r.PATCH("/api/v1/alert-rules/{ruleId}", alertsH.UpdateRule)
	r.DELETE("/api/v1/alert-rules/{ruleId}", alertsH.DeleteRule)

	prefix := "/api/v1/clusters/{clusterId}"

	r.GET(prefix+"/access", accessH.ListSystemAccess)
	r.POST(prefix+"/access", accessH.UpsertSystemAccess)
	r.PUT(prefix+"/access", accessH.UpsertSystemAccess)
	r.DELETE(prefix+"/access/{grantId}", accessH.DeleteSystemAccess)
	r.GET(prefix+"/accounts/{account}/access", accessH.ListAccountAccess)
	r.POST(prefix+"/accounts/{account}/access", accessH.UpsertAccountAccess)
	r.PUT(prefix+"/accounts/{account}/access", accessH.UpsertAccountAccess)
	r.DELETE(prefix+"/accounts/{account}/access/{grantId}", accessH.DeleteAccountAccess)

	r.GET(prefix+"/nats-users", natsAccountH.ListUsers)
	r.POST(prefix+"/nats-users", natsAccountH.CreateUser)
	r.GET(prefix+"/nats-users/{userId}", natsAccountH.GetUser)
	r.PUT(prefix+"/nats-users/{userId}", natsAccountH.UpdateUser)
	r.DELETE(prefix+"/nats-users/{userId}", natsAccountH.DeleteUser)
	r.GET(prefix+"/nats-users/{userId}/creds", natsAccountH.DownloadCreds)
	r.POST(prefix+"/nats-users/{userId}/rotate", natsAccountH.RotateUser)
	r.POST(prefix+"/nats-users/{userId}/mint-jwt", natsAccountH.MintJWT)
	r.POST(prefix+"/nats-users/{userId}/assign", natsAccountH.AssignPerson)
	r.GET(prefix+"/signing-groups", natsAccountH.ListSigningGroups)
	r.POST(prefix+"/signing-groups", natsAccountH.CreateSigningGroup)
	r.PUT(prefix+"/signing-groups/{groupId}", natsAccountH.UpdateSigningGroup)
	r.DELETE(prefix+"/signing-groups/{groupId}", natsAccountH.DeleteSigningGroup)
	r.GET(prefix+"/sharing/exports", natsAccountH.ListExports)
	r.POST(prefix+"/sharing/exports", natsAccountH.CreateExport)
	r.PUT(prefix+"/sharing/exports/{exportId}", natsAccountH.UpdateExport)
	r.DELETE(prefix+"/sharing/exports/{exportId}", natsAccountH.DeleteExport)

	r.GET("/api/v1/pprof/config", h.PprofConfig)
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

	r.GET(prefix+"/account", h.AccountInfo)
	r.GET(prefix+"/metrics/history", metricsH.History)
	r.GET(prefix+"/topology", h.Topology)
	r.GET(prefix+"/snapshots/events", h.SnapshotEventsSSE)
	r.GET(prefix+"/monitoring/varz", h.Varz)
	r.GET(prefix+"/monitoring/jsz", h.Jsz)
	r.GET(prefix+"/monitoring/{endpoint}", h.Monitoring)

	r.GET(prefix+"/streams", h.ListStreams)
	r.POST(prefix+"/streams", h.CreateStream)
	r.GET(prefix+"/streams/{name}", h.GetStream)
	r.PUT(prefix+"/streams/{name}", h.UpdateStream)
	r.DELETE(prefix+"/streams/{name}", h.DeleteStream)
	r.POST(prefix+"/streams/{name}/purge", h.PurgeStream)
	r.GET(prefix+"/streams/{name}/consumers", h.ListConsumers)
	r.POST(prefix+"/streams/{name}/consumers", h.CreateConsumer)
	r.GET(prefix+"/streams/{name}/consumers/{consumer}", h.GetConsumer)
	r.PUT(prefix+"/streams/{name}/consumers/{consumer}", h.UpdateConsumer)
	r.DELETE(prefix+"/streams/{name}/consumers/{consumer}", h.DeleteConsumer)
	r.POST(prefix+"/streams/{name}/consumers/{consumer}/replay", h.ReplayConsumer)
	r.GET(prefix+"/streams/{name}/messages", h.GetMessage)
	r.POST(prefix+"/streams/{name}/messages", h.PublishMessage)

	r.GET(prefix+"/live/ws", liveHub.Handle)
	r.POST(prefix+"/assistant/chat", assistantH.Chat)

	r.GET(prefix+"/kv/buckets", h.ListKVBuckets)
	r.POST(prefix+"/kv/buckets", h.CreateKVBucket)
	r.GET(prefix+"/kv/buckets/{bucket}", h.GetKVBucket)
	r.PUT(prefix+"/kv/buckets/{bucket}", h.UpdateKVBucket)
	r.DELETE(prefix+"/kv/buckets/{bucket}", h.DeleteKVBucket)
	r.GET(prefix+"/kv/buckets/{bucket}/keys", h.ListKVKeys)
	r.GET(prefix+"/kv/buckets/{bucket}/keys/{key}", h.GetKVEntry)
	r.PUT(prefix+"/kv/buckets/{bucket}/keys/{key}", h.PutKVEntry)
	r.DELETE(prefix+"/kv/buckets/{bucket}/keys/{key}", h.DeleteKVEntry)
	r.GET(prefix+"/kv/buckets/{bucket}/keys/{key}/history", h.KVHistory)

	r.GET(prefix+"/objects/buckets", h.ListObjectBuckets)
	r.POST(prefix+"/objects/buckets", h.CreateObjectBucket)
	r.GET(prefix+"/objects/buckets/{bucket}", h.GetObjectBucket)
	r.PUT(prefix+"/objects/buckets/{bucket}", h.UpdateObjectBucket)
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
		securityHeadersMiddleware(buildCSP(deps.Config), deps.Config.TLSEnabled()),
		bodySizeLimitMiddleware(deps.Config.MaxBodyBytes()),
		authRateLimitMiddleware(deps.Config),
		metricsMiddleware,
		requestLogMiddleware,
		timeoutMiddlewareWithAI(deps.Config.RequestTimeout, deps.Config.AIRequestTimeout),
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
