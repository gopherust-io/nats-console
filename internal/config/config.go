package config

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/gopherust-io/nats-consol/internal/crypto"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

//go:generate envgen -type Config -output config_env_gen.go

// goalign:ignore // env-backed; trailing bool padding is unavoidable
type HTTPConfig struct {
	Addr                string        `env:"HTTP_ADDR"                 default:":8080"`
	WriteTimeout        time.Duration `env:"HTTP_WRITE_TIMEOUT"        default:"30s"`
	ReadTimeout         time.Duration `env:"HTTP_READ_TIMEOUT"         default:"10s"`
	IdleTimeout         time.Duration `env:"HTTP_IDLE_TIMEOUT"         default:"60s"`
	MaxRequestBodySize  int           `env:"MAX_REQUEST_BODY_SIZE"     default:"1048576"`
	ResponseCompression bool          `env:"HTTP_RESPONSE_COMPRESSION" default:"true"`
}

type DBConfig struct {
	URL               string        `env:"DATABASE_URL"           required:"true"`
	MaxConnLifetime   time.Duration `env:"DB_MAX_CONN_LIFETIME"   default:"1h"`
	HealthCheckPeriod time.Duration `env:"DB_HEALTH_CHECK_PERIOD" default:"1m"`
	MaxConnIdleTime   time.Duration `env:"DB_MAX_CONN_IDLE_TIME"  default:"30m"`
	MaxConns          int           `env:"DB_MAX_CONNS"           default:"25"`
	MinConns          int           `env:"DB_MIN_CONNS"           default:"2"`
}

// goalign:ignore // env-backed; trailing bool padding is unavoidable
type NATSConfig struct {
	URL                   string        `env:"URL"`
	CredsFile             string        `env:"CREDS_FILE"`
	Token                 string        `env:"TOKEN"`
	AccountSeed           string        `env:"ACCOUNT_SEED"             sensitive:"true"`
	MonitoringURL         string        `env:"MONITORING_URL"`
	TlsCAFile             string        `env:"TLS_CA_FILE"`
	TlsCertFile           string        `env:"TLS_CERT_FILE"`
	TlsKeyFile            string        `env:"TLS_KEY_FILE"`
	TlsServerName         string        `env:"TLS_SERVER_NAME"`
	ClientCacheTTL        time.Duration `env:"CLIENT_CACHE_TTL"         default:"5m"`
	InitialRetryAttempts  int           `env:"INITIAL_RETRY_ATTEMPTS"   default:"0"`
	MaxReconnect          int           `env:"MAX_RECONNECT"            default:"-1"`
	ReconnectWait         time.Duration `env:"RECONNECT_WAIT"           default:"2s"`
	AllowReconnect        bool          `env:"ALLOW_RECONNECT"          default:"true"`
	TlsInsecureSkipVerify bool          `env:"TLS_INSECURE_SKIP_VERIFY" default:"false"`
	DontRandomize         bool          `env:"DONT_RANDOMIZE"           default:"false"`
	AllowMetrics          bool          `env:"ALLOW_METRICS"            default:"true"`
	AllowTracing          bool          `env:"ALLOW_TRACING"            default:"true"`
}

// goalign:ignore // env-backed; trailing bool padding is unavoidable
type AIConfig struct {
	GeminiAPIBase   string        `env:"GEMINI_API_BASE"   default:"https://generativelanguage.googleapis.com/v1beta"`
	Model           string        `env:"MODEL"             default:"gemini-2.5-flash"`
	APIKey          string        `env:"API_KEY"           sensitive:"true"`
	ContextCacheTTL time.Duration `env:"CONTEXT_CACHE_TTL" default:"45s"`
	RequestTimeout  time.Duration `env:"REQUEST_TIMEOUT"   default:"60s"`
	MaxTokens       int           `env:"MAX_TOKENS"        default:"4096"`
	Enabled         bool          `env:"ENABLED"           default:"false"`
}

// goalign:ignore // env-backed; trailing bool padding is unavoidable
type SMTPConfig struct {
	From     string        `env:"FROM"`
	Password string        `env:"PASSWORD" sensitive:"true"`
	Username string        `env:"USERNAME"`
	Host     string        `env:"HOST"     required:"true"`
	Port     int           `env:"PORT"     required:"true"`
	Timeout  time.Duration `env:"TIMEOUT"  default:"15s"`
	Enabled  bool          `env:"ENABLED"  default:"false"`
	TLS      bool          `env:"TLS"      default:"true"`
}

type AuthConfig struct {
	SessionPrivateKey string        `env:"SESSION_PRIVATE_KEY"    required:"true" sensitive:"true"`
	SessionPublicKey  string        `env:"SESSION_PUBLIC_KEY"     required:"true"`
	SessionTTL        time.Duration `env:"SESSION_TTL"            default:"15m"`
	RefreshTokenTTL   time.Duration `env:"REFRESH_TOKEN_TTL"      default:"168h"`
	RateLimitWindow   time.Duration `env:"AUTH_RATE_LIMIT_WINDOW" default:"1m"`
	RateLimit         int           `env:"AUTH_RATE_LIMIT"        default:"10"`
}

type LiveWSConfig struct {
	IdleTimeout          time.Duration `env:"IDLE_TIMEOUT"           default:"5m"`
	RateLimit            time.Duration `env:"RATE_LIMIT"             default:"100ms"`
	MaxMessages          int           `env:"MAX_MESSAGES"           default:"1000"`
	PayloadTruncateBytes int           `env:"PAYLOAD_TRUNCATE_BYTES" default:"4096"`
}

// goalign:ignore // env-backed; trailing bool padding is unavoidable
type MetricsSnapshotConfig struct {
	Interval            time.Duration `env:"INTERVAL"             default:"60s"`
	Retention           time.Duration `env:"RETENTION"            default:"168h"`
	BottleneckRetention time.Duration `env:"BOTTLENECK_RETENTION" default:"672h"`
	CleanupInterval     time.Duration `env:"CLEANUP_INTERVAL"     default:"1h"`
}

// goalign:ignore // env-backed; trailing bool padding is unavoidable
type PprofConfig struct {
	CPUMaxSeconds int  `env:"CPU_MAX_SECONDS" default:"120"`
	AuthEnabled   bool `env:"AUTH_ENABLED"    default:"true"`
	Enabled       bool `env:"ENABLED"         default:"false"`
}

type SlowConsumerConfig struct {
	PendingThreshold uint64  `env:"PENDING_THRESHOLD" default:"1000"`
	LagThreshold     uint64  `env:"LAG_THRESHOLD"     default:"1000"`
	AckPendingRatio  float64 `env:"ACK_PENDING_RATIO" default:"0.9"`
}

type PaginationConfig struct {
	MaxLimit     int `env:"MAX_LIMIT"     default:"500"`
	DefaultLimit int `env:"DEFAULT_LIMIT" default:"100"`
}

// goalign:ignore // env-backed aggregate; nested groups prefer readability over packing
//
//nolint:govet // fieldalignment: env-backed config struct is intentionally grouped
type Config struct {
	ProjectName     string `env:"PROJECT_NAME" required:"true"`
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

	EncryptionKey               string        `env:"ENCRYPTION_KEY"                  required:"true" sensitive:"true"`
	StaticDir                   string        `env:"STATIC_DIR"`
	AdminUsername               string        `env:"ADMIN_USERNAME"                  default:"admin"`
	AdminPassword               string        `env:"ADMIN_PASSWORD"                  required:"true" sensitive:"true"`
	PublicBaseURL               string        `env:"PUBLIC_BASE_URL"                 default:"http://localhost:8080"`
	DefaultClusterName          string        `env:"DEFAULT_CLUSTER_NAME"            default:"default"`
	CORSAllowedOrigins          string        `env:"CORS_ALLOWED_ORIGINS"`
	TrustedProxies              string        `env:"TRUSTED_PROXIES"`
	BehaviorFingerprintKVBucket string        `env:"BEHAVIOR_FINGERPRINT_KV_BUCKET"  default:"nats_consol_fingerprints"`
	RequestTimeout              time.Duration `env:"REQUEST_TIMEOUT"                 default:"10s"`
	LookBackDuration            time.Duration `env:"LOOKBACK_DURATION"`
	HealthCheckTimeout          time.Duration `env:"HEALTH_CHECK_TIMEOUT"            default:"2s"`
	JetStreamViewCacheTTL       time.Duration `env:"JETSTREAM_VIEW_CACHE_TTL"        default:"3s"`
	InviteTTL                   time.Duration `env:"INVITE_TTL"                      default:"24h"`
	MaxMonitoringBodyBytes      int64         `env:"MAX_MONITORING_BODY_BYTES"       default:"8388608"`
	AuditDefaultLimit           int           `env:"AUDIT_DEFAULT_LIMIT"             default:"50"`
	MetricsAuthEnabled          bool          `env:"METRICS_AUTH_ENABLED"            default:"false"`
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

// IsProduction reports whether ENV=production (independent of PUBLIC_BASE_URL scheme).
func IsProduction() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("ENV")), "production")
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
	// Production or HTTPS public URL: refuse the default admin password.
	if (c.TLSEnabled() || IsProduction()) && c.AdminPassword == "admin" {
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
