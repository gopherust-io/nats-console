package api

import (
	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/accounts"
	"github.com/gopherust-io/nats-consol/internal/api/insights"
	"github.com/gopherust-io/nats-consol/internal/api/jetstream"
	"github.com/gopherust-io/nats-consol/internal/api/kvobj"
	"github.com/gopherust-io/nats-consol/internal/api/middleware"
	"github.com/gopherust-io/nats-consol/internal/api/ops"
	"github.com/gopherust-io/nats-consol/internal/api/spa"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/audit"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/live"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type Router struct {
	accessHandler         *AccessHandler
	alertsHandler         *AlertsHandler
	authHandler           *AuthHandler
	assistantHandler      *AssistantHandler
	userHandler           *UsersHandler
	auditHandler          *AuditHandler
	adminHandler          *AdminHandler
	schemaHandler         *Handler
	natsAccHandler        *accounts.Handler
	jsHandler             *jetstream.Handler
	kvHandler             *kvobj.Handler
	insightsHandler       *insights.Handler
	opsHandler            *ops.Handler
	metricsHandler        *MetricsHistoryHandler
	incidentHandler       *IncidentReconstructionHandler
	requestReplyHandler   *RequestReplyHandler
	eventCatalogHandler   *insights.EventCatalogHandler
	eventWikipediaHandler *insights.EventWikipediaHandler
	liveHub               *live.Hub
	mw                    *middleware.MwHandler
	r                     *router.Router
	staticDir             string
}

func NewRouter(
	cfg config.Config,
	services *app.Services,
	auditWriter *audit.Writer,
	snapshotHub *snapshot.Hub,
) *Router {
	return &Router{
		schemaHandler:         NewHandler(services, cfg, snapshotHub),
		jsHandler:             jetstream.NewHandler(services, cfg, snapshotHub),
		kvHandler:             kvobj.NewHandler(services, cfg, snapshotHub),
		insightsHandler:       insights.NewHandler(services, cfg, snapshotHub),
		opsHandler:            ops.NewHandler(services, cfg, snapshotHub),
		natsAccHandler:        accounts.NewHandler(services.NATSAccounts, services.Auth, cfg),
		authHandler:           NewAuthHandler(services.Auth, cfg),
		assistantHandler:      NewAssistantHandler(services.Assistant),
		userHandler:           NewUsersHandler(services, cfg),
		auditHandler:          NewAuditHandler(services, cfg),
		adminHandler:          NewAdminHandler(services.Admin),
		accessHandler:         NewAccessHandler(services.Access, services.Users, services.Auth, cfg),
		metricsHandler:        NewMetricsHistoryHandler(services.Metrics),
		alertsHandler:         NewAlertsHandler(services.Alerts, cfg),
		incidentHandler:       NewIncidentReconstructionHandler(services.Incident),
		requestReplyHandler:   NewRequestReplyHandler(services, snapshotHub),
		eventCatalogHandler:   insights.NewEventCatalogHandler(services, cfg, snapshotHub),
		eventWikipediaHandler: insights.NewEventWikipediaHandler(services, cfg, snapshotHub),
		liveHub:               live.NewHub(services.JetStream.Gateway(), cfg),
		mw:                    middleware.New(cfg, services.Auth, auditWriter),
		r:                     router.New(),
		staticDir:             cfg.StaticDir,
	}
}

func (rh *Router) Init() fasthttp.RequestHandler {
	rh.r.GET("/api/health", rh.opsHandler.Health)
	rh.r.GET("/api/openapi.yaml", rh.opsHandler.OpenAPI)
	rh.r.GET("/api/v1/schemas", rh.schemaHandler.OpenAPIModels)
	rh.r.GET("/api/v1/pprof/config", rh.opsHandler.PprofConfig)
	rh.r.GET("/api/v1/pprof/runtime", rh.opsHandler.PprofRuntime)
	rh.r.GET("/api/v1/pprof/profile/{profile}/download", rh.opsHandler.PprofProfileDownload)
	rh.r.GET("/api/v1/pprof/profile/{profile}", rh.opsHandler.PprofProfileSummary)

	rh.r.GET("/api/v1/auth/me", rh.authHandler.Me)
	rh.r.GET("/api/v1/auth/config", rh.authHandler.Config)
	rh.r.GET("/api/v1/assistant/config", rh.assistantHandler.Config)
	rh.r.GET("/api/v1/architecture-export/demo", rh.insightsHandler.ArchitectureExportDemo)
	rh.r.GET("/api/v1/architecture-refactor/demo", rh.insightsHandler.ArchitectureRefactorDemo)
	rh.r.GET("/api/v1/architecture-score/demo", rh.insightsHandler.ArchitectureScoreDemo)
	rh.r.GET("/api/v1/chaos-story/demo", rh.insightsHandler.ChaosStoryDemo)
	rh.r.POST("/api/v1/auth/login", rh.authHandler.Login)
	rh.r.POST("/api/v1/auth/refresh", rh.authHandler.Refresh)
	rh.r.POST("/api/v1/auth/logout", rh.authHandler.Logout)
	rh.r.GET("/api/v1/auth/invite/{token}", rh.accessHandler.GetInvite)
	rh.r.POST("/api/v1/auth/invite/accept", rh.accessHandler.AcceptInvite)

	rh.r.GET("/api/v1/users", rh.userHandler.List)
	rh.r.POST("/api/v1/users", rh.userHandler.Create)
	rh.r.PUT("/api/v1/users/{userId}", rh.userHandler.Update)
	rh.r.DELETE("/api/v1/users/{userId}", rh.userHandler.Delete)
	rh.r.PUT("/api/v1/users/{userId}/roles", rh.userHandler.SetRoles)
	rh.r.GET("/api/v1/people", rh.userHandler.List)
	rh.r.POST("/api/v1/people", rh.userHandler.Create)
	rh.r.POST("/api/v1/people/invite", rh.accessHandler.InvitePerson)
	rh.r.GET("/api/v1/audit", rh.auditHandler.List)
	rh.r.POST("/api/v1/admin/rotate-encryption-key", rh.adminHandler.RotateEncryptionKey)

	rh.r.GET("/api/v1/alerts", rh.alertsHandler.List)
	rh.r.GET("/api/v1/alerts/open-summary", rh.alertsHandler.OpenSummary)
	rh.r.GET("/api/v1/alerts/{alertId}", rh.alertsHandler.Get)
	rh.r.POST("/api/v1/alerts/{alertId}/acknowledge", rh.alertsHandler.Acknowledge)
	rh.r.GET("/api/v1/alert-rules", rh.alertsHandler.ListRules)
	rh.r.POST("/api/v1/alert-rules", rh.alertsHandler.CreateRule)
	rh.r.GET("/api/v1/alert-rules/metrics", rh.alertsHandler.Metrics)
	rh.r.GET("/api/v1/alert-rules/{ruleId}", rh.alertsHandler.GetRule)
	rh.r.PATCH("/api/v1/alert-rules/{ruleId}", rh.alertsHandler.UpdateRule)
	rh.r.DELETE("/api/v1/alert-rules/{ruleId}", rh.alertsHandler.DeleteRule)

	rh.r.GET("/api/v1/clusters", rh.opsHandler.ListClusters)
	rh.r.POST("/api/v1/clusters", rh.opsHandler.CreateClusterDisabled)
	rh.r.GET("/api/v1/clusters/connections", rh.opsHandler.ListClusterConnections)
	rh.r.GET("/api/v1/clusters/{clusterId}", rh.opsHandler.GetCluster)
	rh.r.PUT("/api/v1/clusters/{clusterId}", rh.opsHandler.UpdateClusterDisabled)
	rh.r.DELETE("/api/v1/clusters/{clusterId}", rh.opsHandler.DeleteCluster)
	rh.r.POST("/api/v1/clusters/{clusterId}/test", rh.opsHandler.TestCluster)
	rh.r.GET("/api/v1/clusters/{clusterId}/connection", rh.opsHandler.GetClusterConnection)
	rh.r.GET("/api/v1/clusters/{clusterId}/connection/events", rh.opsHandler.ConnectionEventsSSE)

	const prefix = "/api/v1/clusters/{clusterId}"

	rh.r.GET(prefix+"/access", rh.accessHandler.ListSystemAccess)
	rh.r.POST(prefix+"/access", rh.accessHandler.UpsertSystemAccess)
	rh.r.PUT(prefix+"/access", rh.accessHandler.UpsertSystemAccess)
	rh.r.DELETE(prefix+"/access/{grantId}", rh.accessHandler.DeleteSystemAccess)
	rh.r.GET(prefix+"/accounts/{account}/access", rh.accessHandler.ListAccountAccess)
	rh.r.POST(prefix+"/accounts/{account}/access", rh.accessHandler.UpsertAccountAccess)
	rh.r.PUT(prefix+"/accounts/{account}/access", rh.accessHandler.UpsertAccountAccess)
	rh.r.DELETE(prefix+"/accounts/{account}/access/{grantId}", rh.accessHandler.DeleteAccountAccess)

	rh.r.GET(prefix+"/nats-users", rh.natsAccHandler.ListUsers)
	rh.r.GET(prefix+"/subject-permissions", rh.natsAccHandler.SubjectPermissions)
	// Legacy alias kept so older frontends keep working; static path must stay ahead of {userId}.
	rh.r.GET(prefix+"/nats-users/subject-permissions", rh.natsAccHandler.SubjectPermissions)
	rh.r.POST(prefix+"/nats-users", rh.natsAccHandler.CreateUser)
	rh.r.GET(prefix+"/nats-users/{userId}", rh.natsAccHandler.GetUser)
	rh.r.PUT(prefix+"/nats-users/{userId}", rh.natsAccHandler.UpdateUser)
	rh.r.DELETE(prefix+"/nats-users/{userId}", rh.natsAccHandler.DeleteUser)
	rh.r.GET(prefix+"/nats-users/{userId}/creds", rh.natsAccHandler.DownloadCreds)
	rh.r.POST(prefix+"/nats-users/{userId}/rotate", rh.natsAccHandler.RotateUser)
	rh.r.POST(prefix+"/nats-users/{userId}/mint-jwt", rh.natsAccHandler.MintJWT)
	rh.r.POST(prefix+"/nats-users/{userId}/assign", rh.natsAccHandler.AssignPerson)
	rh.r.GET(prefix+"/signing-groups", rh.natsAccHandler.ListSigningGroups)
	rh.r.POST(prefix+"/signing-groups", rh.natsAccHandler.CreateSigningGroup)
	rh.r.PUT(prefix+"/signing-groups/{groupId}", rh.natsAccHandler.UpdateSigningGroup)
	rh.r.DELETE(prefix+"/signing-groups/{groupId}", rh.natsAccHandler.DeleteSigningGroup)
	rh.r.GET(prefix+"/sharing/exports", rh.natsAccHandler.ListExports)
	rh.r.POST(prefix+"/sharing/exports", rh.natsAccHandler.CreateExport)
	rh.r.PUT(prefix+"/sharing/exports/{exportId}", rh.natsAccHandler.UpdateExport)
	rh.r.DELETE(prefix+"/sharing/exports/{exportId}", rh.natsAccHandler.DeleteExport)

	rh.r.GET(prefix+"/account", rh.jsHandler.AccountInfo)
	rh.r.GET(prefix+"/account/events", rh.jsHandler.AccountOverviewEventsSSE)
	rh.r.GET(prefix+"/metrics/history", rh.metricsHandler.History)
	rh.r.POST(prefix+"/incident-annotations", rh.incidentHandler.CreateAnnotation)
	rh.r.GET(prefix+"/topology", rh.insightsHandler.Topology)
	rh.r.GET(prefix+"/replicas", rh.insightsHandler.Replicas)
	rh.r.GET(prefix+"/replicas/events", rh.insightsHandler.ReplicasEventsSSE)
	rh.r.GET(prefix+"/zombies", rh.insightsHandler.Zombies)
	rh.r.GET(prefix+"/subject-naming", rh.insightsHandler.SubjectNaming)
	rh.r.GET(prefix+"/event-genome", rh.insightsHandler.EventGenome)
	rh.r.GET(prefix+"/architecture-review", rh.insightsHandler.ArchitectureReview)
	rh.r.POST(prefix+"/architecture-review/ask", rh.insightsHandler.ArchitectureReviewAsk)
	rh.r.GET(prefix+"/architecture-refactor", rh.insightsHandler.ArchitectureRefactor)
	rh.r.POST(prefix+"/architecture-refactor/ask", rh.insightsHandler.ArchitectureRefactorAsk)
	rh.r.GET(prefix+"/architecture-score", rh.insightsHandler.ArchitectureScore)
	rh.r.POST(prefix+"/architecture-score/ask", rh.insightsHandler.ArchitectureScoreAsk)
	rh.r.GET(prefix+"/hidden-bottlenecks", rh.insightsHandler.HiddenBottlenecks)
	rh.r.POST(prefix+"/hidden-bottlenecks/ask", rh.insightsHandler.HiddenBottlenecksAsk)
	rh.r.GET(prefix+"/chaos-story", rh.insightsHandler.ChaosStory)
	rh.r.POST(prefix+"/chaos-story/generate", rh.insightsHandler.ChaosStoryGenerate)
	rh.r.GET(prefix+"/architecture-export", rh.insightsHandler.ArchitectureExport)
	rh.r.POST(prefix+"/architecture-export", rh.insightsHandler.ArchitectureExport)
	rh.r.GET(prefix+"/event-catalog", rh.eventCatalogHandler.List)
	rh.r.PUT(prefix+"/event-catalog/{subject}", rh.eventCatalogHandler.Upsert)
	rh.r.DELETE(prefix+"/event-catalog/{subject}", rh.eventCatalogHandler.Delete)
	rh.r.GET(prefix+"/event-wikipedia", rh.eventWikipediaHandler.List)
	rh.r.GET(prefix+"/request-reply", rh.requestReplyHandler.Snapshot)
	rh.r.GET(prefix+"/snapshots/events", rh.insightsHandler.SnapshotEventsSSE)
	rh.r.GET(prefix+"/monitoring/connz/events", rh.insightsHandler.ConnzEventsSSE)
	rh.r.GET(prefix+"/monitoring/varz", rh.insightsHandler.Varz)
	rh.r.GET(prefix+"/monitoring/jsz", rh.insightsHandler.Jsz)
	rh.r.GET(prefix+"/monitoring/{endpoint}", rh.insightsHandler.Monitoring)

	rh.r.GET(prefix+"/streams", rh.jsHandler.ListStreams)
	rh.r.POST(prefix+"/streams", rh.jsHandler.CreateStream)
	rh.r.GET(prefix+"/streams/{name}", rh.jsHandler.GetStream)
	rh.r.PUT(prefix+"/streams/{name}", rh.jsHandler.UpdateStream)
	rh.r.DELETE(prefix+"/streams/{name}", rh.jsHandler.DeleteStream)
	rh.r.GET(prefix+"/streams/{name}/impact", rh.jsHandler.GetStreamImpact)
	rh.r.POST(prefix+"/streams/{name}/purge", rh.jsHandler.PurgeStream)
	rh.r.GET(prefix+"/streams/{name}/consumers", rh.jsHandler.ListConsumers)
	rh.r.POST(prefix+"/streams/{name}/consumers", rh.jsHandler.CreateConsumer)
	rh.r.GET(prefix+"/streams/{name}/consumers/{consumer}", rh.jsHandler.GetConsumer)
	rh.r.PUT(prefix+"/streams/{name}/consumers/{consumer}", rh.jsHandler.UpdateConsumer)
	rh.r.DELETE(prefix+"/streams/{name}/consumers/{consumer}", rh.jsHandler.DeleteConsumer)
	rh.r.GET(prefix+"/streams/{name}/consumers/{consumer}/behavior-fingerprint", rh.jsHandler.GetBehaviorFingerprint)
	rh.r.GET(prefix+"/streams/{name}/consumers/{consumer}/incident-reconstruction", rh.incidentHandler.GetReconstruction)
	rh.r.GET(prefix+"/streams/{name}/consumers/{consumer}/incident-capsules", rh.jsHandler.ListIncidentCapsules)
	rh.r.POST(prefix+"/streams/{name}/consumers/{consumer}/replay/dry-run", rh.jsHandler.ReplayConsumerDryRun)
	rh.r.POST(prefix+"/streams/{name}/consumers/{consumer}/replay", rh.jsHandler.ReplayConsumer)
	rh.r.GET(prefix+"/streams/{name}/messages/range", rh.jsHandler.GetMessageRange)
	rh.r.GET(prefix+"/streams/{name}/messages", rh.jsHandler.GetMessage)
	rh.r.POST(prefix+"/streams/{name}/messages", rh.jsHandler.PublishMessage)
	rh.r.GET(prefix+"/streams/{name}/dlq/messages", rh.jsHandler.ListDLQMessages)
	rh.r.POST(prefix+"/streams/{name}/dlq/retry", rh.jsHandler.RetryDLQMessages)
	rh.r.POST(prefix+"/streams/{name}/dlq/messages/{seq}/capsule", rh.jsHandler.CaptureIncidentCapsuleFromDLQ)
	rh.r.POST(prefix+"/streams/{name}/incident-capsules", rh.jsHandler.CaptureIncidentCapsule)
	rh.r.GET(prefix+"/incident-capsules/{id}", rh.jsHandler.GetIncidentCapsule)
	rh.r.POST(prefix+"/incident-capsules/{id}/replay/dry-run", rh.jsHandler.ReplayIncidentCapsuleDryRun)

	rh.r.GET(prefix+"/live/ws", rh.liveHub.Handle)
	rh.r.POST(prefix+"/assistant/chat", rh.assistantHandler.Chat)
	rh.r.GET(prefix+"/kv/buckets", rh.kvHandler.ListKVBuckets)
	rh.r.POST(prefix+"/kv/buckets", rh.kvHandler.CreateKVBucket)
	rh.r.GET(prefix+"/kv/buckets/{bucket}", rh.kvHandler.GetKVBucket)
	rh.r.PUT(prefix+"/kv/buckets/{bucket}", rh.kvHandler.UpdateKVBucket)
	rh.r.DELETE(prefix+"/kv/buckets/{bucket}", rh.kvHandler.DeleteKVBucket)
	rh.r.GET(prefix+"/kv/buckets/{bucket}/keys", rh.kvHandler.ListKVKeys)
	rh.r.GET(prefix+"/kv/buckets/{bucket}/keys/{key}", rh.kvHandler.GetKVEntry)
	rh.r.PUT(prefix+"/kv/buckets/{bucket}/keys/{key}", rh.kvHandler.PutKVEntry)
	rh.r.DELETE(prefix+"/kv/buckets/{bucket}/keys/{key}", rh.kvHandler.DeleteKVEntry)
	rh.r.GET(prefix+"/kv/buckets/{bucket}/keys/{key}/history", rh.kvHandler.KVHistory)

	rh.r.GET(prefix+"/objects/buckets", rh.kvHandler.ListObjectBuckets)
	rh.r.POST(prefix+"/objects/buckets", rh.kvHandler.CreateObjectBucket)
	rh.r.GET(prefix+"/objects/buckets/{bucket}", rh.kvHandler.GetObjectBucket)
	rh.r.PUT(prefix+"/objects/buckets/{bucket}", rh.kvHandler.UpdateObjectBucket)
	rh.r.DELETE(prefix+"/objects/buckets/{bucket}", rh.kvHandler.DeleteObjectBucket)
	rh.r.GET(prefix+"/objects/buckets/{bucket}/objects", rh.kvHandler.ListObjects)
	rh.r.GET(prefix+"/objects/buckets/{bucket}/objects/{objectName}", rh.kvHandler.GetObject)
	rh.r.PUT(prefix+"/objects/buckets/{bucket}/objects/{objectName}", rh.kvHandler.PutObject)
	rh.r.DELETE(prefix+"/objects/buckets/{bucket}/objects/{objectName}", rh.kvHandler.DeleteObject)

	if !strings.IsEmpty(rh.staticDir) {
		rh.r.NotFound = spa.NewSPAHandler(rh.staticDir).ServeHTTP
	}

	return rh.mw.ApplyRecover(rh.mw.ApplyDebugPprof(
		middleware.Chain(
			rh.mw.ApplyResponseCompression,
			rh.mw.ApplyRequestID,
			rh.mw.ApplySecurityHeaders,
			rh.mw.DecompressRequestBody,
			rh.mw.CheckBodySizeLimit,
			rh.mw.VerifyAuthRateLimit,
			rh.mw.ApplyMetrics,
			rh.mw.ApplyRequestLogger,
			rh.mw.ApplyAITimeout,
			rh.mw.VerifyCors,
			rh.mw.VerifyAudit,
			rh.mw.VerifyCSRF,
			rh.mw.VerifyAuth,
			rh.mw.VerifyRBAC)(rh.r.Handler)))
}
