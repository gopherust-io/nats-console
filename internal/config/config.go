package config

import (
	"strconv"
	"strings"
	"time"
)

//go:generate envgen -type Config -output config_env_gen.go

type Config struct {
	HTTPAddr           string        `env:"HTTP_ADDR" default:":8080"`
	DatabaseURL        string        `env:"DATABASE_URL" default:"postgres://natsconsol:natsconsol@localhost:5432/natsconsol?sslmode=disable"`
	EncryptionKey      string        `env:"ENCRYPTION_KEY" sensitive:"true"`
	NATSURL            string        `env:"NATS_URL" default:"nats://localhost:4222"`
	NATSCredsFile      string        `env:"NATS_CREDS_FILE"`
	NATSToken          string        `env:"NATS_TOKEN"`
	MonitoringURL      string        `env:"NATS_MONITORING_URL" default:"http://localhost:8222"`
	RequestTimeout     time.Duration `env:"REQUEST_TIMEOUT" default:"10s"`
	StaticDir          string        `env:"STATIC_DIR"`
	AdminUsername      string        `env:"ADMIN_USERNAME" default:"admin"`
	AdminPassword      string        `env:"ADMIN_PASSWORD" default:"admin" sensitive:"true"`
	AuthEnabled        bool          `env:"AUTH_ENABLED" default:"true"`
	DefaultClusterName string        `env:"DEFAULT_CLUSTER_NAME" default:"default"`
	Env                string        `env:"ENV" default:"development"`
	CORSAllowedOrigins string        `env:"CORS_ALLOWED_ORIGINS"`
	SessionSecret      string        `env:"SESSION_SECRET" sensitive:"true"`
	SessionTTL         time.Duration `env:"SESSION_TTL" default:"8h"`
	OIDCEnabled        bool          `env:"OIDC_ENABLED" default:"false"`
	OIDCIssuer         string        `env:"OIDC_ISSUER"`
	OIDCClientID       string        `env:"OIDC_CLIENT_ID"`
	OIDCClientSecret   string        `env:"OIDC_CLIENT_SECRET" sensitive:"true"`
	OIDCRedirectURL    string        `env:"OIDC_REDIRECT_URL"`
	BasicAuthEnabled   bool          `env:"BASIC_AUTH_ENABLED" default:"true"`
	MetricsAuthEnabled bool          `env:"METRICS_AUTH_ENABLED" default:"false"`
	LogJSON            bool          `env:"LOG_JSON" default:"false"`
	OpenAPIPath        string        `env:"OPENAPI_PATH" default:"api/openapi.yaml"`
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

func (c Config) IsProduction() bool {
	return strings.EqualFold(c.Env, "production")
}

func (c Config) CORSOrigins() []string {
	if c.CORSAllowedOrigins == "" {
		return nil
	}
	parts := strings.Split(c.CORSAllowedOrigins, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
