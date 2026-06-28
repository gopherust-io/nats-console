package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/store"
)

const (
	SessionCookie = "nats_consol_session"
	ContextUser   = "auth_user"
	ContextRoles  = "auth_roles"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

type Claims struct {
	UserID   string   `json:"uid"`
	Username string   `json:"usr"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

type Service struct {
	cfg      config.Config
	store    *store.Store
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
}

func NewService(cfg config.Config, st *store.Store) (*Service, error) {
	s := &Service{cfg: cfg, store: st}
	if cfg.OIDCEnabled {
		if cfg.OIDCIssuer == "" || cfg.OIDCClientID == "" || cfg.OIDCRedirectURL == "" {
			return nil, fmt.Errorf("OIDC_ENABLED requires OIDC_ISSUER, OIDC_CLIENT_ID, and OIDC_REDIRECT_URL")
		}
		provider, err := oidc.NewProvider(context.Background(), cfg.OIDCIssuer)
		if err != nil {
			return nil, fmt.Errorf("oidc provider: %w", err)
		}
		s.verifier = provider.Verifier(&oidc.Config{ClientID: cfg.OIDCClientID})
		s.oauth = &oauth2.Config{
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			RedirectURL:  cfg.OIDCRedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		}
	}
	return s, nil
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
	})
	return err
}

func (s *Service) AuthenticateBasic(ctx context.Context, username, password string) (store.User, error) {
	user, hash, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) && username == s.cfg.AdminUsername && password == s.cfg.AdminPassword {
			return store.User{
				Username: username,
				Roles:    []string{store.RoleAdmin},
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
	}, nil
}

func (s *Service) OIDCEnabled() bool {
	return s.cfg.OIDCEnabled && s.oauth != nil
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

func (s *Service) OIDCAuthURL(state string) string {
	if s.oauth == nil {
		return ""
	}
	return s.oauth.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (s *Service) HandleOIDCCallback(ctx context.Context, code string) (store.User, error) {
	if s.oauth == nil || s.verifier == nil {
		return store.User{}, fmt.Errorf("oidc not configured")
	}
	token, err := s.oauth.Exchange(ctx, code)
	if err != nil {
		return store.User{}, fmt.Errorf("oidc exchange: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return store.User{}, fmt.Errorf("missing id_token")
	}
	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return store.User{}, fmt.Errorf("verify id_token: %w", err)
	}

	var profile struct {
		Email    string `json:"email"`
		Username string `json:"preferred_username"`
		Name     string `json:"name"`
	}
	_ = idToken.Claims(&profile)

	user, err := s.store.GetUserByOIDCSub(ctx, idToken.Subject)
	if errors.Is(err, store.ErrNotFound) {
		username := profile.Username
		if username == "" {
			username = profile.Email
		}
		if username == "" {
			username = idToken.Subject
		}
		user, err = s.store.CreateUser(ctx, store.UserCreate{
			Username: username,
			Email:    profile.Email,
			OIDCSub:  idToken.Subject,
			Roles:    []string{store.RoleViewer},
		})
	}
	if err != nil {
		return store.User{}, err
	}
	return user, nil
}

func (s *Service) SessionCookie(token string) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.cfg.SessionTTL.Seconds()),
	}
}

func (s *Service) ClearSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.IsProduction(),
		MaxAge:   -1,
	}
}

func (s *Service) OAuthStateCookie(state string) *http.Cookie {
	return &http.Cookie{
		Name:     "nats_consol_oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	}
}

func (s *Service) ClearOAuthStateCookie() *http.Cookie {
	return &http.Cookie{
		Name:     "nats_consol_oauth_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.IsProduction(),
		MaxAge:   -1,
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
		return nil, fmt.Errorf("session secret must be at least 16 characters")
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

func RolesFromContext(ctx context.Context) []string {
	if v, ok := ctx.Value(ContextRoles).([]string); ok {
		return v
	}
	return nil
}

func ContextWithUser(ctx context.Context, user store.User) context.Context {
	ctx = context.WithValue(ctx, ContextUser, user)
	ctx = context.WithValue(ctx, ContextRoles, user.Roles)
	return ctx
}

func CanWrite(role string) bool {
	return role == store.RoleAdmin || role == store.RoleOperator
}

func CanDeleteCluster(role string) bool {
	return role == store.RoleAdmin
}

func CanManageUsers(role string) bool {
	return role == store.RoleAdmin
}

func CanViewAudit(role string) bool {
	return role == store.RoleAdmin
}
