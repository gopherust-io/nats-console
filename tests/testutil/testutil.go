// Package testutil provides shared helpers for integration, contract, and security tests.
package testutil

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/nats"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"

	natsadapter "github.com/gopherust-io/nats-consol/internal/adapter/nats"
	pgadapter "github.com/gopherust-io/nats-consol/internal/adapter/postgres"
	"github.com/gopherust-io/nats-consol/internal/api"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/store"
)

// Stack holds containers, store, and services for API integration tests.
type Stack struct {
	Store   *store.Store
	Manager *natsclient.Manager
	Cfg     config.Config
}

// RequireDocker skips the test when Docker is unavailable or SKIP_TESTCONTAINERS is set.
func RequireDocker(t *testing.T) {
	t.Helper()
	if os.Getenv("SKIP_TESTCONTAINERS") != "" {
		t.Skip("SKIP_TESTCONTAINERS set")
	}
	if _, err := testcontainers.NewDockerProvider(); err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
}

// MigrationsDir returns the path to SQL migrations relative to the repo root.
func MigrationsDir() string {
	return migrationsDir()
}

func migrationsDir() string {
	for _, dir := range []string{"migrations", filepath.Join("..", "..", "migrations")} {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	return "migrations"
}

// StartPostgres spins up a PostgreSQL testcontainer and returns its connection string.
func StartPostgres(t *testing.T, ctx context.Context) string {
	t.Helper()
	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("natsconsol"),
		postgres.WithUsername("natsconsol"),
		postgres.WithPassword("natsconsol"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	if err != nil {
		t.Fatalf("postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	pgURL, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	return pgURL
}

// NATSEndpoints holds client and monitoring URLs from a NATS testcontainer.
type NATSEndpoints struct {
	ClientURL     string
	MonitoringURL string
}

// StartNATS spins up a NATS testcontainer with JetStream enabled.
func StartNATS(t *testing.T, ctx context.Context) NATSEndpoints {
	t.Helper()
	natsContainer, err := nats.Run(ctx, "nats:2.11-alpine",
		nats.WithArgument("m", "8222"),)
	if err != nil {
		t.Fatalf("nats container: %v", err)
	}
	t.Cleanup(func() { _ = natsContainer.Terminate(ctx) })

	clientURL, err := natsContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	monitoringURL, err := natsContainer.PortEndpoint(ctx, "8222/tcp", "http")
	if err != nil {
		t.Fatal(err)
	}
	return NATSEndpoints{ClientURL: clientURL, MonitoringURL: monitoringURL}
}

// OpenStore opens the store against pgURL and runs migrations.
func OpenStore(t *testing.T, ctx context.Context, pgURL string) *store.Store {
	t.Helper()
	st, err := store.Open(ctx, pgURL, migrationsDir(), nil, store.DefaultPoolConfig())
	if err != nil {
		t.Fatalf("store open: %v", err)
	}
	t.Cleanup(st.Close)
	return st
}

// SetupStack starts postgres + nats, opens the store, and bootstraps a default cluster.
func SetupStack(t *testing.T) *Stack {
	t.Helper()
	RequireDocker(t)
	ctx := context.Background()

	pgURL := StartPostgres(t, ctx)
	natsEP := StartNATS(t, ctx)
	st := OpenStore(t, ctx, pgURL)

	cfg := config.Config{
		NATSURL:            natsEP.ClientURL,
		MonitoringURL:      natsEP.MonitoringURL,
		RequestTimeout:     10 * time.Second,
		AuthEnabled:        false,
		DefaultClusterName: "test",
		AdminUsername:      "admin",
		AdminPassword:      "admin",
		SessionSecret:      "test-session-secret-key",
	}

	manager := natsclient.NewManager(st, cfg)
	t.Cleanup(manager.Close)

	if err := manager.BootstrapDefaultCluster(ctx); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	return &Stack{Store: st, Manager: manager, Cfg: cfg}
}

// DefaultClusterID returns the first cluster ID from the store.
func (s *Stack) DefaultClusterID(t *testing.T) string {
	t.Helper()
	clusters, err := s.Store.ListClusters(context.Background())
	if err != nil || len(clusters) == 0 {
		t.Fatalf("clusters: %v", err)
	}
	return clusters[0].ID
}

// Services builds app services for the stack.
func (s *Stack) Services(t *testing.T) *app.Services {
	t.Helper()
	authSvc, err := auth.NewService(s.Cfg, s.Store)
	if err != nil {
		t.Fatalf("auth service: %v", err)
	}
	gateway := natsadapter.NewGateway(s.Manager)
	uow := pgadapter.WrapStore(s.Store)
	svc := app.NewServices(uow, gateway, authSvc, nil)
	svc.JetStream = gateway
	return svc
}

// Server wraps an in-memory HTTP server backed by fasthttp.
type Server struct {
	Client *http.Client
}

// NewServer starts an in-memory API server with the given config overrides applied to stack cfg.
func (s *Stack) NewServer(t *testing.T, mutate func(*config.Config)) *Server {
	t.Helper()
	cfg := s.Cfg
	if mutate != nil {
		mutate(&cfg)
	}

	authSvc, err := auth.NewService(cfg, s.Store)
	if err != nil {
		t.Fatalf("auth service: %v", err)
	}
	if cfg.AuthEnabled {
		if err := authSvc.SeedAdmin(context.Background()); err != nil {
			t.Fatalf("seed admin: %v", err)
		}
	}

	gateway := natsadapter.NewGateway(s.Manager)
	uow := pgadapter.WrapStore(s.Store)
	services := app.NewServices(uow, gateway, authSvc, nil)
	services.JetStream = gateway

	handler := api.NewRouter(api.RouterDeps{
		Config:   cfg,
		Services: services,
	})

	ln := fasthttputil.NewInmemoryListener()
	server := &fasthttp.Server{Handler: handler}
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(func() { _ = server.Shutdown() })

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return ln.Dial()
			},
		},
	}
	return &Server{Client: client}
}

// BaseURL is a helper for building cluster-scoped paths.
func (s *Server) BaseURL(clusterID string) string {
	return "http://nats-consol.local/api/v1/clusters/" + clusterID
}
