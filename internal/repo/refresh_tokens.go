package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// RefreshToken is a persisted opaque refresh-token row.
type RefreshToken struct {
	ID              string
	UserID          string
	TokenHash       string
	FingerprintHash string
	ExpiresAt       time.Time
	CreatedAt       time.Time
	ReplacedBy      string // empty when not rotated
}

// InsertRefreshToken stores a new refresh token hash bound to a fingerprint.
func (db *DB) InsertRefreshToken(ctx context.Context, userID, tokenHash, fingerprintHash string, expiresAt time.Time) (RefreshToken, error) {
	id := newID()
	_, err := db.pool.Exec(ctx, queryInsertRefreshToken, id, userID, tokenHash, fingerprintHash, expiresAt)
	if err != nil {
		return RefreshToken{}, err
	}
	return RefreshToken{
		ID:              id,
		UserID:          userID,
		TokenHash:       tokenHash,
		FingerprintHash: fingerprintHash,
		ExpiresAt:       expiresAt,
		CreatedAt:       time.Now().UTC(),
	}, nil
}

// GetRefreshTokenByHash loads a refresh token by its SHA-256 hash.
func (db *DB) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (RefreshToken, error) {
	var rt RefreshToken
	var replacedBy *string
	err := db.pool.QueryRow(ctx, queryGetRefreshTokenByHash, tokenHash).Scan(
		&rt.ID, &rt.UserID, &rt.TokenHash, &rt.FingerprintHash, &rt.ExpiresAt, &rt.CreatedAt, &replacedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshToken{}, ErrNotFound
	}
	if err != nil {
		return RefreshToken{}, err
	}
	if replacedBy != nil {
		rt.ReplacedBy = *replacedBy
	}
	return rt, nil
}

// MarkRefreshTokenReplaced sets replaced_by when rotating a refresh token.
// Returns false if the row was already replaced (reuse).
func (db *DB) MarkRefreshTokenReplaced(ctx context.Context, id, newID string) (bool, error) {
	tag, err := db.pool.Exec(ctx, queryMarkRefreshTokenReplaced, id, newID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// DeleteRefreshTokensByUser removes all refresh tokens for a user (logout / theft).
func (db *DB) DeleteRefreshTokensByUser(ctx context.Context, userID string) error {
	_, err := db.pool.Exec(ctx, queryDeleteRefreshTokensByUser, userID)
	return err
}

// DeleteRefreshTokenByHash removes a single refresh token by hash.
func (db *DB) DeleteRefreshTokenByHash(ctx context.Context, tokenHash string) error {
	_, err := db.pool.Exec(ctx, queryDeleteRefreshTokenByHash, tokenHash)
	return err
}

// PurgeExpiredRefreshTokens deletes expired refresh rows.
func (db *DB) PurgeExpiredRefreshTokens(ctx context.Context) (int64, error) {
	tag, err := db.pool.Exec(ctx, queryPurgeExpiredRefreshTokens)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
