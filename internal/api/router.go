package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"

	"github.com/gopherust-io/nats-consol/internal/api/middleware"
	"github.com/gopherust-io/nats-consol/internal/api/spa"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/audit"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/live"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type Router struct {
	accessHandler         *AccessHandler
	alertsHandler         *AlertsHandler
	authHandler           *AuthHandler
	assistantHandler      *AssistantHandler
	userHandler           *UsersHandler
	auditHandler          *AuditHandler
	adminHandler          *AdminHandler
	natsAccHandler        *NATSAccountHandler
	jsHandler             *Handler
	authService           *auth.Service
	metricsHandler        *MetricsHistoryHandler
	incidentHandler       *IncidentReconstructionHandler
	requestReplyHandler   *RequestReplyHandler
	eventCatalogHandler   *EventCatalogHandler
	eventWikipediaHandler *EventWikipediaHandler
	liveHub               *live.Hub
	mw                    *middleware.MwHandler
	cfg                   config.Config
}

func NewRouter(
	services *app.Services,
	auditWriter *audit.Writer,
	snapshotHub *snapshot.Hub,
	cfg config.Config) *Router {
	return &Router{
		cfg:                   cfg,
		jsHandler:             NewHandler(services, cfg, snapshotHub),
		authHandler:           NewAuthHandler(services.Auth, cfg),
		assistantHandler:      NewAssistantHandler(services.Assistant),
		userHandler:           NewUsersHandler(services, cfg),
		auditHandler:          NewAuditHandler(services, cfg),
		adminHandler:          NewAdminHandler(services.Admin),
		natsAccHandler:        NewNATSAccountHandler(services.NATSAccounts, services.Auth, cfg),
		accessHandler:         NewAccessHandler(services.Access, services.Users, services.Auth, cfg),
		metricsHandler:        NewMetricsHistoryHandler(services.Metrics),
		alertsHandler:         NewAlertsHandler(services.Alerts, cfg),
		incidentHandler:       NewIncidentReconstructionHandler(services.Incident),
		requestReplyHandler:   NewRequestReplyHandler(services, snapshotHub),
		eventCatalogHandler:   NewEventCatalogHandler(services, snapshotHub, cfg.MaxMonitoringBodyBytes),
		eventWikipediaHandler: NewEventWikipediaHandler(services, snapshotHub, cfg.MaxMonitoringBodyBytes),
		liveHub:               live.NewHub(services.JetStream, cfg),
		mw:                    middleware.New(cfg, services.Auth, auditWriter),
		authService:           services.Auth,
	}
}

func (rh *Router) InitRouter() fasthttp.RequestHandler {
	r := router.New()

	r.GET("/api/health", rh.jsHandler.Health)
	r.GET("/api/openapi.yaml", func(path string) fasthttp.RequestHandler {
		// Path comes from server config (OPENAPI_PATH), not user input.
		data, err := os.ReadFile(path) //nolint:gosec // G304: trusted config path
		if err != nil {
			return func(ctx *fasthttp.RequestCtx) {
				httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
			}
		}
		return func(ctx *fasthttp.RequestCtx) {
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetContentType("application/yaml")
			ctx.SetBody(data)
		}
	}(rh.cfg.OpenAPIPath))

	r.GET("/api/v1/auth/me", rh.authHandler.Me)
	r.GET("/api/v1/auth/config", rh.authHandler.Config)
	r.GET("/api/v1/assistant/config", rh.assistantHandler.Config)
	r.GET("/api/v1/architecture-export/demo", rh.jsHandler.ArchitectureExportDemo)
	r.GET("/api/v1/architecture-refactor/demo", rh.jsHandler.ArchitectureRefactorDemo)
	r.GET("/api/v1/architecture-score/demo", rh.jsHandler.ArchitectureScoreDemo)
	r.GET("/api/v1/chaos-story/demo", rh.jsHandler.ChaosStoryDemo)
	r.POST("/api/v1/auth/login", rh.authHandler.Login)
	r.POST("/api/v1/auth/refresh", rh.authHandler.Refresh)
	r.POST("/api/v1/auth/logout", rh.authHandler.Logout)
	r.GET("/api/v1/auth/invite/{token}", rh.accessHandler.GetInvite)
	r.POST("/api/v1/auth/invite/accept", rh.accessHandler.AcceptInvite)

	r.GET("/api/v1/users", rh.userHandler.List)
	r.POST("/api/v1/users", rh.userHandler.Create)
	r.PUT("/api/v1/users/{userId}", rh.userHandler.Update)
	r.DELETE("/api/v1/users/{userId}", rh.userHandler.Delete)
	r.PUT("/api/v1/users/{userId}/roles", rh.userHandler.SetRoles)
	r.GET("/api/v1/people", rh.userHandler.List)
	r.POST("/api/v1/people", rh.userHandler.Create)
	r.POST("/api/v1/people/invite", rh.accessHandler.InvitePerson)
	r.GET("/api/v1/audit", rh.auditHandler.List)
	r.POST("/api/v1/admin/rotate-encryption-key", rh.adminHandler.RotateEncryptionKey)

	r.GET("/api/v1/alerts", rh.alertsHandler.List)
	r.GET("/api/v1/alerts/open-summary", rh.alertsHandler.OpenSummary)
	r.GET("/api/v1/alerts/{alertId}", rh.alertsHandler.Get)
	r.POST("/api/v1/alerts/{alertId}/acknowledge", rh.alertsHandler.Acknowledge)
	r.GET("/api/v1/alert-rules", rh.alertsHandler.ListRules)
	r.POST("/api/v1/alert-rules", rh.alertsHandler.CreateRule)
	r.GET("/api/v1/alert-rules/metrics", rh.alertsHandler.Metrics)
	r.GET("/api/v1/alert-rules/{ruleId}", rh.alertsHandler.GetRule)
	r.PATCH("/api/v1/alert-rules/{ruleId}", rh.alertsHandler.UpdateRule)
	r.DELETE("/api/v1/alert-rules/{ruleId}", rh.alertsHandler.DeleteRule)

	prefix := "/api/v1/clusters/{clusterId}"

	r.GET(prefix+"/access", rh.accessHandler.ListSystemAccess)
	r.POST(prefix+"/access", rh.accessHandler.UpsertSystemAccess)
	r.PUT(prefix+"/access", rh.accessHandler.UpsertSystemAccess)
	r.DELETE(prefix+"/access/{grantId}", rh.accessHandler.DeleteSystemAccess)
	r.GET(prefix+"/accounts/{account}/access", rh.accessHandler.ListAccountAccess)
	r.POST(prefix+"/accounts/{account}/access", rh.accessHandler.UpsertAccountAccess)
	r.PUT(prefix+"/accounts/{account}/access", rh.accessHandler.UpsertAccountAccess)
	r.DELETE(prefix+"/accounts/{account}/access/{grantId}", rh.accessHandler.DeleteAccountAccess)

	r.GET(prefix+"/nats-users", rh.natsAccHandler.ListUsers)
	r.GET(prefix+"/subject-permissions", rh.natsAccHandler.SubjectPermissions)
	// Legacy alias kept so older frontends keep working; static path must stay ahead of {userId}.
	r.GET(prefix+"/nats-users/subject-permissions", rh.natsAccHandler.SubjectPermissions)
	r.POST(prefix+"/nats-users", rh.natsAccHandler.CreateUser)
	r.GET(prefix+"/nats-users/{userId}", rh.natsAccHandler.GetUser)
	r.PUT(prefix+"/nats-users/{userId}", rh.natsAccHandler.UpdateUser)
	r.DELETE(prefix+"/nats-users/{userId}", rh.natsAccHandler.DeleteUser)
	r.GET(prefix+"/nats-users/{userId}/creds", rh.natsAccHandler.DownloadCreds)
	r.POST(prefix+"/nats-users/{userId}/rotate", rh.natsAccHandler.RotateUser)
	r.POST(prefix+"/nats-users/{userId}/mint-jwt", rh.natsAccHandler.MintJWT)
	r.POST(prefix+"/nats-users/{userId}/assign", rh.natsAccHandler.AssignPerson)
	r.GET(prefix+"/signing-groups", rh.natsAccHandler.ListSigningGroups)
	r.POST(prefix+"/signing-groups", rh.natsAccHandler.CreateSigningGroup)
	r.PUT(prefix+"/signing-groups/{groupId}", rh.natsAccHandler.UpdateSigningGroup)
	r.DELETE(prefix+"/signing-groups/{groupId}", rh.natsAccHandler.DeleteSigningGroup)
	r.GET(prefix+"/sharing/exports", rh.natsAccHandler.ListExports)
	r.POST(prefix+"/sharing/exports", rh.natsAccHandler.CreateExport)
	r.PUT(prefix+"/sharing/exports/{exportId}", rh.natsAccHandler.UpdateExport)
	r.DELETE(prefix+"/sharing/exports/{exportId}", rh.natsAccHandler.DeleteExport)

	r.GET("/api/v1/pprof/config", rh.jsHandler.PprofConfig)
	r.GET("/api/v1/pprof/runtime", rh.jsHandler.PprofRuntime)
	r.GET("/api/v1/pprof/profile/{profile}/download", rh.jsHandler.PprofProfileDownload)
	r.GET("/api/v1/pprof/profile/{profile}", rh.jsHandler.PprofProfileSummary)

	r.GET("/api/v1/clusters", rh.jsHandler.ListClusters)
	r.POST("/api/v1/clusters", rh.jsHandler.CreateClusterDisabled)
	r.GET("/api/v1/clusters/connections", rh.jsHandler.ListClusterConnections)
	r.GET("/api/v1/clusters/{clusterId}", rh.jsHandler.GetCluster)
	r.PUT("/api/v1/clusters/{clusterId}", rh.jsHandler.UpdateClusterDisabled)
	r.DELETE("/api/v1/clusters/{clusterId}", rh.jsHandler.DeleteCluster)
	r.POST("/api/v1/clusters/{clusterId}/test", rh.jsHandler.TestCluster)
	r.GET("/api/v1/clusters/{clusterId}/connection", rh.jsHandler.GetClusterConnection)

	r.GET(prefix+"/account", rh.jsHandler.AccountInfo)
	r.GET(prefix+"/metrics/history", rh.metricsHandler.History)
	r.POST(prefix+"/incident-annotations", rh.incidentHandler.CreateAnnotation)
	r.GET(prefix+"/topology", rh.jsHandler.Topology)
	r.GET(prefix+"/replicas", rh.jsHandler.Replicas)
	r.GET(prefix+"/replicas/events", rh.jsHandler.ReplicasEventsSSE)
	r.GET(prefix+"/zombies", rh.jsHandler.Zombies)
	r.GET(prefix+"/subject-naming", rh.jsHandler.SubjectNaming)
	r.GET(prefix+"/event-genome", rh.jsHandler.EventGenome)
	r.GET(prefix+"/architecture-review", rh.jsHandler.ArchitectureReview)
	r.POST(prefix+"/architecture-review/ask", rh.jsHandler.ArchitectureReviewAsk)
	r.GET(prefix+"/architecture-refactor", rh.jsHandler.ArchitectureRefactor)
	r.POST(prefix+"/architecture-refactor/ask", rh.jsHandler.ArchitectureRefactorAsk)
	r.GET(prefix+"/architecture-score", rh.jsHandler.ArchitectureScore)
	r.POST(prefix+"/architecture-score/ask", rh.jsHandler.ArchitectureScoreAsk)
	r.GET(prefix+"/hidden-bottlenecks", rh.jsHandler.HiddenBottlenecks)
	r.POST(prefix+"/hidden-bottlenecks/ask", rh.jsHandler.HiddenBottlenecksAsk)
	r.GET(prefix+"/chaos-story", rh.jsHandler.ChaosStory)
	r.POST(prefix+"/chaos-story/generate", rh.jsHandler.ChaosStoryGenerate)
	r.GET(prefix+"/architecture-export", rh.jsHandler.ArchitectureExport)
	r.POST(prefix+"/architecture-export", rh.jsHandler.ArchitectureExport)
	r.GET(prefix+"/event-catalog", rh.eventCatalogHandler.List)
	r.PUT(prefix+"/event-catalog/{subject}", rh.eventCatalogHandler.Upsert)
	r.DELETE(prefix+"/event-catalog/{subject}", rh.eventCatalogHandler.Delete)
	r.GET(prefix+"/event-wikipedia", rh.eventWikipediaHandler.List)
	r.GET(prefix+"/request-reply", rh.requestReplyHandler.Snapshot)
	r.GET(prefix+"/snapshots/events", rh.jsHandler.SnapshotEventsSSE)
	r.GET(prefix+"/monitoring/connz/events", rh.jsHandler.ConnzEventsSSE)
	r.GET(prefix+"/monitoring/varz", rh.jsHandler.Varz)
	r.GET(prefix+"/monitoring/jsz", rh.jsHandler.Jsz)
	r.GET(prefix+"/monitoring/{endpoint}", rh.jsHandler.Monitoring)

	r.GET(prefix+"/streams", rh.jsHandler.ListStreams)
	r.POST(prefix+"/streams", rh.jsHandler.CreateStream)
	r.GET(prefix+"/streams/{name}", rh.jsHandler.GetStream)
	r.PUT(prefix+"/streams/{name}", rh.jsHandler.UpdateStream)
	r.DELETE(prefix+"/streams/{name}", rh.jsHandler.DeleteStream)
	r.GET(prefix+"/streams/{name}/impact", rh.jsHandler.GetStreamImpact)
	r.POST(prefix+"/streams/{name}/purge", rh.jsHandler.PurgeStream)
	r.GET(prefix+"/streams/{name}/consumers", rh.jsHandler.ListConsumers)
	r.POST(prefix+"/streams/{name}/consumers", rh.jsHandler.CreateConsumer)
	r.GET(prefix+"/streams/{name}/consumers/{consumer}", rh.jsHandler.GetConsumer)
	r.PUT(prefix+"/streams/{name}/consumers/{consumer}", rh.jsHandler.UpdateConsumer)
	r.DELETE(prefix+"/streams/{name}/consumers/{consumer}", rh.jsHandler.DeleteConsumer)
	r.GET(prefix+"/streams/{name}/consumers/{consumer}/behavior-fingerprint", rh.jsHandler.GetBehaviorFingerprint)
	r.GET(prefix+"/streams/{name}/consumers/{consumer}/incident-reconstruction", rh.incidentHandler.GetReconstruction)
	r.POST(prefix+"/streams/{name}/consumers/{consumer}/replay/dry-run", rh.jsHandler.ReplayConsumerDryRun)
	r.POST(prefix+"/streams/{name}/consumers/{consumer}/replay", rh.jsHandler.ReplayConsumer)
	r.GET(prefix+"/streams/{name}/messages/range", rh.jsHandler.GetMessageRange)
	r.GET(prefix+"/streams/{name}/messages", rh.jsHandler.GetMessage)
	r.POST(prefix+"/streams/{name}/messages", rh.jsHandler.PublishMessage)
	r.GET(prefix+"/streams/{name}/dlq/messages", rh.jsHandler.ListDLQMessages)
	r.POST(prefix+"/streams/{name}/dlq/retry", rh.jsHandler.RetryDLQMessages)

	r.GET(prefix+"/live/ws", rh.liveHub.Handle)
	r.POST(prefix+"/assistant/chat", rh.assistantHandler.Chat)
	r.GET(prefix+"/kv/buckets", rh.jsHandler.ListKVBuckets)
	r.POST(prefix+"/kv/buckets", rh.jsHandler.CreateKVBucket)
	r.GET(prefix+"/kv/buckets/{bucket}", rh.jsHandler.GetKVBucket)
	r.PUT(prefix+"/kv/buckets/{bucket}", rh.jsHandler.UpdateKVBucket)
	r.DELETE(prefix+"/kv/buckets/{bucket}", rh.jsHandler.DeleteKVBucket)
	r.GET(prefix+"/kv/buckets/{bucket}/keys", rh.jsHandler.ListKVKeys)
	r.GET(prefix+"/kv/buckets/{bucket}/keys/{key}", rh.jsHandler.GetKVEntry)
	r.PUT(prefix+"/kv/buckets/{bucket}/keys/{key}", rh.jsHandler.PutKVEntry)
	r.DELETE(prefix+"/kv/buckets/{bucket}/keys/{key}", rh.jsHandler.DeleteKVEntry)
	r.GET(prefix+"/kv/buckets/{bucket}/keys/{key}/history", rh.jsHandler.KVHistory)

	r.GET(prefix+"/objects/buckets", rh.jsHandler.ListObjectBuckets)
	r.POST(prefix+"/objects/buckets", rh.jsHandler.CreateObjectBucket)
	r.GET(prefix+"/objects/buckets/{bucket}", rh.jsHandler.GetObjectBucket)
	r.PUT(prefix+"/objects/buckets/{bucket}", rh.jsHandler.UpdateObjectBucket)
	r.DELETE(prefix+"/objects/buckets/{bucket}", rh.jsHandler.DeleteObjectBucket)
	r.GET(prefix+"/objects/buckets/{bucket}/objects", rh.jsHandler.ListObjects)
	r.GET(prefix+"/objects/buckets/{bucket}/objects/{objectName}", rh.jsHandler.GetObject)
	r.PUT(prefix+"/objects/buckets/{bucket}/objects/{objectName}", rh.jsHandler.PutObject)
	r.DELETE(prefix+"/objects/buckets/{bucket}/objects/{objectName}", rh.jsHandler.DeleteObject)

	if !commonstrings.IsEmpty(rh.cfg.StaticDir) {
		spaHandler := spa.NewSPAHandler(rh.cfg.StaticDir)
		r.NotFound = spaHandler.ServeHTTP
	}

	handler := middleware.Chain(
		rh.mw.ApplyRequestID,
		rh.mw.ApplySecurityHeaders,
		rh.mw.CheckBodySizeLimit,
		rh.mw.VerifyAuthRateLimit,
		rh.mw.ApplyMetrics,
		rh.mw.ApplyRequestLogger,
		rh.mw.ApplyAITimeout,
		rh.mw.VerifyCors,
		rh.mw.VerifyAudit,
		rh.mw.VerifyCSRF,
		rh.mw.VerifyAuth,
		rh.mw.VerifyRBAC)(r.Handler)

	return rh.mw.ApplyRecover(func(ctx *fasthttp.RequestCtx) {
		path := commonstrings.BytesToString(ctx.Path())
		if strings.HasPrefix(path, middleware.PathPrefixPprof) {
			if !rh.cfg.Pprof.Enabled {
				httpstatus.WriteError(ctx, fasthttp.StatusNotFound, errors.New("not found"))
				return
			}
			if rh.cfg.Pprof.AuthEnabled {
				user, ok := rh.mw.Authenticate(ctx)
				if !ok {
					httpstatus.WriteUnauthorized(ctx)
					return
				}
				if !commonstrings.IsEmpty(user.ID) {
					loaded, err := rh.authService.LoadUser(context.Background(), user.ID)
					if err != nil {
						httpstatus.WriteUnauthorized(ctx)
						return
					}
					user = loaded
				}
				if !auth.CanViewProfiling(user) {
					httpstatus.WriteForbidden(ctx)
					return
				}
			}
			fasthttpadaptor.NewFastHTTPHandler(http.DefaultServeMux)(ctx)
			return
		}
		handler(ctx)
	})
}
