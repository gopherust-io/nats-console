package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gopherust-io/tel"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/crypto"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/repo"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type contextKey string

const (
	SessionCookie = "nats_consol_session"
	RefreshCookie = "nats_consol_refresh"
	CSRFCookie    = "nats_consol_csrf"
)

const ContextUser contextKey = "auth_user"

var ErrUnauthorized = errors.New("unauthorized")

// goalign:ignore
type Claims struct {
	jwt.RegisteredClaims

	UserID      string   `json:"uid"`
	Username    string   `json:"usr"`
	Fingerprint string   `json:"fph"`
	Roles       []string `json:"roles"`
	UserVersion int64    `json:"uv,omitempty"`
	IsRoot      bool     `json:"isRoot"`
}

type Service struct {
	db    *repo.DB
	users *userCache
	// localVersions is the source of truth for user versions when store is
	// nil (tests, or auth running without a DB), and also serves as an
	// immediate same-process view right after a version bump.
	localVersions sync.Map // userID -> int64
	// versionCache is a short-TTL cache fronting store.GetUserVersion so
	// CurrentUserVersion doesn't hit Postgres on every authenticated request.
	versionCache *ttlCache[string, int64]
	sessions     *sessionCache
	revocations  *sessionRevocations
	// revocationCache is a short-TTL cache fronting store.IsSessionRevoked,
	// analogous to versionCache above.
	revocationCache *ttlCache[string, bool]
	sessionPrivate  *rsa.PrivateKey
	sessionPublic   *rsa.PublicKey
	cfg             config.Config
}

// revocationCacheTTL bounds how long a "not revoked" answer from Postgres is
// trusted before being re-checked, so a revoke issued on another replica
// propagates quickly without requiring a DB round-trip on every request.
const revocationCacheTTL = 5 * time.Second

// userVersionCacheTTL bounds how long a Postgres-backed user version is
// trusted before being re-checked (H2): short enough that a version bump on
// one replica (role/grant change, logout-all) becomes visible on other
// replicas quickly, long enough to avoid a DB round-trip on every request.
const userVersionCacheTTL = 5 * time.Second

func NewService(cfg config.Config, db *repo.DB) (*Service, error) {
	priv, pub, err := crypto.ParseSessionRSAKeyPair(cfg.Auth.SessionPrivateKey, cfg.Auth.SessionPublicKey)
	if err != nil {
		return nil, err
	}
	return &Service{
		cfg:             cfg,
		db:              db,
		users:           newUserCache(defaultUserCacheTTL),
		versionCache:    newTTLCache[string, int64](userVersionCacheTTL),
		sessions:        newSessionCache(defaultSessionCacheTTL),
		revocations:     newSessionRevocations(),
		revocationCache: newTTLCache[string, bool](revocationCacheTTL),
		sessionPrivate:  priv,
		sessionPublic:   pub,
	}, nil
}

func (s *Service) SeedAdmin(ctx context.Context) error {
	count, err := s.db.CountUsers(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = s.db.CreateUser(ctx, repo.UserCreate{
		Username: s.cfg.AdminUsername,
		Email:    s.cfg.AdminUsername + "@local",
		Password: s.cfg.AdminPassword,
		Roles:    []string{repo.RoleAdmin},
		IsRoot:   true,
	})
	return err
}

func (s *Service) LoadUser(ctx context.Context, userID string) (domain.User, error) {
	if commonstrings.IsEmpty(userID) {
		return domain.User{}, ErrUnauthorized
	}
	if user, ok := s.users.Get(userID); ok {
		return user, nil
	}
	repoUser, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}
	repoUser.SessionVersion = s.CurrentUserVersion(ctx, userID)
	user := StoreUserToDomain(repoUser)
	s.users.Set(user)
	return user, nil
}

// LoadUserForSession skips a DB round-trip when the JWT userVersion matches the in-memory stamp and a full user is already in the TTL cache.
func (s *Service) LoadUserForSession(ctx context.Context, partial domain.User) (domain.User, error) {
	if commonstrings.IsEmpty(partial.ID) {
		return domain.User{}, ErrUnauthorized
	}
	current := s.CurrentUserVersion(ctx, partial.ID)
	if partial.SessionVersion > 0 && partial.SessionVersion == current {
		if user, ok := s.users.Get(partial.ID); ok {
			return user, nil
		}
		if s.db == nil {
			return partial, nil
		}
	}
	if s.db == nil {
		return domain.User{}, ErrUnauthorized
	}
	repoUser, err := s.db.GetUserByID(ctx, partial.ID)
	if err != nil {
		return domain.User{}, err
	}
	repoUser.SessionVersion = current
	user := StoreUserToDomain(repoUser)
	s.users.Set(user)
	return user, nil
}

// LoadUserFresh bypasses user and version caches so grant/role revocation is
// visible immediately on this replica for sensitive authz paths (creds, assign, grants).
func (s *Service) LoadUserFresh(ctx context.Context, userID string) (domain.User, error) {
	if commonstrings.IsEmpty(userID) {
		return domain.User{}, ErrUnauthorized
	}
	s.users.Invalidate(userID)
	if s.versionCache != nil {
		s.versionCache.Invalidate(userID)
	}
	if s.db == nil {
		return domain.User{}, ErrUnauthorized
	}
	repoUser, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}
	repoUser.SessionVersion = s.CurrentUserVersion(ctx, userID)
	user := StoreUserToDomain(repoUser)
	s.users.Set(user)
	return user, nil
}

// InvalidateUser drops the cached user record and bumps its persisted
// version (H2) so that permission/role changes and forced re-fetches are
// visible on every replica, not just the one handling this request.
func (s *Service) InvalidateUser(ctx context.Context, userID string) {
	s.users.Invalidate(userID)
	if s.versionCache != nil {
		s.versionCache.Invalidate(userID)
	}
	s.BumpUserVersion(ctx, userID)
}

// CurrentUserVersion returns the user's session-invalidation version. When a
// store is configured, the authoritative value is persisted in Postgres
// (auth_user_versions) and cached locally for a short TTL (H2); with no
// store (e.g. unit tests), it falls back to a process-local value.
func (s *Service) CurrentUserVersion(ctx context.Context, userID string) int64 {
	if commonstrings.IsEmpty(userID) {
		return 0
	}
	if s.db == nil {
		return s.currentLocalVersion(userID)
	}
	if v, ok := s.versionCache.Get(userID); ok {
		return v
	}
	version, err := s.db.GetUserVersion(ctx, userID)
	if err != nil {
		// DB unreachable: degrade to whatever this replica has observed
		// locally rather than failing every authenticated request.
		return s.currentLocalVersion(userID)
	}
	s.versionCache.Set(userID, version)
	return version
}

// BumpUserVersion atomically increments the persisted user version (via
// Postgres when available, so the bump is visible to every replica) and
// returns the new value.
func (s *Service) BumpUserVersion(ctx context.Context, userID string) int64 {
	if commonstrings.IsEmpty(userID) {
		return 0
	}
	if s.db != nil {
		if v, err := s.db.BumpUserVersion(ctx, userID); err == nil {
			s.versionCache.Set(userID, v)
			s.localVersions.Store(userID, v)
			return v
		}
	}
	return s.bumpLocalVersion(userID)
}

func (s *Service) currentLocalVersion(userID string) int64 {
	if v, ok := s.localVersions.Load(userID); ok {
		return v.(int64)
	}
	return 1
}

func (s *Service) bumpLocalVersion(userID string) int64 {
	for {
		cur := s.currentLocalVersion(userID)
		next := cur + 1
		if cur == 1 {
			if _, loaded := s.localVersions.LoadOrStore(userID, next); !loaded {
				return next
			}
			continue
		}
		if s.localVersions.CompareAndSwap(userID, cur, next) {
			return next
		}
	}
}

// InvalidateSession revokes a session token both locally (fast path for this
// replica) and, when a store is configured, in Postgres (H2) so logout is
// enforced across all replicas rather than only the one that served it.
func (s *Service) InvalidateSession(ctx context.Context, token string) {
	s.sessions.Invalidate(token)
	until := time.Now().Add(s.cfg.Auth.SessionTTL)
	s.revocations.Revoke(token, until)
	if s.db != nil {
		if err := s.db.RevokeSession(ctx, hashSessionToken(token), until); err != nil {
			tel.Error().Err(err).Msg("auth: persist session revocation failed:")
		}
	}
}

func (s *Service) AuthenticateBasic(ctx context.Context, username, password string) (domain.User, error) {
	user, hash, err := s.db.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) && username == s.cfg.AdminUsername && password == s.cfg.AdminPassword {
			return s.ensureEnvAdminUser(ctx)
		}
		return domain.User{}, ErrUnauthorized
	}
	if commonstrings.IsEmpty(hash) || !repo.CheckPassword(hash, password) {
		return domain.User{}, ErrUnauthorized
	}
	return StoreUserToDomain(user), nil
}

// ensureEnvAdminUser recreates the configured env admin as a real DB user so
// sessions always have a non-empty ID and can be version-revoked.
func (s *Service) ensureEnvAdminUser(ctx context.Context) (domain.User, error) {
	if s.db == nil {
		return domain.User{}, ErrUnauthorized
	}
	created, err := s.db.CreateUser(ctx, repo.UserCreate{
		Username: s.cfg.AdminUsername,
		Email:    s.cfg.AdminUsername + "@local",
		Password: s.cfg.AdminPassword,
		Roles:    []string{repo.RoleAdmin},
		IsRoot:   true,
	})
	if err == nil {
		return StoreUserToDomain(created), nil
	}
	if errors.Is(err, repo.ErrConflict) {
		// A root user already exists under another username — recreate env
		// admin without the root bit so login still works.
		created, err = s.db.CreateUser(ctx, repo.UserCreate{
			Username: s.cfg.AdminUsername,
			Email:    s.cfg.AdminUsername + "@local",
			Password: s.cfg.AdminPassword,
			Roles:    []string{repo.RoleAdmin},
			IsRoot:   false,
		})
		if err == nil {
			return StoreUserToDomain(created), nil
		}
	}
	// Concurrent create or transient error: re-fetch if the row now exists.
	user, _, getErr := s.db.GetUserByUsername(ctx, s.cfg.AdminUsername)
	if getErr == nil && !commonstrings.IsEmpty(user.ID) {
		return StoreUserToDomain(user), nil
	}

	tel.Error().Err(err).Str("admin", s.cfg.AdminUsername).Msg("auth: failed to ensure env admin user")

	return domain.User{}, ErrUnauthorized
}

func (s *Service) CreateSession(ctx context.Context, user domain.User, fingerprint string) (string, error) {
	if commonstrings.IsEmpty(user.ID) || commonstrings.IsEmpty(fingerprint) {
		return "", ErrUnauthorized
	}
	now := time.Now()
	version := s.CurrentUserVersion(ctx, user.ID)
	claims := Claims{
		UserID:      user.ID,
		Username:    user.Username,
		Roles:       user.Roles,
		IsRoot:      user.IsRoot,
		UserVersion: version,
		Fingerprint: fingerprint,
		ExpiresAt:   jwt.NewNumericDate(now.Add(s.cfg.Auth.SessionTTL)),
		IssuedAt:    jwt.NewNumericDate(now),
		Subject:     user.Username,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(s.sessionPrivate)
}

// ParseSession validates a session token, checking revocation denylists and
// that the request fingerprint matches the fph claim embedded at issue time.
func (s *Service) ParseSession(ctx context.Context, tokenStr, fingerprint string) (domain.User, error) {
	if commonstrings.IsEmpty(fingerprint) {
		return domain.User{}, ErrUnauthorized
	}
	if s.revocations.IsRevoked(tokenStr) {
		return domain.User{}, ErrUnauthorized
	}
	if s.isPersistentlyRevoked(ctx, tokenStr) {
		return domain.User{}, ErrUnauthorized
	}
	if user, ok := s.sessions.Get(tokenStr, fingerprint); ok {
		return user, nil
	}
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodRS256 {
			return nil, ErrUnauthorized
		}
		return s.sessionPublic, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	if err != nil || !token.Valid {
		return domain.User{}, ErrUnauthorized
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return domain.User{}, ErrUnauthorized
	}
	if commonstrings.IsEmpty(claims.Fingerprint) || claims.Fingerprint != fingerprint {
		return domain.User{}, ErrUnauthorized
	}
	user := domain.User{
		ID:             claims.UserID,
		Username:       claims.Username,
		Roles:          claims.Roles,
		IsRoot:         claims.IsRoot,
		SessionVersion: claims.UserVersion,
	}
	s.sessions.Set(tokenStr, fingerprint, user)
	return user, nil
}

// isPersistentlyRevoked checks the Postgres-backed session denylist,
// short-circuiting via a small local cache. Positive (revoked) results are
// also pushed into the fast in-memory denylist so subsequent checks for the
// same token skip the DB entirely.
func (s *Service) isPersistentlyRevoked(ctx context.Context, tokenStr string) bool {
	if s.db == nil || commonstrings.IsEmpty(tokenStr) {
		return false
	}
	jti := hashSessionToken(tokenStr)
	if v, ok := s.revocationCache.Get(jti); ok {
		return v
	}
	revoked, err := s.db.IsSessionRevoked(ctx, jti)
	if err != nil {
		tel.Warn().Err(err).Msg("auth: session revocation check failed")
		return true
	}
	s.revocationCache.Set(jti, revoked)
	if revoked {
		s.revocations.Revoke(tokenStr, time.Now().Add(s.cfg.Auth.SessionTTL))
	}
	return revoked
}

func (s *Service) SessionCookie(token string) *http.Cookie {
	return s.newCookie(SessionCookie, token, int(s.cfg.Auth.SessionTTL.Seconds()), true)
}

func (s *Service) ClearSessionCookie() *http.Cookie {
	return s.newCookie(SessionCookie, "", -1, true)
}

func (s *Service) RefreshTokenCookie(token string) *http.Cookie {
	return s.newCookie(RefreshCookie, token, int(s.cfg.Auth.RefreshTokenTTL.Seconds()), true)
}

func (s *Service) ClearRefreshCookie() *http.Cookie {
	return s.newCookie(RefreshCookie, "", -1, true)
}

func (s *Service) CSRFCookie(token string) *http.Cookie {
	return s.newCookie(CSRFCookie, token, int(s.cfg.Auth.RefreshTokenTTL.Seconds()), false)
}

func (s *Service) ClearCSRFCookie() *http.Cookie {
	return s.newCookie(CSRFCookie, "", -1, false)
}

func (s *Service) NewCSRFToken() (string, error) {
	return NewRandomToken()
}

func (s *Service) newCookie(name, value string, maxAge int, httpOnly bool) *http.Cookie {
	return &http.Cookie{ //nolint:gosec
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: httpOnly,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
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
	user, pass, found := strings.Cut(commonstrings.BytesToString(decoded), ":")
	return user, pass, found
}

func UserFromContext(ctx context.Context) (domain.User, bool) {
	if v, ok := ctx.Value(ContextUser).(domain.User); ok {
		return v, true
	}
	return domain.User{}, false
}

func ContextWithUser(ctx context.Context, user domain.User) context.Context {
	return context.WithValue(ctx, ContextUser, user)
}
