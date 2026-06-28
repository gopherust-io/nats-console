package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gopherust-io/nats-consol/internal/crypto"
)

type JWTAccount struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt *time.Time
	ID        string
	ClusterID string
	Name      string
	JWT       string
}

type JWTAccountCreate struct {
	ExpiresAt *time.Time
	ClusterID string
	Name      string
	JWT       string
}

func (s *Store) ListJWTAccounts(ctx context.Context, clusterID string) ([]JWTAccount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, cluster_id, name, jwt, expires_at, created_at, updated_at
		FROM nats_jwt_accounts
		WHERE cluster_id = $1
		ORDER BY name ASC`, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]JWTAccount, 0)
	for rows.Next() {
		var item JWTAccount
		if err := rows.Scan(&item.ID, &item.ClusterID, &item.Name, &item.JWT, &item.ExpiresAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if out == nil {
		return []JWTAccount{}, rows.Err()
	}
	return out, rows.Err()
}

func (s *Store) CreateJWTAccount(ctx context.Context, in JWTAccountCreate) (JWTAccount, error) {
	encrypted, err := s.encryptToken(in.JWT)
	if err != nil {
		return JWTAccount{}, fmt.Errorf("encrypt jwt: %w", err)
	}
	id := uuid.NewString()
	now := time.Now().UTC()
	row := s.pool.QueryRow(ctx, `
		INSERT INTO nats_jwt_accounts (id, cluster_id, name, jwt, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, cluster_id, name, jwt, expires_at, created_at, updated_at`,
		id, in.ClusterID, in.Name, encrypted, in.ExpiresAt, now, now)
	var item JWTAccount
	if err := row.Scan(&item.ID, &item.ClusterID, &item.Name, &item.JWT, &item.ExpiresAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return JWTAccount{}, err
	}
	return item, nil
}

func (s *Store) DeleteJWTAccount(ctx context.Context, clusterID, name string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM nats_jwt_accounts WHERE cluster_id = $1 AND name = $2`, clusterID, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ExportJWTAccounts(ctx context.Context, clusterID string) ([]JWTAccount, error) {
	return s.ListJWTAccounts(ctx, clusterID)
}

type EncryptionRotationStats struct {
	ClustersUpdated int
	JWTUpdated      int
}

func (s *Store) RotateEncryptionKeys(ctx context.Context, currentKey, newKey string, dryRun bool) (EncryptionRotationStats, error) {
	oldEnc, err := crypto.New(currentKey)
	if err != nil {
		return EncryptionRotationStats{}, fmt.Errorf("current key: %w", err)
	}
	newEnc, err := crypto.New(newKey)
	if err != nil {
		return EncryptionRotationStats{}, fmt.Errorf("new key: %w", err)
	}

	stats := EncryptionRotationStats{}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return stats, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	clusterRows, err := tx.Query(ctx, `SELECT id, token FROM clusters WHERE token <> ''`)
	if err != nil {
		return stats, err
	}
	defer clusterRows.Close()
	for clusterRows.Next() {
		var id, token string
		if err := clusterRows.Scan(&id, &token); err != nil {
			return stats, err
		}
		plain, err := decryptWithFallback(token, oldEnc, s.encryptor)
		if err != nil {
			return stats, fmt.Errorf("cluster %s token: %w", id, err)
		}
		encrypted, err := newEnc.Encrypt(plain)
		if err != nil {
			return stats, err
		}
		stats.ClustersUpdated++
		if !dryRun {
			if _, err := tx.Exec(ctx, `UPDATE clusters SET token = $2, updated_at = NOW() WHERE id = $1`, id, encrypted); err != nil {
				return stats, err
			}
		}
	}
	if err := clusterRows.Err(); err != nil {
		return stats, err
	}

	jwtRows, err := tx.Query(ctx, `SELECT id, jwt FROM nats_jwt_accounts WHERE jwt <> ''`)
	if err != nil {
		return stats, err
	}
	defer jwtRows.Close()
	for jwtRows.Next() {
		var id, jwt string
		if err := jwtRows.Scan(&id, &jwt); err != nil {
			return stats, err
		}
		plain, err := decryptWithFallback(jwt, oldEnc, s.encryptor)
		if err != nil {
			return stats, fmt.Errorf("jwt account %s: %w", id, err)
		}
		encrypted, err := newEnc.Encrypt(plain)
		if err != nil {
			return stats, err
		}
		stats.JWTUpdated++
		if !dryRun {
			if _, err := tx.Exec(ctx, `UPDATE nats_jwt_accounts SET jwt = $2, updated_at = NOW() WHERE id = $1`, id, encrypted); err != nil {
				return stats, err
			}
		}
	}
	if err := jwtRows.Err(); err != nil {
		return stats, err
	}

	if dryRun {
		return stats, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return stats, err
	}
	return stats, nil
}

func decryptWithFallback(value string, oldEnc, active *crypto.Encryptor) (string, error) {
	if value == "" {
		return "", nil
	}
	if !crypto.IsEncrypted(value) {
		return value, nil
	}
	if plain, err := oldEnc.Decrypt(value); err == nil {
		return plain, nil
	}
	if active != nil {
		return active.Decrypt(value)
	}
	return "", crypto.ErrDecrypt
}
