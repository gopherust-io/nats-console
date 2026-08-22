package bootstrap

import (
	"context"

	"github.com/gopherust-io/tel"
	"github.com/valyala/fasthttp"

	natsadapter "github.com/gopherust-io/nats-consol/internal/adapter/nats"
	"github.com/gopherust-io/nats-consol/internal/adapter/postgres"
	"github.com/gopherust-io/nats-consol/internal/api"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/internal/audit"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/crypto"
	"github.com/gopherust-io/nats-consol/internal/mail"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
)

// App holds the wired runtime dependencies for the HTTP server.
type App struct {
	Cfg      config.Config
	DB       *postgres.DB
	Auth     *auth.Service
	Gateway  port.ClusterGateway
	Services *app.Services
	Audit    *audit.Writer
	Metrics  *snapshot.Collector
	SMTP     mail.Sender

	handler fasthttp.RequestHandler
}

// New loads config and wires adapters, services, and the HTTP handler.
// Failures use the same tel.Fatal messages as the former cmd/main wiring.
func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		tel.Fatal().Err(err).Str("component", "config").Msg("failed to load config")
	}
	tel.Info().Str("component", "config").Msg("config successfully initialized")

	encryptor, err := crypto.New(cfg.EncryptionKey)
	if err != nil {
		tel.Fatal().Err(err).Str("component", "encryptor").Msg("failed to init encryptor")
	}

	db, err := postgres.OpenWithConfig(ctx, cfg, encryptor)
	if err != nil {
		tel.Fatal().Err(err).Str("component", "db").Msg("failed to init db")
	}

	if cfg.TLSEnabled() || config.IsProduction() {
		if err := natsclient.ValidateEnvConfig(cfg); err != nil {
			db.Stop()
			tel.Fatal().Err(err).Str("component", "NATS").Msg("failed to validate NATS config")
		}
	}

	authSvc, err := auth.NewService(cfg, db.DB())
	if err != nil {
		db.Stop()
		tel.Fatal().Err(err).Str("component", "authService").Msg("failed to init auth service")
	}

	err = authSvc.SeedAdmin(ctx)
	if err != nil {
		db.Stop()
		tel.Fatal().Err(err).Str("component", "authService").Msg("failed to seed admin")
	}

	manager := natsclient.NewManager(db.DB(), cfg)
	go manager.StartSweeper(ctx)

	gateway := natsadapter.NewGateway(manager)
	services := app.NewServices(db, gateway, authSvc, nil, cfg.HealthCheckTimeout, cfg.LookBackDuration)
	services.Bottlenecks = app.NewBottleneckService(db, cfg.MetricsSnapshot.BottleneckRetention)

	err = services.Cluster.BootstrapDefault(ctx)
	if err != nil {
		db.Stop()
		manager.Stop()
		tel.Fatal().Err(err).Str("component", "cluster").Msg("failed to init default cluster")
	}

	assistantSvc, err := assistant.NewService(cfg, db.DB(), manager)
	if err != nil {
		db.Stop()
		manager.Stop()
		tel.Fatal().Err(err).Str("component", "assistant").Msg("failed to init assistant")
	}
	services.Assistant = assistantSvc

	if services.Assistant != nil {
		tel.Info().Str("component", "assistant").
			Str("provider", services.Assistant.Provider()+":"+cfg.AI.Model).
			Msg("ai assistant successfully initialized")
	}

	smtpSender, err := mail.NewSMTPSenderFromConfig(ctx, cfg)
	if err != nil {
		tel.Fatal().Err(err).Str("component", "bootstrap").Msg("failed to init smtpSender")
	}
	if smtpSender == nil {
		tel.Info().Str("component", "smtpSender").Msg("smtp disabled; alert email delivery off")
	} else {
		tel.Info().Str("component", "smtpSender").Msg("smtpSender successfully initialized")
	}

	natsMetrics := snapshot.NewSnapshot(db.DB(), manager, cfg, smtpSender)
	go natsMetrics.Start(ctx)
	go natsMetrics.StartCleanup(ctx)
	tel.Info().Str("component", "natsMetrics").Msg("natsMetrics successfully initialized")

	services.ConfigureMonitoring(cfg.MaxMonitoringBodyBytes, cfg.JetStreamViewCacheTTL)
	services.SetSnapshotHub(natsMetrics.Hub())

	auditWriter := audit.NewWriter(db.DB())
	go auditWriter.Start(ctx)

	handler := api.NewRouter(cfg, services, auditWriter, natsMetrics.Hub()).Init()

	return &App{
		Cfg:      cfg,
		DB:       db,
		Auth:     authSvc,
		Gateway:  gateway,
		Services: services,
		Audit:    auditWriter,
		Metrics:  natsMetrics,
		SMTP:     smtpSender,
		handler:  handler,
	}, nil
}

// Handler returns the FastHTTP request handler.
func (a *App) Handler() fasthttp.RequestHandler {
	return a.handler
}

// Close stops background workers and closes adapters. HTTP server and
// telemetry shutdown remain the caller's responsibility.
func (a *App) Close() error {
	if a == nil {
		return nil
	}

	tel.Info().Str("component", "natsMetrics").Msg("stop NATS metrics")
	if a.Metrics != nil {
		a.Metrics.Stop()
	}

	tel.Info().Str("component", "mailer").Msg("stop audit writer")
	if a.Audit != nil {
		a.Audit.Stop()
	}

	tel.Info().Str("component", "gateway").Msg("stop gateway")
	if a.Gateway != nil {
		a.Gateway.Stop()
	}

	tel.Info().Str("component", "db").Msg("stop db")
	if a.DB != nil {
		a.DB.Stop()
	}

	tel.Info().Str("component", "mailer").Msg("stop mailer")
	if a.SMTP != nil {
		if err := a.SMTP.Stop(); err != nil {
			tel.Error().Err(err).Str("component", "mailer").Msg("mailer stop failed")
		}
	}

	return nil
}
