package config

import (
	"strconv"
	"time"
)

//go:generate envgen -type Config -output config_env_gen.go

type Config struct {
	HTTPAddr       string        `env:"HTTP_ADDR" default:":8080"`
	DatabaseURL    string        `env:"DATABASE_URL" default:"postgres://natsconsol:natsconsol@localhost:5432/natsconsol?sslmode=disable"`
	EncryptionKey  string        `env:"ENCRYPTION_KEY" sensitive:"true"`
	NATSURL        string        `env:"NATS_URL" default:"nats://localhost:4222"`
	NATSCredsFile  string        `env:"NATS_CREDS_FILE"`
	NATSToken      string        `env:"NATS_TOKEN"`
	MonitoringURL  string        `env:"NATS_MONITORING_URL" default:"http://localhost:8222"`
	RequestTimeout time.Duration `env:"REQUEST_TIMEOUT" default:"10s"`
	StaticDir      string        `env:"STATIC_DIR"`
	AdminUsername  string        `env:"ADMIN_USERNAME" default:"admin"`
	AdminPassword  string        `env:"ADMIN_PASSWORD" default:"admin" sensitive:"true"`
	AuthEnabled    bool          `env:"AUTH_ENABLED" default:"true"`
	DefaultClusterName string    `env:"DEFAULT_CLUSTER_NAME" default:"default"`
}

func Load() (Config, error) {
	return LoadConfig()
}

func (c Config) Port() int {
	if len(c.HTTPAddr) > 0 && c.HTTPAddr[0] == ':' {
		if p, err := strconv.Atoi(c.HTTPAddr[1:]); err == nil {
			return p
		}
	}
	return 8080
}
