package config

import (
	"errors"
	"strings"
	"time"
)

//go:generate envgen -type Config -output config_env_gen.go

type Config struct {
	OIDCMicrosoftClientSecret string        `env:"OIDC_MICROSOFT_CLIENT_SECRET"                                                   sensitive:"true"`
	HTTPAddr                  string        `default:":8080"                                                                      env:"HTTP_ADDR"`
	AIGeminiAPIBase           string        `default:"https://generativelanguage.googleapis.com/v1beta"                           env:"AI_GEMINI_API_BASE"`
	AIModel                   string        `default:"gemini-2.5-flash"                                                           env:"AI_MODEL"`
	DatabaseURL               string        `default:"postgres://natsconsol:natsconsol@localhost:5432/natsconsol?sslmode=disable" env:"DATABASE_URL"`
	AIAPIKey                  string        `env:"AI_API_KEY"                                                                     sensitive:"true"`
	OpenAPIPath               string        `default:"api/openapi.yaml"                                                           env:"OPENAPI_PATH"`
	LogLevel                  string        `default:"info"                                                                       env:"LOG_LEVEL"`
	OIDCMicrosoftTenant       string        `default:"common"                                                                     env:"OIDC_MICROSOFT_TENANT"`
	OIDCDiscoveryURL          string        `env:"OIDC_DISCOVERY_URL"`
	EncryptionKey             string        `env:"ENCRYPTION_KEY"                                                                 sensitive:"true"`
	NATSURL                   string        `default:"nats://localhost:4222"                                                      env:"NATS_URL"`
	NATSCredsFile             string        `env:"NATS_CREDS_FILE"`
	NATSToken                 string        `env:"NATS_TOKEN"`
	OIDCMicrosoftClientID     string        `env:"OIDC_MICROSOFT_CLIENT_ID"`
	MonitoringURL             string        `default:"http://localhost:8222"                                                      env:"NATS_MONITORING_URL"`
	OIDCGitLabBaseURL         string        `default:"https://gitlab.com"                                                         env:"OIDC_GITLAB_BASE_URL"`
	OIDCGitLabClientSecret    string        `env:"OIDC_GITLAB_CLIENT_SECRET"                                                      sensitive:"true"`
	OIDCGitLabClientID        string        `env:"OIDC_GITLAB_CLIENT_ID"`
	OIDCGitHubClientSecret    string        `env:"OIDC_GITHUB_CLIENT_SECRET"                                                      sensitive:"true"`
	OIDCGitHubClientID        string        `env:"OIDC_GITHUB_CLIENT_ID"`
	OIDCGoogleClientSecret    string        `env:"OIDC_GOOGLE_CLIENT_SECRET"                                                      sensitive:"true"`
	OIDCGoogleClientID        string        `env:"OIDC_GOOGLE_CLIENT_ID"`
	StaticDir                 string        `env:"STATIC_DIR"`
	AdminUsername             string        `default:"admin"                                                                      env:"ADMIN_USERNAME"`
	AdminPassword             string        `default:"admin"                                                                      env:"ADMIN_PASSWORD"             sensitive:"true"`
	PublicBaseURL             string        `default:"http://localhost:8080"                                                      env:"PUBLIC_BASE_URL"`
	DefaultClusterName        string        `default:"default"                                                                    env:"DEFAULT_CLUSTER_NAME"`
	Env                       string        `default:"development"                                                                env:"ENV"`
	CORSAllowedOrigins        string        `env:"CORS_ALLOWED_ORIGINS"`
	SessionSecret             string        `env:"SESSION_SECRET"                                                                 sensitive:"true"`
	OIDCPublicURL             string        `env:"OIDC_PUBLIC_URL"`
	OIDCRedirectURL           string        `env:"OIDC_REDIRECT_URL"`
	OIDCClientSecret          string        `env:"OIDC_CLIENT_SECRET"                                                             sensitive:"true"`
	OIDCIssuer                string        `env:"OIDC_ISSUER"`
	OIDCClientID              string        `env:"OIDC_CLIENT_ID"`
	DBHealthCheckPeriod       time.Duration `default:"1m"                                                                         env:"DB_HEALTH_CHECK_PERIOD"`
	AuditDefaultLimit         int           `default:"50"                                                                         env:"AUDIT_DEFAULT_LIMIT"`
	SessionTTL                time.Duration `default:"8h"                                                                         env:"SESSION_TTL"`
	AuthRateLimitWindow       time.Duration `default:"1m"                                                                         env:"AUTH_RATE_LIMIT_WINDOW"`
	AuthRateLimit             int           `default:"10"                                                                         env:"AUTH_RATE_LIMIT"`
	LiveWSRateLimit           time.Duration `default:"100ms"                                                                      env:"LIVE_WS_RATE_LIMIT"`
	LiveWSIdleTimeout         time.Duration `default:"5m"                                                                         env:"LIVE_WS_IDLE_TIMEOUT"`
	MaxRequestBodySize        int64         `default:"1048576"                                                                    env:"MAX_REQUEST_BODY_SIZE"`
	LiveWSMaxMessages         int           `default:"1000"                                                                       env:"LIVE_WS_MAX_MESSAGES"`
	DBMaxConnIdleTime         time.Duration `default:"30m"                                                                        env:"DB_MAX_CONN_IDLE_TIME"`
	HTTPWriteTimeout          time.Duration `default:"30s"                                                                        env:"HTTP_WRITE_TIMEOUT"`
	PaginationMaxLimit        int           `default:"500"                                                                        env:"PAGINATION_MAX_LIMIT"`
	PaginationDefaultLimit    int           `default:"100"                                                                        env:"PAGINATION_DEFAULT_LIMIT"`
	PprofCPUMaxSeconds        int           `default:"120"                                                                        env:"PPROF_CPU_MAX_SECONDS"`
	RequestTimeout            time.Duration `default:"10s"                                                                        env:"REQUEST_TIMEOUT"`
	AIContextCacheTTL         time.Duration `default:"45s"                                                                        env:"AI_CONTEXT_CACHE_TTL"`
	HTTPReadTimeout           time.Duration `default:"10s"                                                                        env:"HTTP_READ_TIMEOUT"`
	AIRequestTimeout          time.Duration `default:"60s"                                                                        env:"AI_REQUEST_TIMEOUT"`
	AIMaxTokens               int           `default:"4096"                                                                       env:"AI_MAX_TOKENS"`
	NATSClientCacheTTL        time.Duration `default:"5m"                                                                         env:"NATS_CLIENT_CACHE_TTL"`
	HTTPIdleTimeout           time.Duration `default:"60s"                                                                        env:"HTTP_IDLE_TIMEOUT"`
	DBMaxConns                int           `default:"25"                                                                         env:"DB_MAX_CONNS"`
	DBMaxConnLifetime         time.Duration `default:"1h"                                                                         env:"DB_MAX_CONN_LIFETIME"`
	DBMinConns                int           `default:"2"                                                                          env:"DB_MIN_CONNS"`
	AIEnabled                 bool          `default:"false"                                                                      env:"AI_ENABLED"`
	LogJSON                   bool          `default:"false"                                                                      env:"LOG_JSON"`
	MetricsAuthEnabled        bool          `default:"false"                                                                      env:"METRICS_AUTH_ENABLED"`
	PprofEnabled              bool          `default:"false"                                                                      env:"PPROF_ENABLED"`
	PprofAuthEnabled          bool          `default:"true"                                                                       env:"PPROF_AUTH_ENABLED"`
	PprofContinuousEnabled    bool          `default:"true"                                                                       env:"PPROF_CONTINUOUS"`
	PprofContinuousInterval   time.Duration `default:"15s"                                                                        env:"PPROF_CONTINUOUS_INTERVAL"`
	PprofContinuousCPUSlice   time.Duration `default:"5s"                                                                         env:"PPROF_CONTINUOUS_CPU_SLICE"`
	BasicAuthEnabled          bool          `default:"true"                                                                       env:"BASIC_AUTH_ENABLED"`
	OIDCEnabled               bool          `default:"false"                                                                      env:"OIDC_ENABLED"`
	OIDCMicrosoftEnabled      bool          `default:"false"                                                                      env:"OIDC_MICROSOFT_ENABLED"`
	OIDCGitLabEnabled         bool          `default:"false"                                                                      env:"OIDC_GITLAB_ENABLED"`
	OIDCGitHubEnabled         bool          `default:"false"                                                                      env:"OIDC_GITHUB_ENABLED"`
	OIDCGoogleEnabled         bool          `default:"false"                                                                      env:"OIDC_GOOGLE_ENABLED"`
	AuthEnabled               bool          `default:"true"                                                                       env:"AUTH_ENABLED"`
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

func (c Config) OAuthRedirectURL(provider string) string {
	return strings.TrimSuffix(c.oauthPublicBase(), "/") + "/api/v1/auth/oidc/" + provider + "/callback"
}

func (c Config) oauthPublicBase() string {
	if c.OIDCPublicURL != "" {
		return strings.TrimSuffix(c.OIDCPublicURL, "/")
	}
	if c.OIDCRedirectURL != "" {
		if idx := strings.Index(c.OIDCRedirectURL, "/api/v1/auth/oidc"); idx > 0 {
			return c.OIDCRedirectURL[:idx]
		}
	}
	return strings.TrimSuffix(c.PublicBaseURL, "/")
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
	if !c.IsProduction() {
		return nil
	}
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
	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "; "))
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

func (c Config) PprofContinuous() bool {
	return c.PprofEnabled && c.PprofContinuousEnabled
}

func (c Config) ContinuousPprofInterval() time.Duration {
	if c.PprofContinuousInterval <= 0 {
		return 15 * time.Second
	}
	return c.PprofContinuousInterval
}

func (c Config) ContinuousPprofCPUSlice() time.Duration {
	if c.PprofContinuousCPUSlice <= 0 {
		return 5 * time.Second
	}
	return c.PprofContinuousCPUSlice
}
