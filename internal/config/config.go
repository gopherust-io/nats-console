package config

import (
	"errors"
	"strings"
	"time"

	"github.com/gopherust-io/nats-consol/internal/crypto"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

//go:generate envgen -type Config -output config_env_gen.go

type HTTPConfig struct {
	Addr               string        `default:":8080"   env:"HTTP_ADDR"`
	WriteTimeout       time.Duration `default:"30s"     env:"HTTP_WRITE_TIMEOUT"`
	ReadTimeout        time.Duration `default:"10s"     env:"HTTP_READ_TIMEOUT"`
	IdleTimeout        time.Duration `default:"60s"     env:"HTTP_IDLE_TIMEOUT"`
	MaxRequestBodySize int64         `default:"1048576" env:"MAX_REQUEST_BODY_SIZE"`
}

type DBConfig struct {
	URL               string        `env:"DATABASE_URL" required:"true"`
	MaxConnLifetime   time.Duration `default:"1h"       env:"DB_MAX_CONN_LIFETIME"`
	HealthCheckPeriod time.Duration `default:"1m"       env:"DB_HEALTH_CHECK_PERIOD"`
	MaxConnIdleTime   time.Duration `default:"30m"      env:"DB_MAX_CONN_IDLE_TIME"`
	MaxConns          int           `default:"25"       env:"DB_MAX_CONNS"`
	MinConns          int           `default:"2"        env:"DB_MIN_CONNS"`
}

// goalign:ignore // env-backed; trailing bool padding is unavoidable
type NATSConfig struct {
	URL                   string        `env:"URL"`
	CredsFile             string        `env:"CREDS_FILE"`
	Token                 string        `env:"TOKEN"`
	AccountSeed           string        `env:"ACCOUNT_SEED"    sensitive:"true"`
	MonitoringURL         string        `env:"MONITORING_URL"`
	TlsCAFile             string        `env:"TLS_CA_FILE"`
	TlsCertFile           string        `env:"TLS_CERT_FILE"`
	TlsKeyFile            string        `env:"TLS_KEY_FILE"`
	TlsServerName         string        `env:"TLS_SERVER_NAME"`
	ClientCacheTTL        time.Duration `default:"5m"          env:"CLIENT_CACHE_TTL"`
	TlsInsecureSkipVerify bool          `default:"false"       env:"TLS_INSECURE_SKIP_VERIFY"`
}

// goalign:ignore // env-backed; trailing bool padding is unavoidable
type AIConfig struct {
	GeminiAPIBase   string        `default:"https://generativelanguage.googleapis.com/v1beta" env:"GEMINI_API_BASE"`
	Model           string        `default:"gemini-2.5-flash"                                 env:"MODEL"`
	APIKey          string        `env:"API_KEY"                                              sensitive:"true"`
	ContextCacheTTL time.Duration `default:"45s"                                              env:"CONTEXT_CACHE_TTL"`
	RequestTimeout  time.Duration `default:"60s"                                              env:"REQUEST_TIMEOUT"`
	MaxTokens       int           `default:"4096"                                             env:"MAX_TOKENS"`
	Enabled         bool          `default:"false"                                            env:"ENABLED"`
}

// goalign:ignore // env-backed; trailing bool padding is unavoidable
type SMTPConfig struct {
	From     string `env:"FROM"`
	Password string `env:"PASSWORD"  sensitive:"true"`
	Username string `env:"USERNAME"`
	Host     string `env:"HOST"`
	Port     int    `default:"587"   env:"PORT"`
	Enabled  bool   `default:"false" env:"ENABLED"`
	TLS      bool   `default:"true"  env:"TLS"`
}

type AuthConfig struct {
	SessionPrivateKey string        `env:"SESSION_PRIVATE_KEY" required:"true"              sensitive:"true"`
	SessionPublicKey  string        `env:"SESSION_PUBLIC_KEY"  required:"true"`
	SessionTTL        time.Duration `default:"15m"             env:"SESSION_TTL"`
	RefreshTokenTTL   time.Duration `default:"168h"            env:"REFRESH_TOKEN_TTL"`
	RateLimitWindow   time.Duration `default:"1m"              env:"AUTH_RATE_LIMIT_WINDOW"`
	RateLimit         int           `default:"10"              env:"AUTH_RATE_LIMIT"`
}

type LiveWSConfig struct {
	IdleTimeout          time.Duration `default:"5m"    env:"IDLE_TIMEOUT"`
	RateLimit            time.Duration `default:"100ms" env:"RATE_LIMIT"`
	MaxMessages          int           `default:"1000"  env:"MAX_MESSAGES"`
	PayloadTruncateBytes int           `default:"4096"  env:"PAYLOAD_TRUNCATE_BYTES"`
}

// goalign:ignore // env-backed; trailing bool padding is unavoidable
type MetricsSnapshotConfig struct {
	Interval            time.Duration `default:"60s"  env:"INTERVAL"`
	Retention           time.Duration `default:"168h" env:"RETENTION"`
	BottleneckRetention time.Duration `default:"672h" env:"BOTTLENECK_RETENTION"`
	CleanupInterval     time.Duration `default:"1h"   env:"CLEANUP_INTERVAL"`
	Enabled             bool          `default:"true" env:"ENABLED"`
}

// goalign:ignore // env-backed; trailing bool padding is unavoidable
type PprofConfig struct {
	CPUMaxSeconds int  `default:"120"   env:"CPU_MAX_SECONDS"`
	AuthEnabled   bool `default:"true"  env:"AUTH_ENABLED"`
	Enabled       bool `default:"false" env:"ENABLED"`
}

type SlowConsumerConfig struct {
	PendingThreshold uint64  `default:"1000" env:"PENDING_THRESHOLD"`
	LagThreshold     uint64  `default:"1000" env:"LAG_THRESHOLD"`
	AckPendingRatio  float64 `default:"0.9"  env:"ACK_PENDING_RATIO"`
}

type PaginationConfig struct {
	MaxLimit     int `default:"500" env:"MAX_LIMIT"`
	DefaultLimit int `default:"100" env:"DEFAULT_LIMIT"`
}

// goalign:ignore // env-backed aggregate; nested groups prefer readability over packing
//
//nolint:govet // fieldalignment: env-backed config struct is intentionally grouped
type Config struct {
	HTTP            HTTPConfig
	DB              DBConfig
	NATS            NATSConfig `prefix:"NATS_"`
	AI              AIConfig   `prefix:"AI_"`
	SMTP            SMTPConfig `prefix:"SMTP_"`
	Auth            AuthConfig
	LiveWS          LiveWSConfig          `prefix:"LIVE_WS_"`
	MetricsSnapshot MetricsSnapshotConfig `prefix:"METRICS_SNAPSHOT_"`
	Pprof           PprofConfig           `prefix:"PPROF_"`
	SlowConsumer    SlowConsumerConfig    `prefix:"SLOW_CONSUMER_"`
	Pagination      PaginationConfig      `prefix:"PAGINATION_"`

	OpenAPIPath                 string        `default:"api/openapi.yaml"         env:"OPENAPI_PATH"`
	EncryptionKey               string        `env:"ENCRYPTION_KEY"               required:"true"                      sensitive:"true"`
	StaticDir                   string        `env:"STATIC_DIR"`
	AdminUsername               string        `default:"admin"                    env:"ADMIN_USERNAME"`
	AdminPassword               string        `env:"ADMIN_PASSWORD"               required:"true"                      sensitive:"true"`
	PublicBaseURL               string        `default:"http://localhost:8080"    env:"PUBLIC_BASE_URL"`
	DefaultClusterName          string        `default:"default"                  env:"DEFAULT_CLUSTER_NAME"`
	CORSAllowedOrigins          string        `env:"CORS_ALLOWED_ORIGINS"`
	TrustedProxies              string        `env:"TRUSTED_PROXIES"`
	BehaviorFingerprintKVBucket string        `default:"nats_consol_fingerprints" env:"BEHAVIOR_FINGERPRINT_KV_BUCKET"`
	RequestTimeout              time.Duration `default:"10s"                      env:"REQUEST_TIMEOUT"`
	HealthCheckTimeout          time.Duration `default:"2s"                       env:"HEALTH_CHECK_TIMEOUT"`
	JetStreamViewCacheTTL       time.Duration `default:"3s"                       env:"JETSTREAM_VIEW_CACHE_TTL"`
	MaxMonitoringBodyBytes      int64         `default:"8388608"                  env:"MAX_MONITORING_BODY_BYTES"`
	AuditDefaultLimit           int           `default:"50"                       env:"AUDIT_DEFAULT_LIMIT"`
	MetricsAuthEnabled          bool          `default:"false"                    env:"METRICS_AUTH_ENABLED"`
}

func (c Config) TrustedProxyList() []string {
	// TrustedProxies is comma-separated IPs/CIDRs. Empty means no proxy is
	// trusted, so clientIP must ignore X-Forwarded-For and use the remote IP.
	if commonstrings.IsEmpty(strings.TrimSpace(c.TrustedProxies)) {
		return nil
	}
	parts := strings.Split(c.TrustedProxies, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !commonstrings.IsEmpty(p) {
			out = append(out, p)
		}
	}
	return out
}

func (c Config) CORSOrigins() []string {
	if commonstrings.IsEmpty(c.CORSAllowedOrigins) {
		return nil
	}
	parts := strings.Split(c.CORSAllowedOrigins, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !commonstrings.IsEmpty(p) {
			out = append(out, p)
		}
	}
	return out
}

func (c Config) NormalizePaginationLimit(limit int) int {
	return c.clampLimit(limit, c.Pagination.DefaultLimit)
}

func (c Config) NormalizeAuditLimit(limit int) int {
	return c.clampLimit(limit, c.AuditDefaultLimit)
}

func (c Config) clampLimit(limit, defaultLimit int) int {
	if limit <= 0 {
		limit = defaultLimit
	}
	if c.Pagination.MaxLimit > 0 && limit > c.Pagination.MaxLimit {
		limit = c.Pagination.MaxLimit
	}
	return limit
}

func (c Config) AIActive() bool {
	return c.AI.Enabled && !commonstrings.IsEmpty(c.AI.APIKey)
}

func (c Config) TLSEnabled() bool {
	return strings.HasPrefix(strings.ToLower(c.PublicBaseURL), "https://")
}

func (c Config) Validate() error {
	var errs []string

	if _, _, err := crypto.ParseSessionRSAKeyPair(c.Auth.SessionPrivateKey, c.Auth.SessionPublicKey); err != nil {
		errs = append(errs, err.Error())
	}
	if c.SMTP.Enabled {
		errs = append(errs, c.validateSMTP()...)
	}
	errs = append(errs, c.validateSecurity()...)

	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "; "))
}

func (c Config) validateSMTP() []string {
	var errs []string
	if commonstrings.IsEmpty(strings.TrimSpace(c.SMTP.Host)) {
		errs = append(errs, "SMTP_HOST is required when SMTP_ENABLED is true")
	}
	if commonstrings.IsEmpty(strings.TrimSpace(c.SMTP.From)) {
		errs = append(errs, "SMTP_FROM is required when SMTP_ENABLED is true")
	}
	if c.SMTP.Port <= 0 || c.SMTP.Port > 65535 {
		errs = append(errs, "SMTP_PORT must be between 1 and 65535 when SMTP_ENABLED is true")
	}
	return errs
}

func (c Config) validateSecurity() []string {
	var errs []string
	// Match prior deploy behavior: only force a non-default admin password behind HTTPS.
	if c.TLSEnabled() && c.AdminPassword == "admin" {
		errs = append(errs, "ADMIN_PASSWORD must be changed from the default")
	}
	if c.Pprof.Enabled && !c.Pprof.AuthEnabled {
		errs = append(errs, "PPROF_AUTH_ENABLED must be true when PPROF_ENABLED is true")
	}
	if c.Pprof.Enabled && c.HTTP.WriteTimeout < time.Duration(c.Pprof.CPUMaxSeconds)*time.Second+5*time.Second {
		errs = append(errs, "HTTP_WRITE_TIMEOUT must be at least PPROF_CPU_MAX_SECONDS + 5s when PPROF_ENABLED is true")
	}
	return errs
}
