package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
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

var ErrUnauthorized = errors.New("unauthorized")

// goalign:ignore
type Claims struct {
	jwt.RegisteredClaims

	UserID      string   `json:"uid"`
	Username    string   `json:"usr"`
	Roles       []string `json:"roles"`
	IsRoot      bool     `json:"isRoot"`
	UserVersion int64    `json:"uv,omitempty"`
}

type Service struct {
	store        *store.Store
	users        *userCache
	sessions     *sessionCache
	revocations  *sessionRevocations
	userVersions sync.Map // userID -> int64
	sessionKey   []byte
	cfg          config.Config
}

func NewService(cfg config.Config, st *store.Store) (*Service, error) {
	return &Service{
		cfg:         cfg,
		store:       st,
		users:       newUserCache(defaultUserCacheTTL),
		sessions:    newSessionCache(defaultSessionCacheTTL),
		revocations: newSessionRevocations(),
		sessionKey:  resolveSessionSecret(cfg),
	}, nil
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
	if user, ok := s.users.Get(userID); ok {
		return user, nil
	}
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return store.User{}, err
	}
	user.SessionVersion = s.CurrentUserVersion(userID)
	s.users.Set(user)
	return user, nil
}

// LoadUserForSession skips a DB round-trip when the JWT userVersion matches the
// in-memory stamp and a full user is already in the TTL cache.
func (s *Service) LoadUserForSession(ctx context.Context, partial store.User) (store.User, error) {
	if partial.ID == "" {
		return store.User{}, ErrUnauthorized
	}
	current := s.CurrentUserVersion(partial.ID)
	if partial.SessionVersion > 0 && partial.SessionVersion == current {
		if user, ok := s.users.Get(partial.ID); ok {
			return user, nil
		}
	}
	user, err := s.store.GetUserByID(ctx, partial.ID)
	if err != nil {
		return store.User{}, err
	}
	user.SessionVersion = current
	s.users.Set(user)
	return user, nil
}

func (s *Service) InvalidateUser(userID string) {
	s.users.Invalidate(userID)
	s.BumpUserVersion(userID)
}

func (s *Service) CurrentUserVersion(userID string) int64 {
	if userID == "" {
		return 0
	}
	if v, ok := s.userVersions.Load(userID); ok {
		return v.(int64)
	}
	return 1
}

func (s *Service) BumpUserVersion(userID string) int64 {
	if userID == "" {
		return 0
	}
	for {
		cur := s.CurrentUserVersion(userID)
		next := cur + 1
		if cur == 1 {
			if _, loaded := s.userVersions.LoadOrStore(userID, next); !loaded {
				return next
			}
			continue
		}
		if s.userVersions.CompareAndSwap(userID, cur, next) {
			return next
		}
	}
}

func (s *Service) InvalidateSession(token string) {
	s.sessions.Invalidate(token)
	until := time.Now().Add(s.cfg.SessionTTL)
	if s.cfg.SessionTTL <= 0 {
		until = time.Now().Add(8 * time.Hour)
	}
	s.revocations.Revoke(token, until)
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
	version := s.CurrentUserVersion(user.ID)
	claims := Claims{
		UserID:      user.ID,
		Username:    user.Username,
		Roles:       user.Roles,
		IsRoot:      user.IsRoot,
		UserVersion: version,
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
	if s.revocations.IsRevoked(tokenStr) {
		return store.User{}, ErrUnauthorized
	}
	if user, ok := s.sessions.Get(tokenStr); ok {
		return user, nil
	}
	secret, err := s.sessionSecret()
	if err != nil {
		return store.User{}, ErrUnauthorized
	}
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, ErrUnauthorized
		}
		return secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		return store.User{}, ErrUnauthorized
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return store.User{}, ErrUnauthorized
	}
	user := store.User{
		ID:             claims.UserID,
		Username:       claims.Username,
		Roles:          claims.Roles,
		IsRoot:         claims.IsRoot,
		SessionVersion: claims.UserVersion,
	}
	s.sessions.Set(tokenStr, user)
	return user, nil
}

func (s *Service) AuthEnabled() bool {
	return s.cfg.AuthEnabled
}

func (s *Service) BasicAuthEnabled() bool {
	return s.cfg.AuthEnabled
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
	return NewRandomToken()
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
	return s.sessionKey, nil
}

func resolveSessionSecret(cfg config.Config) []byte {
	key := cfg.SessionSecret
	if key == "" {
		key = cfg.EncryptionKey
	}
	if key == "" {
		key = cfg.AdminPassword
	}
	return []byte(key)
}

func NewRandomToken() (string, error) {
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
