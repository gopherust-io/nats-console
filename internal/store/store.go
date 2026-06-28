package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gopherust-io/nats-consol/internal/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool      *pgxpool.Pool
	encryptor *crypto.Encryptor
}

func Open(ctx context.Context, databaseURL, migrationsDir string, encryptor *crypto.Encryptor, poolCfg PoolConfig) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}
	if poolCfg.MaxConns <= 0 {
		poolCfg = DefaultPoolConfig()
	}
	cfg.MaxConns = poolCfg.MaxConns
	cfg.MinConns = poolCfg.MinConns
	cfg.MaxConnLifetime = poolCfg.MaxConnLifetime
	cfg.MaxConnIdleTime = poolCfg.MaxConnIdleTime
	cfg.HealthCheckPeriod = poolCfg.HealthCheckPeriod

	dbPool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := dbPool.Ping(ctx); err != nil {
		dbPool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	s := &Store{pool: dbPool, encryptor: encryptor}
	if err := s.migrate(ctx, migrationsDir); err != nil {
		dbPool.Close()
		return nil, err
	}
	if encryptor != nil {
		if err := s.ReencryptCredentials(ctx); err != nil {
			dbPool.Close()
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

func (s *Store) migrate(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		applied, err := s.isMigrationApplied(ctx, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		// Migration files are read from the configured migrations directory only.
		sql, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // G304: controlled migration dir
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := s.pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT (version) DO NOTHING`, version); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}
	return nil
}

func (s *Store) isMigrationApplied(ctx context.Context, version string) (bool, error) {
	if version == "000_schema_migrations" {
		return false, nil
	}

	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables WHERE table_name = 'schema_migrations'
		)`).Scan(&exists)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	var count int
	err = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = $1`, version).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) encryptToken(token string) (string, error) {
	if token == "" || s.encryptor == nil {
		return token, nil
	}
	return s.encryptor.Encrypt(token)
}

func (s *Store) decryptToken(token string) (string, error) {
	if token == "" || s.encryptor == nil {
		return token, nil
	}
	return s.encryptor.Decrypt(token)
}

func (s *Store) DecryptCredential(value string) (string, error) {
	return s.decryptToken(value)
}

func (s *Store) ReencryptCredentials(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT id, token FROM clusters WHERE token <> ''`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, token string
		if err := rows.Scan(&id, &token); err != nil {
			return err
		}
		if crypto.IsEncrypted(token) {
			continue
		}
		encrypted, err := s.encryptor.Encrypt(token)
		if err != nil {
			return fmt.Errorf("encrypt cluster %s token: %w", id, err)
		}
		if _, err := s.pool.Exec(ctx, `UPDATE clusters SET token = $2, updated_at = NOW() WHERE id = $1`, id, encrypted); err != nil {
			return err
		}
	}
	return rows.Err()
}
