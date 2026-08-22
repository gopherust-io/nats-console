package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gopherust-io/tel"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/bootstrap"
)

// NATS Consol API.
//
// @title NATS Consol API
// @version 0.14.0
// @description Self-hosted admin console for NATS JetStream — cluster-scoped REST, WebSocket, and SSE API.
// @BasePath /
//
// @securityDefinitions.basic BasicAuth
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description RS256 session JWT as `Bearer <token>` (same claims as the session cookie).
//
// @securityDefinitions.apikey SessionCookie
// @in cookie
// @name nats_consol_session
// @description HttpOnly RS256 session JWT cookie set by login.
func main() {
	ctx := context.Background()
	telemetry, telStop := tel.Init(ctx)
	ctx = tel.WrapContext(ctx, telemetry)

	app, err := bootstrap.New(ctx)
	if err != nil {
		tel.Fatal().Err(err).Str("component", "bootstrap").Msg("failed to bootstrap app")
	}

	cfg := app.Cfg
	server := &fasthttp.Server{
		Handler:            app.Handler(),
		ReadTimeout:        cfg.HTTP.ReadTimeout,
		WriteTimeout:       cfg.HTTP.WriteTimeout,
		IdleTimeout:        cfg.HTTP.IdleTimeout,
		MaxRequestBodySize: cfg.HTTP.MaxRequestBodySize,
	}

	go func() {
		tel.Info().Str("component", "server").Str("addr", cfg.HTTP.Addr).Msg("nats-console listening")
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

	_ = app.Close()

	tel.Info().Str("component", "telemetry").Msg("stop telemetry")
	stopCtx, stopCancel := context.WithTimeout(ctx, 5*time.Second)
	if err = telStop(stopCtx); err != nil {
		tel.Error().Err(err).Str("component", "tel").Msg("telemetry stop failed")
	}
	stopCancel()
	// GracefulPause is the short settle delay after Close used by main.
	time.Sleep(500 * time.Millisecond)
}
