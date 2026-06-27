package api_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/nats"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"

	"github.com/gopherust-io/nats-consol/internal/api"
	"github.com/gopherust-io/nats-consol/internal/config"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/store"
)

func requireDocker(t *testing.T) {
	t.Helper()
	if os.Getenv("SKIP_TESTCONTAINERS") != "" {
		t.Skip("SKIP_TESTCONTAINERS set")
	}
	if _, err := testcontainers.NewDockerProvider(); err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
}
func TestClusterStreamConsumerLifecycle(t *testing.T) {
	requireDocker(t)

	ctx := context.Background()

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

	natsContainer, err := nats.Run(ctx, "nats:2.11-alpine")
	if err != nil {
		t.Fatalf("nats container: %v", err)
	}
	t.Cleanup(func() { _ = natsContainer.Terminate(ctx) })

	natsURL, err := natsContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}

	monitoringURL, err := natsContainer.PortEndpoint(ctx, "8222/tcp", "http")
	if err != nil {
		t.Fatal(err)
	}

	migrationsDir := filepath.Join("..", "..", "migrations")
	if _, err := os.Stat(migrationsDir); err != nil {
		migrationsDir = "migrations"
	}

	st, err := store.Open(ctx, pgURL, migrationsDir)
	if err != nil {
		t.Fatalf("store open: %v", err)
	}
	t.Cleanup(st.Close)

	cfg := config.Config{
		NATSURL:            natsURL,
		MonitoringURL:      monitoringURL,
		RequestTimeout:     10 * time.Second,
		AuthEnabled:        false,
		DefaultClusterName: "test",
	}

	manager := natsclient.NewManager(st, cfg)
	t.Cleanup(manager.Close)

	if err := manager.BootstrapDefaultCluster(ctx); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	clusters, err := st.ListClusters(ctx)
	if err != nil || len(clusters) == 0 {
		t.Fatalf("clusters: %v", err)
	}
	clusterID := clusters[0].ID

	handler := api.NewRouter(cfg, st, manager)
	ln := fasthttputil.NewInmemoryListener()
	server := &fasthttp.Server{Handler: handler}
	go server.Serve(ln)
	t.Cleanup(func() { _ = server.Shutdown() })

	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return ln.Dial()
			},
		},
	}

	base := fmt.Sprintf("http://nats-consol.local/api/v1/clusters/%s", clusterID)

	// Create stream
	createBody := `{"name":"ORDERS","subjects":["orders.>"]}`
	resp, err := client.Post(base+"/streams", "application/json", strings.NewReader(createBody))
	if err != nil {
		t.Fatal(err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create stream: %d %s", resp.StatusCode, string(respBody))
	}

	// Create consumer
	consumerBody := `{"durable_name":"orders-worker","deliver_policy":"all","ack_policy":"explicit"}`
	resp, err = client.Post(base+"/streams/ORDERS/consumers", "application/json", strings.NewReader(consumerBody))
	if err != nil {
		t.Fatal(err)
	}
	respBody, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create consumer: %d %s", resp.StatusCode, string(respBody))
	}

	// List consumers
	resp, err = client.Get(base + "/streams/ORDERS/consumers")
	if err != nil {
		t.Fatal(err)
	}
	respBody, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list consumers: %d", resp.StatusCode)
	}
	var consumerList struct {
		Total int `json:"total"`
	}
	if err := sonic.Unmarshal(respBody, &consumerList); err != nil {
		t.Fatal(err)
	}
	if consumerList.Total != 1 {
		t.Fatalf("consumer total = %d", consumerList.Total)
	}

	// Delete consumer
	req, _ := http.NewRequest(http.MethodDelete, base+"/streams/ORDERS/consumers/orders-worker", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete consumer: %d", resp.StatusCode)
	}
}

func TestHealthEndpoint(t *testing.T) {
	requireDocker(t)

	ctx := context.Background()
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

	migrationsDir := filepath.Join("..", "..", "migrations")
	if _, err := os.Stat(migrationsDir); err != nil {
		migrationsDir = "migrations"
	}

	st, err := store.Open(ctx, pgURL, migrationsDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)

	cfg := config.Config{AuthEnabled: false, RequestTimeout: 5 * time.Second}
	manager := natsclient.NewManager(st, cfg)
	t.Cleanup(manager.Close)

	handler := api.NewRouter(cfg, st, manager)
	ln := fasthttputil.NewInmemoryListener()
	server := &fasthttp.Server{Handler: handler}
	go server.Serve(ln)
	t.Cleanup(func() { _ = server.Shutdown() })

	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return ln.Dial()
			},
		},
	}

	resp, err := client.Get("http://nats-consol.local/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", resp.StatusCode)
	}
}
