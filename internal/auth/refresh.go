package auth

import (
	"context"
	"errors"
	"time"

	"github.com/gopherust-io/tel"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

var ErrRefreshReuse = errors.New("refresh token reuse detected")

// IssueRefresh creates an opaque refresh token bound to fingerprint and persists its hash.
// Returns the raw token and the persisted row ID.
func (s *Service) IssueRefresh(ctx context.Context, userID, fingerprint string) (rawToken, tokenID string, err error) {
	if s.db == nil || commonstrings.IsEmpty(userID) || commonstrings.IsEmpty(fingerprint) {
		return "", "", ErrUnauthorized
	}
	raw, err := NewRandomToken()
	if err != nil {
		return "", "", err
	}
	raw2, err := NewRandomToken()
	if err != nil {
		return "", "", err
	}
	raw = raw + raw2
	expires := time.Now().Add(s.cfg.Auth.RefreshTokenTTL)
	row, err := s.db.InsertRefreshToken(ctx, userID, hashRefreshToken(raw), fingerprint, expires)
	if err != nil {
		return "", "", err
	}
	return raw, row.ID, nil
}

// RotateRefresh validates a refresh token + fingerprint, rotates it, and returns a new access JWT + refresh token.
func (s *Service) RotateRefresh(ctx context.Context, rawToken, fingerprint string) (user domain.User, accessToken, newRefresh string, err error) {
	if s.db == nil || commonstrings.IsEmpty(rawToken) || commonstrings.IsEmpty(fingerprint) {
		return domain.User{}, "", "", ErrUnauthorized
	}

	existing, err := s.db.GetRefreshTokenByHash(ctx, hashRefreshToken(rawToken))
	if err != nil {
		return domain.User{}, "", "", ErrUnauthorized
	}

	if !commonstrings.IsEmpty(existing.ReplacedBy) {
		if delErr := s.db.DeleteRefreshTokensByUser(ctx, existing.UserID); delErr != nil {
			tel.Warn().Err(delErr).Msg("auth: revoke refresh family after reuse failed")
		}
		return domain.User{}, "", "", ErrRefreshReuse
	}

	if time.Now().After(existing.ExpiresAt) {
		_ = s.db.DeleteRefreshTokenByHash(ctx, existing.TokenHash)
		return domain.User{}, "", "", ErrUnauthorized
	}
	if existing.FingerprintHash != fingerprint {
		return domain.User{}, "", "", ErrUnauthorized
	}

	user, err = s.LoadUser(ctx, existing.UserID)
	if err != nil {
		return domain.User{}, "", "", ErrUnauthorized
	}

	newRefresh, newID, err := s.IssueRefresh(ctx, user.ID, fingerprint)
	if err != nil {
		return domain.User{}, "", "", err
	}
	ok, err := s.db.MarkRefreshTokenReplaced(ctx, existing.ID, newID)
	if err != nil {
		return domain.User{}, "", "", err
	}
	if !ok {
		_ = s.db.DeleteRefreshTokensByUser(ctx, existing.UserID)
		return domain.User{}, "", "", ErrRefreshReuse
	}

	accessToken, err = s.CreateSession(ctx, user, fingerprint)
	if err != nil {
		return domain.User{}, "", "", err
	}
	return user, accessToken, newRefresh, nil
}

// RevokeRefreshTokensForUser deletes all refresh tokens for the user.
func (s *Service) RevokeRefreshTokensForUser(ctx context.Context, userID string) {
	if s.db == nil || commonstrings.IsEmpty(userID) {
		return
	}
	if err := s.db.DeleteRefreshTokensByUser(ctx, userID); err != nil {
		tel.Error().Err(err).Msg("auth: delete refresh tokens failed")
	}
}

// RevokeRefreshToken deletes a single refresh token by raw value.
func (s *Service) RevokeRefreshToken(ctx context.Context, rawToken string) {
	if s.db == nil || commonstrings.IsEmpty(rawToken) {
		return
	}
	if err := s.db.DeleteRefreshTokenByHash(ctx, hashRefreshToken(rawToken)); err != nil {
		tel.Error().Err(err).Msg("auth: delete refresh token failed")
	}
}
