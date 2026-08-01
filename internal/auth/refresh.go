package auth

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/gopherust-io/nats-consol/internal/store"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

var ErrRefreshReuse = errors.New("refresh token reuse detected")

// IssueRefresh creates an opaque refresh token bound to fingerprint and persists its hash.
// Returns the raw token and the persisted row ID.
func (s *Service) IssueRefresh(ctx context.Context, userID, fingerprint string) (rawToken, tokenID string, err error) {
	if s.store == nil || commonstrings.IsEmpty(userID) || commonstrings.IsEmpty(fingerprint) {
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
	row, err := s.store.InsertRefreshToken(ctx, userID, hashRefreshToken(raw), fingerprint, expires)
	if err != nil {
		return "", "", err
	}
	return raw, row.ID, nil
}

// RotateRefresh validates a refresh token + fingerprint, rotates it, and returns a new access JWT + refresh token.
func (s *Service) RotateRefresh(ctx context.Context, rawToken, fingerprint string) (user store.User, accessToken, newRefresh string, err error) {
	if s.store == nil || commonstrings.IsEmpty(rawToken) || commonstrings.IsEmpty(fingerprint) {
		return store.User{}, "", "", ErrUnauthorized
	}
	existing, err := s.store.GetRefreshTokenByHash(ctx, hashRefreshToken(rawToken))
	if err != nil {
		return store.User{}, "", "", ErrUnauthorized
	}
	if !commonstrings.IsEmpty(existing.ReplacedBy) {
		if delErr := s.store.DeleteRefreshTokensByUser(ctx, existing.UserID); delErr != nil {
			log.Printf("auth: revoke refresh family after reuse failed: %v", delErr)
		}
		return store.User{}, "", "", ErrRefreshReuse
	}
	if time.Now().After(existing.ExpiresAt) {
		_ = s.store.DeleteRefreshTokenByHash(ctx, existing.TokenHash)
		return store.User{}, "", "", ErrUnauthorized
	}
	if existing.FingerprintHash != fingerprint {
		return store.User{}, "", "", ErrUnauthorized
	}

	user, err = s.LoadUser(ctx, existing.UserID)
	if err != nil {
		return store.User{}, "", "", ErrUnauthorized
	}

	newRefresh, newID, err := s.IssueRefresh(ctx, user.ID, fingerprint)
	if err != nil {
		return store.User{}, "", "", err
	}
	ok, err := s.store.MarkRefreshTokenReplaced(ctx, existing.ID, newID)
	if err != nil {
		return store.User{}, "", "", err
	}
	if !ok {
		_ = s.store.DeleteRefreshTokensByUser(ctx, existing.UserID)
		return store.User{}, "", "", ErrRefreshReuse
	}

	accessToken, err = s.CreateSession(ctx, user, fingerprint)
	if err != nil {
		return store.User{}, "", "", err
	}
	return user, accessToken, newRefresh, nil
}

// RevokeRefreshTokensForUser deletes all refresh tokens for the user.
func (s *Service) RevokeRefreshTokensForUser(ctx context.Context, userID string) {
	if s.store == nil || commonstrings.IsEmpty(userID) {
		return
	}
	if err := s.store.DeleteRefreshTokensByUser(ctx, userID); err != nil {
		log.Printf("auth: delete refresh tokens failed: %v", err)
	}
}

// RevokeRefreshToken deletes a single refresh token by raw value.
func (s *Service) RevokeRefreshToken(ctx context.Context, rawToken string) {
	if s.store == nil || commonstrings.IsEmpty(rawToken) {
		return
	}
	if err := s.store.DeleteRefreshTokenByHash(ctx, hashRefreshToken(rawToken)); err != nil {
		log.Printf("auth: delete refresh token failed: %v", err)
	}
}
