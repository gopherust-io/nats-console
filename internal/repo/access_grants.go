package repo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const (
	ResourceSystem   = domain.ResourceSystem
	ResourceAccount  = domain.ResourceAccount
	ResourceNATSUser = domain.ResourceNATSUser

	GrantAdmin                = domain.GrantAdmin
	GrantObserver             = domain.GrantObserver
	GrantCredentialDownloader = domain.GrantCredentialDownloader
)

type (
	AccessGrant       = domain.AccessGrant
	AccessGrantUpsert = domain.AccessGrantUpsert
	UserInvite        = domain.UserInvite
)

func (db *DB) ListAccessGrantsByUser(ctx context.Context, userID string) ([]AccessGrant, error) {
	rows, err := db.pool.Query(ctx, queryListAccessGrantsByUser, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAccessGrants(rows)
}

func (db *DB) ListAccessGrantsByResource(ctx context.Context, resourceType, resourceKey string) ([]AccessGrant, error) {
	rows, err := db.pool.Query(ctx, queryListAccessGrantsByResource, resourceType, resourceKey)
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

func (db *DB) UpsertAccessGrant(ctx context.Context, in AccessGrantUpsert) (AccessGrant, error) {
	if err := validateGrant(in); err != nil {
		return AccessGrant{}, err
	}
	now := time.Now().UTC()
	id := newID()
	row := db.pool.QueryRow(ctx, queryUpsertAccessGrant,
		id, in.UserID, in.ResourceType, in.ResourceKey, in.Role, now)
	var g AccessGrant
	if err := row.Scan(&g.ID, &g.UserID, &g.ResourceType, &g.ResourceKey, &g.Role, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return AccessGrant{}, err
	}
	return g, nil
}

func (db *DB) DeleteAccessGrant(ctx context.Context, id string) error {
	tag, err := db.pool.Exec(ctx, queryDeleteAccessGrant, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAccessGrantScoped deletes a grant only when it matches the given resource.
// Returns the affected user ID for session invalidation.
func (db *DB) DeleteAccessGrantScoped(ctx context.Context, id, resourceType, resourceKey string) (userID string, err error) {
	err = db.pool.QueryRow(ctx, queryDeleteAccessGrantScoped, id, resourceType, resourceKey).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return userID, err
}

func (db *DB) DeleteAccessGrantByResource(ctx context.Context, userID, resourceType, resourceKey string) error {
	tag, err := db.pool.Exec(ctx, queryDeleteAccessGrantByResource, userID, resourceType, resourceKey)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (db *DB) CreateUserInvite(ctx context.Context, userID string, ttl time.Duration) (UserInvite, error) {
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
	_, err = db.pool.Exec(ctx, queryInsertUserInvite, inv.Token, inv.UserID, inv.ExpiresAt, inv.CreatedAt)
	if err != nil {
		return UserInvite{}, err
	}
	return inv, nil
}

func (db *DB) GetUserInvite(ctx context.Context, token string) (UserInvite, error) {
	var inv UserInvite
	err := db.pool.QueryRow(ctx, queryGetUserInvite, token).
		Scan(&inv.Token, &inv.UserID, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt, &inv.Username, &inv.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserInvite{}, ErrNotFound
	}
	if err != nil {
		return UserInvite{}, err
	}
	return inv, nil
}

func (db *DB) AcceptUserInvite(ctx context.Context, token, password string) (User, error) {
	inv, err := db.GetUserInvite(ctx, token)
	if err != nil {
		return User{}, err
	}
	if inv.AcceptedAt != nil {
		return User{}, ErrConflict
	}
	if time.Now().UTC().After(inv.ExpiresAt) {
		return User{}, ErrNotFound
	}
	password = strings.TrimSpace(password)
	if commonstrings.IsEmpty(password) {
		return User{}, errors.New("password required")
	}
	if len(password) < 8 {
		return User{}, errors.New("password must be at least 8 characters")
	}
	// Hash before opening the transaction: bcrypt is expensive and doesn't
	// touch the DB, so there's no reason to hold the tx open for it.
	hash, err := bcrypt.GenerateFromPassword(commonstrings.StringToBytes(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	tag, err := tx.Exec(ctx, queryAcceptUserInvite, token, now)
	if err != nil {
		return User{}, err
	}
	if tag.RowsAffected() == 0 {
		return User{}, ErrConflict
	}
	// Set the password in the SAME transaction as marking the invite accepted
	// (Medium: invite accept TX), so a failure here can't leave a burned
	// invite with the user's password unchanged.
	if _, err := tx.Exec(ctx, queryUpdateUserPassword, inv.UserID, commonstrings.BytesToString(hash)); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return db.GetUserByID(ctx, inv.UserID)
}

func (db *DB) attachGrants(ctx context.Context, users []User) error {
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
	rows, err := db.pool.Query(ctx, queryListAccessGrantsByUserIDs, ids)
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
	if commonstrings.IsEmpty(strings.TrimSpace(in.UserID)) || commonstrings.IsEmpty(strings.TrimSpace(in.ResourceKey)) {
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
