package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gopherust-io/env"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api"
	"github.com/gopherust-io/nats-consol/internal/audit"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/crypto"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/store"
)

func main() {
	_ = env.LoadDotEnv(".env")

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	setupLogging(cfg)

	var encryptor *crypto.Encryptor
	if cfg.EncryptionKey != "" {
		encryptor, err = crypto.New(cfg.EncryptionKey)
		if err != nil {
			slog.Error("encryption", "error", err)
			os.Exit(1)
		}
	} else if cfg.IsProduction() {
		slog.Error("ENCRYPTION_KEY is required when ENV=production")
		os.Exit(1)
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL, "migrations", encryptor)
	if err != nil {
		slog.Error("store", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	authSvc, err := auth.NewService(cfg, st)
	if err != nil {
		slog.Error("auth", "error", err)
		os.Exit(1)
	}
	if err := authSvc.SeedAdmin(ctx); err != nil {
		slog.Error("seed admin", "error", err)
		os.Exit(1)
	}

	manager := natsclient.NewManager(st, cfg)
	defer manager.Close()

	if err := manager.BootstrapDefaultCluster(ctx); err != nil {
		slog.Error("bootstrap cluster", "error", err)
		os.Exit(1)
	}

	auditWriter := audit.NewWriter(st)

	server := &fasthttp.Server{
		Handler: api.NewRouter(api.RouterDeps{
			Config:      cfg,
			Store:       st,
			NATS:        manager,
			Auth:        authSvc,
			AuditWriter: auditWriter,
		}),
		ReadTimeout:  cfg.RequestTimeout,
		WriteTimeout: cfg.RequestTimeout * 3,
		IdleTimeout:  cfg.RequestTimeout * 6,
	}

	go func() {
		slog.Info("nats-consol v0.3 listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(cfg.HTTPAddr); err != nil {
			slog.Error("server", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	if err := server.Shutdown(); err != nil {
		slog.Error("shutdown", "error", err)
	}
}

func setupLogging(cfg config.Config) {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	var handler slog.Handler
	if cfg.LogJSON {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}
