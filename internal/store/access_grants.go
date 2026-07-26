package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

const (
	ResourceSystem   = domain.ResourceSystem
	ResourceAccount  = domain.ResourceAccount
	ResourceNATSUser = domain.ResourceNATSUser

	GrantAdmin                = domain.GrantAdmin
	GrantObserver             = domain.GrantObserver
	GrantCredentialDownloader = domain.GrantCredentialDownloader
)

type AccessGrant struct {
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	ResourceType string    `json:"resourceType"`
	ResourceKey  string    `json:"resourceKey"`
	Role         string    `json:"role"`
	Username     string    `json:"username,omitempty"`
	Email        string    `json:"email,omitempty"`
}

type AccessGrantUpsert struct {
	UserID       string
	ResourceType string
	ResourceKey  string
	Role         string
}

type UserInvite struct {
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	AcceptedAt *time.Time `json:"acceptedAt,omitempty"`
	Token      string     `json:"token"`
	UserID     string     `json:"userId"`
	Username   string     `json:"username,omitempty"`
	Email      string     `json:"email,omitempty"`
}

func (s *Store) ListAccessGrantsByUser(ctx context.Context, userID string) ([]AccessGrant, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, resource_type, resource_key, role, created_at, updated_at
		FROM access_grants WHERE user_id = $1
		ORDER BY resource_type, resource_key`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAccessGrants(rows)
}

func (s *Store) ListAccessGrantsByResource(ctx context.Context, resourceType, resourceKey string) ([]AccessGrant, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT g.id, g.user_id, g.resource_type, g.resource_key, g.role, g.created_at, g.updated_at,
		       u.username, u.email
		FROM access_grants g
		JOIN users u ON u.id = g.user_id
		WHERE g.resource_type = $1 AND g.resource_key = $2
		ORDER BY u.username`, resourceType, resourceKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AccessGrant, 0)
	for rows.Next() {
		var g AccessGrant
		if err := rows.Scan(&g.ID, &g.UserID, &g.ResourceType, &g.ResourceKey, &g.Role, &g.CreatedAt, &g.UpdatedAt, &g.Username, &g.Email); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) UpsertAccessGrant(ctx context.Context, in AccessGrantUpsert) (AccessGrant, error) {
	if err := validateGrant(in); err != nil {
		return AccessGrant{}, err
	}
	now := time.Now().UTC()
	id := uuid.NewString()
	row := s.pool.QueryRow(ctx, `
		INSERT INTO access_grants (id, user_id, resource_type, resource_key, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (user_id, resource_type, resource_key) DO UPDATE
		SET role = EXCLUDED.role, updated_at = EXCLUDED.updated_at
		RETURNING id, user_id, resource_type, resource_key, role, created_at, updated_at`,
		id, in.UserID, in.ResourceType, in.ResourceKey, in.Role, now)
	var g AccessGrant
	if err := row.Scan(&g.ID, &g.UserID, &g.ResourceType, &g.ResourceKey, &g.Role, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return AccessGrant{}, err
	}
	return g, nil
}

func (s *Store) DeleteAccessGrant(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM access_grants WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteAccessGrantByResource(ctx context.Context, userID, resourceType, resourceKey string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM access_grants
		WHERE user_id = $1 AND resource_type = $2 AND resource_key = $3`, userID, resourceType, resourceKey)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateUserInvite(ctx context.Context, userID string, ttl time.Duration) (UserInvite, error) {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	token, err := randomToken(32)
	if err != nil {
		return UserInvite{}, err
	}
	now := time.Now().UTC()
	inv := UserInvite{
		Token:     token,
		UserID:    userID,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO user_invites (token, user_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4)`, inv.Token, inv.UserID, inv.ExpiresAt, inv.CreatedAt)
	if err != nil {
		return UserInvite{}, err
	}
	return inv, nil
}

func (s *Store) GetUserInvite(ctx context.Context, token string) (UserInvite, error) {
	var inv UserInvite
	err := s.pool.QueryRow(ctx, `
		SELECT i.token, i.user_id, i.expires_at, i.accepted_at, i.created_at, u.username, u.email
		FROM user_invites i
		JOIN users u ON u.id = i.user_id
		WHERE i.token = $1`, token).
		Scan(&inv.Token, &inv.UserID, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt, &inv.Username, &inv.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserInvite{}, ErrNotFound
	}
	if err != nil {
		return UserInvite{}, err
	}
	return inv, nil
}

func (s *Store) AcceptUserInvite(ctx context.Context, token, password string) (User, error) {
	inv, err := s.GetUserInvite(ctx, token)
	if err != nil {
		return User{}, err
	}
	if inv.AcceptedAt != nil {
		return User{}, ErrConflict
	}
	if time.Now().UTC().After(inv.ExpiresAt) {
		return User{}, ErrNotFound
	}
	if strings.TrimSpace(password) == "" {
		return User{}, errors.New("password required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	tag, err := tx.Exec(ctx, `
		UPDATE user_invites SET accepted_at = $2
		WHERE token = $1 AND accepted_at IS NULL AND expires_at > $2`, token, now)
	if err != nil {
		return User{}, err
	}
	if tag.RowsAffected() == 0 {
		return User{}, ErrConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	pwd := password
	return s.UpdateUser(ctx, inv.UserID, UserUpdate{Password: &pwd})
}

func (s *Store) attachGrants(ctx context.Context, users []User) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]string, len(users))
	index := make(map[string]int, len(users))
	for i, u := range users {
		ids[i] = u.ID
		index[u.ID] = i
		users[i].Grants = []AccessGrant{}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, resource_type, resource_key, role, created_at, updated_at
		FROM access_grants WHERE user_id = ANY($1)
		ORDER BY user_id, resource_type, resource_key`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var g AccessGrant
		if err := rows.Scan(&g.ID, &g.UserID, &g.ResourceType, &g.ResourceKey, &g.Role, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return err
		}
		if i, ok := index[g.UserID]; ok {
			users[i].Grants = append(users[i].Grants, g)
		}
	}
	return rows.Err()
}

func scanAccessGrants(rows pgx.Rows) ([]AccessGrant, error) {
	out := make([]AccessGrant, 0)
	for rows.Next() {
		var g AccessGrant
		if err := rows.Scan(&g.ID, &g.UserID, &g.ResourceType, &g.ResourceKey, &g.Role, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func validateGrant(in AccessGrantUpsert) error {
	switch in.ResourceType {
	case ResourceSystem, ResourceAccount, ResourceNATSUser:
	default:
		return errors.New("invalid resource type")
	}
	switch in.Role {
	case GrantAdmin, GrantObserver:
	case GrantCredentialDownloader:
		if in.ResourceType != ResourceAccount && in.ResourceType != ResourceNATSUser {
			return errors.New("credential_downloader requires account or nats_user resource")
		}
	default:
		return errors.New("invalid grant role")
	}
	if strings.TrimSpace(in.UserID) == "" || strings.TrimSpace(in.ResourceKey) == "" {
		return errors.New("userId and resourceKey required")
	}
	return nil
}

func randomToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
