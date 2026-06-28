package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/store"
)

type contextKey string

const (
	SessionCookie            = "nats_consol_session"
	CSRFCookie               = "nats_consol_csrf"
	ContextUser   contextKey = "auth_user"
)

var (
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrUnknownProvider = errors.New("unknown sso provider")
)

type Claims struct {
	jwt.RegisteredClaims

	UserID   string   `json:"uid"`
	Username string   `json:"usr"`
	Roles    []string `json:"roles"`
	IsRoot   bool     `json:"isRoot"`
}

type Service struct {
	store     *store.Store
	providers map[string]*oauthProvider
	cfg       config.Config
}

func NewService(cfg config.Config, st *store.Store) (*Service, error) {
	providers, err := buildProviders(cfg)
	if err != nil {
		return nil, err
	}
	return &Service{cfg: cfg, store: st, providers: providers}, nil
}

func (s *Service) SeedAdmin(ctx context.Context) error {
	count, err := s.store.CountUsers(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = s.store.CreateUser(ctx, store.UserCreate{
		Username: s.cfg.AdminUsername,
		Email:    s.cfg.AdminUsername + "@local",
		Password: s.cfg.AdminPassword,
		Roles:    []string{store.RoleAdmin},
		IsRoot:   true,
	})
	return err
}

func (s *Service) LoadUser(ctx context.Context, userID string) (store.User, error) {
	if userID == "" {
		return store.User{}, ErrUnauthorized
	}
	return s.store.GetUserByID(ctx, userID)
}

func (s *Service) AuthenticateBasic(ctx context.Context, username, password string) (store.User, error) {
	user, hash, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) && username == s.cfg.AdminUsername && password == s.cfg.AdminPassword {
			return store.User{
				Username: username,
				Roles:    []string{store.RoleAdmin},
				IsRoot:   true,
			}, nil
		}
		return store.User{}, ErrUnauthorized
	}
	if hash == "" || !store.CheckPassword(hash, password) {
		return store.User{}, ErrUnauthorized
	}
	return user, nil
}

func (s *Service) CreateSession(user store.User) (string, error) {
	secret, err := s.sessionSecret()
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Roles:    user.Roles,
		IsRoot:   user.IsRoot,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.SessionTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   user.Username,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func (s *Service) ParseSession(tokenStr string) (store.User, error) {
	secret, err := s.sessionSecret()
	if err != nil {
		return store.User{}, ErrUnauthorized
	}
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil || !token.Valid {
		return store.User{}, ErrUnauthorized
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return store.User{}, ErrUnauthorized
	}
	return store.User{
		ID:       claims.UserID,
		Username: claims.Username,
		Roles:    claims.Roles,
		IsRoot:   claims.IsRoot,
	}, nil
}

func (s *Service) OIDCEnabled() bool {
	return len(s.providers) > 0
}

func (s *Service) SSOProviders() []ProviderInfo {
	order := []string{ProviderGoogle, ProviderGitHub, ProviderGitLab, ProviderMicrosoft, ProviderLegacy}
	out := make([]ProviderInfo, 0, len(s.providers))
	for _, id := range order {
		if p, ok := s.providers[id]; ok {
			out = append(out, ProviderInfo{ID: p.id, Name: p.name})
		}
	}
	return out
}

func (s *Service) provider(id string) (*oauthProvider, error) {
	p, ok := s.providers[id]
	if !ok {
		return nil, ErrUnknownProvider
	}
	return p, nil
}

func (s *Service) AuthEnabled() bool {
	return s.cfg.AuthEnabled
}

func (s *Service) BasicAuthEnabled() bool {
	if !s.cfg.AuthEnabled {
		return false
	}
	if !s.OIDCEnabled() {
		return true
	}
	return s.cfg.BasicAuthEnabled
}

func (s *Service) SSOAuthURL(providerID, state string) (string, error) {
	p, err := s.provider(providerID)
	if err != nil {
		return "", err
	}
	return p.authURL(state), nil
}

func (s *Service) HandleSSOCallback(ctx context.Context, providerID, code string) (store.User, error) {
	p, err := s.provider(providerID)
	if err != nil {
		return store.User{}, err
	}

	var username, email, sub string
	if p.github {
		username, email, sub, err = p.userFromGitHub(ctx, code)
	} else {
		username, email, sub, err = p.userFromOIDC(ctx, code)
	}
	if err != nil {
		return store.User{}, err
	}

	user, err := s.store.GetUserByOIDCSub(ctx, sub)
	if errors.Is(err, store.ErrNotFound) {
		clusterIDs, listErr := s.defaultClusterIDs(ctx)
		if listErr != nil {
			return store.User{}, listErr
		}
		var accessRules *store.AccessRules
		if len(clusterIDs) > 0 {
			accessRules = &store.AccessRules{ClusterIDs: clusterIDs}
		}
		user, err = s.store.CreateUser(ctx, store.UserCreate{
			Username:    username,
			Email:       email,
			OIDCSub:     sub,
			Roles:       []string{store.RoleViewer},
			AccessRules: accessRules,
		})
	}
	if err != nil {
		return store.User{}, err
	}
	return user, nil
}

func (s *Service) SessionCookie(token string) *http.Cookie {
	return s.newCookie(SessionCookie, token, int(s.cfg.SessionTTL.Seconds()), true)
}

func (s *Service) ClearSessionCookie() *http.Cookie {
	return s.newCookie(SessionCookie, "", -1, true)
}

func (s *Service) CSRFCookie(token string) *http.Cookie {
	return s.newCookie(CSRFCookie, token, int(s.cfg.SessionTTL.Seconds()), false)
}

func (s *Service) ClearCSRFCookie() *http.Cookie {
	return s.newCookie(CSRFCookie, "", -1, false)
}

func (s *Service) NewCSRFToken() (string, error) {
	return NewOAuthState()
}

func (s *Service) OAuthStateCookie(state string) *http.Cookie {
	return s.newCookie("nats_consol_oauth_state", state, 600, true)
}

func (s *Service) OAuthProviderCookie(provider string) *http.Cookie {
	return s.newCookie("nats_consol_oauth_provider", provider, 600, true)
}

func (s *Service) ClearOAuthStateCookie() *http.Cookie {
	return s.newCookie("nats_consol_oauth_state", "", -1, true)
}

func (s *Service) ClearOAuthProviderCookie() *http.Cookie {
	return s.newCookie("nats_consol_oauth_provider", "", -1, true)
}

func (s *Service) newCookie(name, value string, maxAge int, httpOnly bool) *http.Cookie {
	secure := s.cfg.IsProduction() || s.cfg.TLSEnabled()
	return &http.Cookie{ //nolint:gosec // G124: Secure/HttpOnly/SameSite set from config below
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: httpOnly,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}

func (s *Service) sessionSecret() ([]byte, error) {
	key := s.cfg.SessionSecret
	if key == "" {
		key = s.cfg.EncryptionKey
	}
	if key == "" {
		key = s.cfg.AdminPassword
	}
	if len(key) < 16 {
		return nil, errors.New("session secret must be at least 16 characters")
	}
	return []byte(key), nil
}

func NewOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func ParseBasicAuth(header string) (username, password string, ok bool) {
	if !strings.HasPrefix(header, "Basic ") {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(header[6:])
	if err != nil {
		return "", "", false
	}
	user, pass, found := strings.Cut(string(decoded), ":")
	return user, pass, found
}

func UserFromContext(ctx context.Context) (store.User, bool) {
	if v, ok := ctx.Value(ContextUser).(store.User); ok {
		return v, true
	}
	return store.User{}, false
}

func ContextWithUser(ctx context.Context, user store.User) context.Context {
	return context.WithValue(ctx, ContextUser, user)
}

func (s *Service) defaultClusterIDs(ctx context.Context) ([]string, error) {
	clusters, err := s.store.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		ids = append(ids, cluster.ID)
	}
	return ids, nil
}
