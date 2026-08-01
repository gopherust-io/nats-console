package store

import (
	"context"
	"fmt"

	"github.com/gopherust-io/nats-consol/internal/crypto"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type EncryptionRotationStats struct {
	ClustersUpdated int
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

	clusterRows, err := tx.Query(ctx, queryListClusterTokens)
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
			if _, err := tx.Exec(ctx, queryUpdateClusterToken, id, encrypted); err != nil {
				return stats, err
			}
		}
	}
	if err := clusterRows.Err(); err != nil {
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
	if strings.IsEmpty(value) {
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
