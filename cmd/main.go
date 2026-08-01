package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gopherust-io/tel"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api"
	"github.com/gopherust-io/nats-consol/internal/bootstrap"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/mail"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
)

func main() {
	ctx := context.Background()
	telemetry, telShutdown := tel.Init(ctx)
	ctx = tel.WrapContext(ctx, telemetry)

	cfg, err := config.Load()
	if err != nil {
		tel.Fatal().Err(err).Str("component", "config").Msg("failed to load config")
	}
	tel.Info().Str("component", "config").Msg("config successfully initialized")

	app, err := bootstrap.New(ctx, cfg)
	if err != nil {
		tel.Fatal().Err(err).Str("component", "bootstrap").Msg("failed to bootstrap application")
	}

	if app.Assistant != nil {
		tel.Info().
			Str("component", "assistant").
			Str("provider", app.Assistant.Provider()).
			Str("model", cfg.AI.Model).
			Msg("ai assistant enabled")
	}

	mailer, err := mail.NewSenderFromConfig(cfg.SMTP.Enabled, mail.SMTPConfig{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		Username: cfg.SMTP.Username,
		Password: cfg.SMTP.Password,
		From:     cfg.SMTP.From,
		TLS:      cfg.SMTP.TLS,
	})
	if err != nil {
		tel.Fatal().Err(err).Str("component", "mail").Msg("smtp setup failed")
	}
	if cfg.SMTP.Enabled {
		tel.Info().Str("component", "mail").Str("host", cfg.SMTP.Host).Int("port", cfg.SMTP.Port).Msg("alert email delivery enabled")
	}

	hub := snapshot.NewHub()
	metricsCollector, metricsCancelFn := snapshot.Start(app.UoW.Raw(), app.NATSManager, cfg, mailer, hub)
	if metricsCollector == nil {
		_ = hub
	}

	server := &fasthttp.Server{
		Handler:            api.NewRouter(app.Services, app.AuditWriter, hub, cfg).InitRouter(),
		ReadTimeout:        cfg.HTTP.ReadTimeout,
		WriteTimeout:       cfg.HTTP.WriteTimeout,
		IdleTimeout:        cfg.HTTP.IdleTimeout,
		MaxRequestBodySize: int(cfg.HTTP.MaxRequestBodySize),
	}

	go func() {
		tel.Info().Str("component", "server").Str("addr", cfg.HTTP.Addr).Msg("nats-consol listening")
		if err := server.ListenAndServe(cfg.HTTP.Addr); err != nil {
			tel.Fatal().Err(err).Str("component", "server").Msg("server failed")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	tel.Info().Str("component", "server").Msg("shutdown server")
	if err := server.Shutdown(); err != nil {
		tel.Error().Err(err).Str("component", "server").Msg("shutdown failed")
	}

	tel.Info().Str("component", "metrics").Msg("shutdown metrics")
	if metricsCancelFn != nil {
		metricsCancelFn()
	}

	tel.Info().Str("component", "app").Msg("shutdown app")
	app.Close()

	tel.Info().Str("component", "telemetry").Msg("shutdown telemetry")
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, time.Minute)
	if err := telShutdown(shutdownCtx); err != nil {
		tel.Error().Err(err).Str("component", "tel").Msg("telemetry shutdown failed")
	}
	shutdownCancel()
}
