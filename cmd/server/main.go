package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/gopherust-io/env"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api"
	"github.com/gopherust-io/nats-consol/internal/bootstrap"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/crypto"
	"github.com/gopherust-io/nats-consol/internal/log"
	"github.com/gopherust-io/nats-consol/internal/profiler"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
)

func main() {
	_ = env.LoadDotEnv(".env")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Str("component", "config").Msg("failed to load config")
	}
	if err := cfg.Validate(); err != nil {
		log.Fatal().Err(err).Str("component", "config").Msg("invalid production config")
	}
	log.Init(log.Options{JSON: cfg.LogJSON, Level: cfg.LogLevel})

	encryptor, err := buildEncryptor(cfg)
	if err != nil {
		log.Fatal().Err(err).Str("component", "encryption").Msg("encryption setup failed")
	}

	ctx := context.Background()
	app, err := bootstrap.New(ctx, cfg, encryptor)
	if err != nil {
		log.Fatal().Err(err).Str("component", "bootstrap").Msg("failed to bootstrap application")
	}
	defer app.Close()

	logAssistantEnabled(app, cfg)

	if cfg.PprofContinuous() {
		profiler.StartDefault(profiler.Options{
			Interval: cfg.ContinuousPprofInterval(),
			CPUSlice: cfg.ContinuousPprofCPUSlice(),
		})
		defer profiler.StopDefault()
	}

	metricsCollector := snapshot.Start(app.UoW.Raw(), app.NATSManager, cfg)
	if metricsCollector != nil {
		defer metricsCollector.Stop()
	}

	server := newHTTPServer(cfg, app)
	runUntilSignal(server, cfg.HTTPAddr)
}

func buildEncryptor(cfg config.Config) (*crypto.Encryptor, error) {
	if cfg.EncryptionKey != "" {
		return crypto.New(cfg.EncryptionKey)
	}
	if cfg.IsProduction() {
		return nil, errors.New("ENCRYPTION_KEY is required when ENV=production")
	}
	return nil, nil
}

func logAssistantEnabled(app *bootstrap.Application, cfg config.Config) {
	if app.Assistant == nil {
		return
	}
	log.Info().
		Str("component", "assistant").
		Str("provider", app.Assistant.Provider()).
		Str("model", cfg.AIModel).
		Msg("ai assistant enabled")
}

func newHTTPServer(cfg config.Config, app *bootstrap.Application) *fasthttp.Server {
	return &fasthttp.Server{
		Handler: api.NewRouter(api.RouterDeps{
			Config:      cfg,
			Services:    app.Services,
			AuditWriter: app.AuditWriter,
			Store:       app.UoW.Raw(),
		}),
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		MaxRequestBodySize: cfg.MaxBodyBytes(),
	}
}

func runUntilSignal(server *fasthttp.Server, addr string) {
	go func() {
		log.Info().Str("component", "server").Str("addr", addr).Msg("nats-consol v0.3 listening")
		if err := server.ListenAndServe(addr); err != nil {
			log.Fatal().Err(err).Str("component", "server").Msg("server failed")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	if err := server.Shutdown(); err != nil {
		log.Error().Err(err).Str("component", "server").Msg("shutdown failed")
	}
}
