package config

import (
	"errors"
	"strings"
	"time"
)

//go:generate envgen -type Config -output config_env_gen.go

// goalign:ignore
//
//nolint:govet // fieldalignment: env-backed config struct is intentionally flat
type Config struct {
	HTTPAddr                       string        `default:":8080"                                            env:"HTTP_ADDR"`
	AIGeminiAPIBase                string        `default:"https://generativelanguage.googleapis.com/v1beta" env:"AI_GEMINI_API_BASE"`
	AIModel                        string        `default:"gemini-2.5-flash"                                 env:"AI_MODEL"`
	DatabaseURL                    string        `env:"DATABASE_URL"`
	AIAPIKey                       string        `env:"AI_API_KEY"                                           sensitive:"true"`
	OpenAPIPath                    string        `default:"api/openapi.yaml"                                 env:"OPENAPI_PATH"`
	LogLevel                       string        `default:"info"                                             env:"LOG_LEVEL"`
	EncryptionKey                  string        `env:"ENCRYPTION_KEY"                                       sensitive:"true"`
	NATSURL                        string        `env:"NATS_URL"`
	NATSCredsFile                  string        `env:"NATS_CREDS_FILE"`
	NATSToken                      string        `env:"NATS_TOKEN"`
	NATSAccountSeed                string        `env:"NATS_ACCOUNT_SEED"                                    sensitive:"true"`
	NATSTlsCAFile                  string        `env:"NATS_TLS_CA_FILE"`
	NATSTlsCertFile                string        `env:"NATS_TLS_CERT_FILE"`
	NATSTlsKeyFile                 string        `env:"NATS_TLS_KEY_FILE"`
	NATSTlsServerName              string        `env:"NATS_TLS_SERVER_NAME"`
	MonitoringURL                  string        `env:"NATS_MONITORING_URL"`
	StaticDir                      string        `env:"STATIC_DIR"`
	AdminUsername                  string        `default:"admin"                                            env:"ADMIN_USERNAME"`
	AdminPassword                  string        `env:"ADMIN_PASSWORD"                                       sensitive:"true"`
	PublicBaseURL                  string        `default:"http://localhost:8080"                            env:"PUBLIC_BASE_URL"`
	DefaultClusterName             string        `default:"default"                                          env:"DEFAULT_CLUSTER_NAME"`
	Env                            string        `default:"development"                                      env:"ENV"`
	CORSAllowedOrigins             string        `env:"CORS_ALLOWED_ORIGINS"`
	SessionSecret                  string        `env:"SESSION_SECRET"                                       sensitive:"true"`
	SMTPFrom                       string        `env:"SMTP_FROM"`
	SMTPPassword                   string        `env:"SMTP_PASSWORD"                                        sensitive:"true"`
	SMTPUsername                   string        `env:"SMTP_USERNAME"`
	SMTPHost                       string        `env:"SMTP_HOST"`
	HTTP3Addr                      string        `default:":443"                                             env:"HTTP3_ADDR"`
	HTTP3BackendAddrRaw            string        `default:"127.0.0.1:8080"                                   env:"HTTP3_BACKEND_ADDR"`
	HTTP3KeyFile                   string        `env:"HTTP3_KEY_FILE"`
	HTTP3CertFile                  string        `env:"HTTP3_CERT_FILE"`
	PprofCPUMaxSeconds             int           `default:"120"                                              env:"PPROF_CPU_MAX_SECONDS"`
	LiveWSIdleTimeout              time.Duration `default:"5m"                                               env:"LIVE_WS_IDLE_TIMEOUT"`
	HTTPWriteTimeout               time.Duration `default:"30s"                                              env:"HTTP_WRITE_TIMEOUT"`
	PaginationMaxLimit             int           `default:"500"                                              env:"PAGINATION_MAX_LIMIT"`
	PaginationDefaultLimit         int           `default:"100"                                              env:"PAGINATION_DEFAULT_LIMIT"`
	LiveWSMaxMessages              int           `default:"1000"                                             env:"LIVE_WS_MAX_MESSAGES"`
	RequestTimeout                 time.Duration `default:"10s"                                              env:"REQUEST_TIMEOUT"`
	AIContextCacheTTL              time.Duration `default:"45s"                                              env:"AI_CONTEXT_CACHE_TTL"`
	HTTPReadTimeout                time.Duration `default:"10s"                                              env:"HTTP_READ_TIMEOUT"`
	AIRequestTimeout               time.Duration `default:"60s"                                              env:"AI_REQUEST_TIMEOUT"`
	AIMaxTokens                    int           `default:"4096"                                             env:"AI_MAX_TOKENS"`
	NATSClientCacheTTL             time.Duration `default:"5m"                                               env:"NATS_CLIENT_CACHE_TTL"`
	JetStreamViewCacheTTL          time.Duration `default:"3s"                                               env:"JETSTREAM_VIEW_CACHE_TTL"`
	MaxMonitoringBodyBytes         int64         `default:"8388608"                                          env:"MAX_MONITORING_BODY_BYTES"`
	LiveWSPayloadTruncateBytes     int           `default:"4096"                                             env:"LIVE_WS_PAYLOAD_TRUNCATE_BYTES"`
	HTTPIdleTimeout                time.Duration `default:"60s"                                              env:"HTTP_IDLE_TIMEOUT"`
	DBMaxConns                     int           `default:"25"                                               env:"DB_MAX_CONNS"`
	DBMaxConnLifetime              time.Duration `default:"1h"                                               env:"DB_MAX_CONN_LIFETIME"`
	DBMinConns                     int           `default:"2"                                                env:"DB_MIN_CONNS"`
	SMTPPort                       int           `default:"587"                                              env:"SMTP_PORT"`
	DBHealthCheckPeriod            time.Duration `default:"1m"                                               env:"DB_HEALTH_CHECK_PERIOD"`
	AuditDefaultLimit              int           `default:"50"                                               env:"AUDIT_DEFAULT_LIMIT"`
	SessionTTL                     time.Duration `default:"8h"                                               env:"SESSION_TTL"`
	AuthRateLimitWindow            time.Duration `default:"1m"                                               env:"AUTH_RATE_LIMIT_WINDOW"`
	MetricsSnapshotInterval        time.Duration `default:"60s"                                              env:"METRICS_SNAPSHOT_INTERVAL"`
	MetricsSnapshotRetention       time.Duration `default:"168h"                                             env:"METRICS_SNAPSHOT_RETENTION"`
	MetricsSnapshotCleanupInterval time.Duration `default:"1h"                                               env:"METRICS_SNAPSHOT_CLEANUP_INTERVAL"`
	AuthRateLimit                  int           `default:"10"                                               env:"AUTH_RATE_LIMIT"`
	LiveWSRateLimit                time.Duration `default:"100ms"                                            env:"LIVE_WS_RATE_LIMIT"`
	DBMaxConnIdleTime              time.Duration `default:"30m"                                              env:"DB_MAX_CONN_IDLE_TIME"`
	MaxRequestBodySize             int64         `default:"1048576"                                          env:"MAX_REQUEST_BODY_SIZE"`
	SMTPEnabled                    bool          `default:"false"                                            env:"SMTP_ENABLED"`
	MetricsSnapshotEnabled         bool          `default:"true"                                             env:"METRICS_SNAPSHOT_ENABLED"`
	AuthEnabled                    bool          `default:"true"                                             env:"AUTH_ENABLED"`
	HTTP3OutboundEnabled           bool          `default:"true"                                             env:"HTTP3_OUTBOUND_ENABLED"`
	HTTP3OutboundFallback          bool          `default:"true"                                             env:"HTTP3_OUTBOUND_FALLBACK"`
	HTTP3Enabled                   bool          `default:"false"                                            env:"HTTP3_ENABLED"`
	NATSTlsInsecureSkipVerify      bool          `default:"false"                                            env:"NATS_TLS_INSECURE_SKIP_VERIFY"`
	PprofAuthEnabled               bool          `default:"true"                                             env:"PPROF_AUTH_ENABLED"`
	PprofEnabled                   bool          `default:"false"                                            env:"PPROF_ENABLED"`
	MetricsAuthEnabled             bool          `default:"false"                                            env:"METRICS_AUTH_ENABLED"`
	LogJSON                        bool          `default:"false"                                            env:"LOG_JSON"`
	AIEnabled                      bool          `default:"false"                                            env:"AI_ENABLED"`
	SMTPTLS                        bool          `default:"true"                                             env:"SMTP_TLS"`
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

func (c Config) NormalizePaginationLimit(limit int) int {
	return c.clampLimit(limit, c.PaginationDefaultLimit)
}

func (c Config) NormalizeAuditLimit(limit int) int {
	return c.clampLimit(limit, c.AuditDefaultLimit)
}

func (c Config) clampLimit(limit, defaultLimit int) int {
	if limit <= 0 {
		limit = defaultLimit
	}
	if c.PaginationMaxLimit > 0 && limit > c.PaginationMaxLimit {
		limit = c.PaginationMaxLimit
	}
	return limit
}

func (c Config) AIActive() bool {
	return c.AIEnabled && c.AIAPIKey != ""
}

func (c Config) TLSEnabled() bool {
	return strings.HasPrefix(strings.ToLower(c.PublicBaseURL), "https://")
}

func (c Config) Validate() error {
	var errs []string

	if strings.TrimSpace(c.DatabaseURL) == "" {
		errs = append(errs, "DATABASE_URL is required")
	}
	if c.AuthEnabled && strings.TrimSpace(c.AdminPassword) == "" {
		errs = append(errs, "ADMIN_PASSWORD is required when AUTH_ENABLED is true")
	}
	if c.SMTPEnabled {
		errs = append(errs, c.validateSMTP()...)
	}

	if c.IsProduction() {
		errs = append(errs, c.validateProduction()...)
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "; "))
}

func (c Config) validateSMTP() []string {
	var errs []string
	if strings.TrimSpace(c.SMTPHost) == "" {
		errs = append(errs, "SMTP_HOST is required when SMTP_ENABLED is true")
	}
	if strings.TrimSpace(c.SMTPFrom) == "" {
		errs = append(errs, "SMTP_FROM is required when SMTP_ENABLED is true")
	}
	if c.SMTPPort <= 0 || c.SMTPPort > 65535 {
		errs = append(errs, "SMTP_PORT must be between 1 and 65535 when SMTP_ENABLED is true")
	}
	return errs
}

func (c Config) validateProduction() []string {
	var errs []string
	if c.EncryptionKey == "" {
		errs = append(errs, "ENCRYPTION_KEY is required when ENV=production")
	}
	if c.SessionSecret == "" {
		errs = append(errs, "SESSION_SECRET is required when ENV=production")
	}
	if !c.AuthEnabled {
		errs = append(errs, "AUTH_ENABLED must be true when ENV=production")
	}
	if c.AdminPassword == "admin" {
		errs = append(errs, "ADMIN_PASSWORD must be changed when ENV=production")
	}
	if c.PprofEnabled && !c.PprofAuthEnabled {
		errs = append(errs, "PPROF_AUTH_ENABLED must be true when ENV=production and PPROF_ENABLED is true")
	}
	if c.PprofEnabled && c.HTTPWriteTimeout < time.Duration(c.MaxPprofCPUSecs())*time.Second+5*time.Second {
		errs = append(errs, "HTTP_WRITE_TIMEOUT must be at least PPROF_CPU_MAX_SECONDS + 5s when PPROF_ENABLED is true")
	}
	if c.NATSTlsInsecureSkipVerify {
		errs = append(errs, "NATS_TLS_INSECURE_SKIP_VERIFY must be false when ENV=production")
	}
	if c.DatabaseURL != "" {
		if err := validatePostgresSSLMode(c.DatabaseURL, true); err != "" {
			errs = append(errs, err)
		}
	}
	if c.NATSURL != "" {
		if err := validateSecureNATSURL(c.NATSURL, true); err != "" {
			errs = append(errs, err)
		}
		if c.NATSCredsFile == "" && c.NATSToken == "" {
			errs = append(errs, "NATS_CREDS_FILE or NATS_TOKEN is required when ENV=production and NATS_URL is set")
		}
	}
	if c.MonitoringURL != "" {
		if err := validateSecureMonitoringURL(c.MonitoringURL, true); err != "" {
			errs = append(errs, err)
		}
	}
	return errs
}

func (c Config) MaxBodyBytes() int {
	if c.MaxRequestBodySize <= 0 {
		return 1 << 20
	}
	return int(c.MaxRequestBodySize)
}

func (c Config) AuthRateLimitPerWindow() int {
	if c.AuthRateLimit <= 0 {
		return 10
	}
	return c.AuthRateLimit
}

func (c Config) AuthRateLimitDuration() time.Duration {
	if c.AuthRateLimitWindow <= 0 {
		return time.Minute
	}
	return c.AuthRateLimitWindow
}

func (c Config) MaxPprofCPUSecs() int {
	if c.PprofCPUMaxSeconds <= 0 {
		return 120
	}
	return c.PprofCPUMaxSeconds
}

func (c Config) MetricsSnapshotActive() bool {
	return c.MetricsSnapshotEnabled
}

func (c Config) SnapshotInterval() time.Duration {
	if c.MetricsSnapshotInterval <= 0 {
		return 60 * time.Second
	}
	return c.MetricsSnapshotInterval
}

func (c Config) SnapshotRetention() time.Duration {
	if c.MetricsSnapshotRetention <= 0 {
		return 168 * time.Hour
	}
	return c.MetricsSnapshotRetention
}

func (c Config) SnapshotCleanupInterval() time.Duration {
	if c.MetricsSnapshotCleanupInterval <= 0 {
		return time.Hour
	}
	return c.MetricsSnapshotCleanupInterval
}

func (c Config) MaxMonitoringBytes() int64 {
	if c.MaxMonitoringBodyBytes <= 0 {
		return 8 << 20
	}
	return c.MaxMonitoringBodyBytes
}

func (c Config) LivePayloadTruncate() int {
	if c.LiveWSPayloadTruncateBytes < 0 {
		return 0
	}
	if c.LiveWSPayloadTruncateBytes == 0 {
		return 4096
	}
	return c.LiveWSPayloadTruncateBytes
}

func (c Config) HTTP3BackendAddr() string {
	if c.HTTP3BackendAddrRaw == "" {
		return "127.0.0.1:8080"
	}
	return c.HTTP3BackendAddrRaw
}
