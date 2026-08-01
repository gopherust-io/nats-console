package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// GetUserVersion returns the persisted session-invalidation version for
// userID, defaulting to 1 (the same "current" baseline used throughout
// auth.Service) when no row exists yet.
func (s *Store) GetUserVersion(ctx context.Context, userID string) (int64, error) {
	var version int64
	err := s.pool.QueryRow(ctx, queryGetUserVersion, userID).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	return version, nil
}

// BumpUserVersion atomically increments (or initializes to 2, since the
// implicit baseline is 1) the persisted user version and returns the new
// value. The increment happens entirely inside Postgres so concurrent bumps
// from any replica are serialized correctly.
func (s *Store) BumpUserVersion(ctx context.Context, userID string) (int64, error) {
	var version int64
	err := s.pool.QueryRow(ctx, queryBumpUserVersion, userID).Scan(&version)
	return version, err
}

// RevokeSession persists a session-token denylist entry (keyed by a stable
// hash of the token, see auth.hashSessionToken) so logout is enforced across
// all replicas until the token would have expired naturally anyway.
func (s *Store) RevokeSession(ctx context.Context, jti string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, queryRevokeSession, jti, expiresAt)
	return err
}

// IsSessionRevoked reports whether jti is present in the denylist and has
// not yet expired. Expired entries are treated as not-revoked but are left
// in place for purgeExpiredSessionRevocations to reap in bulk.
func (s *Store) IsSessionRevoked(ctx context.Context, jti string) (bool, error) {
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, queryGetSessionRevocation, jti).Scan(&expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return time.Now().Before(expiresAt), nil
}

// PurgeExpiredSessionRevocations deletes denylist entries that have expired,
// keeping auth_session_revocations from growing unbounded. Safe to call
// periodically from a background task; it is not required for correctness.
func (s *Store) PurgeExpiredSessionRevocations(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, queryPurgeExpiredSessionRevocations)
	return err
}
