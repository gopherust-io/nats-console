package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DEFAULT_CLUSTER_NAME", "")
	t.Setenv("NATS_URL", "")
	t.Setenv("NATS_MONITORING_URL", "")
	t.Setenv("REQUEST_TIMEOUT", "")
	t.Setenv("ADMIN_USERNAME", "")
	t.Setenv("ADMIN_PASSWORD", "")
	t.Setenv("AUTH_ENABLED", "")
	unset := []string{"NATS_CREDS_FILE", "NATS_TOKEN", "STATIC_DIR"}
	for _, key := range unset {
		_ = os.Unsetenv(key)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL != "postgres://natsconsol:natsconsol@localhost:5432/natsconsol?sslmode=disable" {
		t.Fatalf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.DefaultClusterName != "default" {
		t.Fatalf("DefaultClusterName = %q", cfg.DefaultClusterName)
	}
	if cfg.NATSURL != "nats://localhost:4222" {
		t.Fatalf("NATSURL = %q", cfg.NATSURL)
	}
	if cfg.MonitoringURL != "http://localhost:8222" {
		t.Fatalf("MonitoringURL = %q", cfg.MonitoringURL)
	}
	if cfg.RequestTimeout != 10*time.Second {
		t.Fatalf("RequestTimeout = %v", cfg.RequestTimeout)
	}
	if !cfg.AuthEnabled {
		t.Fatal("AuthEnabled should default to true")
	}
	if cfg.AdminUsername != "admin" || cfg.AdminPassword != "admin" {
		t.Fatalf("admin creds = %q / %q", cfg.AdminUsername, cfg.AdminPassword)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("NATS_URL", "nats://example:4222")
	t.Setenv("AUTH_ENABLED", "false")
	t.Setenv("REQUEST_TIMEOUT", "30s")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.NATSURL != "nats://example:4222" {
		t.Fatalf("NATSURL = %q", cfg.NATSURL)
	}
	if cfg.AuthEnabled {
		t.Fatal("AuthEnabled should be false")
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Fatalf("RequestTimeout = %v", cfg.RequestTimeout)
	}
}

func TestMaskedHidesPassword(t *testing.T) {
	cfg := Config{AdminPassword: "secret"}
	masked := cfg.Masked()
	if masked.AdminPassword == "secret" {
		t.Fatal("password should be masked")
	}
}

func TestPort(t *testing.T) {
	if got := (Config{HTTPAddr: ":3000"}).Port(); got != 3000 {
		t.Fatalf("Port() = %d", got)
	}
	if got := (Config{HTTPAddr: "bad"}).Port(); got != 8080 {
		t.Fatalf("Port() fallback = %d", got)
	}
}
