package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	libnats "github.com/gopherust-io/nats"
	"github.com/gopherust-io/tel"
	"github.com/rs/zerolog"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	telem := mustTelemetry(ctx)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := telem.Shutdown(shutdownCtx); err != nil {
			zerolog.Ctx(shutdownCtx).Error().Err(err).Msg("telemetry shutdown")
		}
	}()
	ctx = tel.WrapContext(ctx, telem)
	log := zerolog.Ctx(ctx)

	service := fleetServiceName()
	cfg := buildConfigForService(service)
	client, err := libnats.NewClient(ctx, &cfg)
	if err != nil {
		log.Error().Err(err).Msg("nats client")
		os.Exit(1)
	}
	defer func() {
		fleetExtra.shutdown()
		if err := client.Connector().Shutdown(); err != nil {
			log.Error().Err(err).Msg("nats shutdown")
		}
	}()

	if err := ensureTopology(ctx, client); err != nil {
		log.Error().Err(err).Msg("ensure topology")
		os.Exit(1)
	}

	log.Info().
		Str("service", service).
		Str("client_name", cfg.Conn.ClientName).
		Str("nats_url", cfg.Conn.Address).
		Float64("rate_scale", rateScale()).
		Int("services", len(allServices())).
		Bool("multi_service", multiServiceMode()).
		Msg("nats fleet starting")

	if err := startSelectedServices(ctx, client, service); err != nil {
		log.Error().Err(err).Msg("start services")
		os.Exit(1)
	}

	<-ctx.Done()
	log.Info().Msg("fleet shutting down")
}

func mustTelemetry(ctx context.Context) *tel.Telemetry {
	cfg := tel.DefaultConfig()
	cfg.Service = clientNameFor(fleetServiceName())
	cfg.Version = envOr("SERVICE_VERSION", "1.0.0")
	cfg.Environment = envOr("ENVIRONMENT", "dev")

	switch os.Getenv("TEL_ENABLE") {
	case "false":
		cfg.TelConfig.Enable = false
	case "true":
		cfg.TelConfig.Enable = true
	}

	cfg.MonitorConfig.Enable = false
	cfg.LogEncode = "console"
	cfg.LogLevel = "info"

	tel.ConfigureLogger(cfg)
	telem := tel.NewWithConfig(cfg)
	tel.SetGlobal(telem)

	if err := telem.Start(ctx); err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Msg("telemetry start")
		os.Exit(1)
	}

	zerolog.Ctx(ctx).Info().
		Str("service", cfg.Service).
		Bool("otlp_enabled", cfg.TelConfig.Enable).
		Msg("telemetry ready")

	return telem
}
