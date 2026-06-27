package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gopherust-io/env"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api"
	"github.com/gopherust-io/nats-consol/internal/config"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/store"
)

func main() {
	_ = env.LoadDotEnv(".env")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL, "migrations")
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	manager := natsclient.NewManager(st, cfg)
	defer manager.Close()

	if err := manager.BootstrapDefaultCluster(ctx); err != nil {
		log.Fatalf("bootstrap cluster: %v", err)
	}

	server := &fasthttp.Server{
		Handler:      api.NewRouter(cfg, st, manager),
		ReadTimeout:  cfg.RequestTimeout,
		WriteTimeout: cfg.RequestTimeout * 3,
		IdleTimeout:  cfg.RequestTimeout * 6,
	}

	go func() {
		log.Printf("nats-consol v0.2 listening on %s", cfg.HTTPAddr)
		log.Printf("database: %s", cfg.DatabaseURL)
		if err := server.ListenAndServe(cfg.HTTPAddr); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	if err := server.Shutdown(); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
