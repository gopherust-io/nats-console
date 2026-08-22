package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/gopherust-io/nats-consol/internal/crypto"
	"github.com/gopherust-io/nats-consol/migrations"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type DB struct {
	pool              *pgxpool.Pool
	encryptor         *crypto.Encryptor
	ensuredPartitions sync.Map // parent|YYYY-MM-DD -> struct{}
}

func Open(ctx context.Context, databaseURL string, encryptor *crypto.Encryptor, poolCfg PoolConfig) (*DB, error) {
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

	s := &DB{pool: dbPool, encryptor: encryptor}
	if err := migrate(ctx, databaseURL); err != nil {
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

func (db *DB) Stop() {
	if db.pool != nil {
		db.pool.Close()
	}
}

func (db *DB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// migrate applies pending goose SQL migrations from the embedded migrations.FS.
// Goose wraps each Up in a transaction by default; use -- +goose NO TRANSACTION
// in a SQL file only when Postgres forbids transactional DDL (e.g. CONCURRENTLY).
func migrate(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migrate db: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping migrate db: %w", err)
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	if err := bridgeLegacySchemaMigrations(ctx, db); err != nil {
		return err
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	defer func() { _ = provider.Close() }()

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// bridgeLegacySchemaMigrations seeds goose_db_version from the old
// schema_migrations table (if present), then drops it so goose owns versioning.
func bridgeLegacySchemaMigrations(ctx context.Context, db *sql.DB) error {
	var exists bool
	err := db.QueryRowContext(ctx, querySchemaMigrationsTableExists).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check schema_migrations: %w", err)
	}
	if !exists {
		return nil
	}

	rows, err := db.QueryContext(ctx, queryListSchemaMigrationVersions)
	if err != nil {
		return fmt.Errorf("list schema_migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	versions := make([]int64, 0)
	seen := make(map[int64]struct{})
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("scan schema_migrations: %w", err)
		}
		n, ok := legacyMigrationVersion(version)
		if !ok {
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		versions = append(versions, n)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate schema_migrations: %w", err)
	}

	if _, err := goose.EnsureDBVersion(db); err != nil {
		return fmt.Errorf("ensure goose version table: %w", err)
	}

	for _, v := range versions {
		var applied bool
		err := db.QueryRowContext(ctx, queryGooseVersionApplied, v).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check goose version %d: %w", v, err)
		}
		if applied {
			continue
		}
		if _, err := db.ExecContext(ctx, queryInsertGooseVersion, v); err != nil {
			return fmt.Errorf("seed goose version %d: %w", v, err)
		}
	}

	if _, err := db.ExecContext(ctx, queryDropSchemaMigrations); err != nil {
		return fmt.Errorf("drop schema_migrations: %w", err)
	}
	return nil
}

// legacyMigrationVersion parses "001_clusters" → 1. Skips "000_*" and non-numeric stems.
func legacyMigrationVersion(version string) (int64, bool) {
	version = strings.TrimSpace(version)
	if commonstrings.IsEmpty(version) {
		return 0, false
	}
	i := 0
	for i < len(version) && unicode.IsDigit(rune(version[i])) {
		i++
	}
	if i == 0 {
		return 0, false
	}
	n, err := strconv.ParseInt(version[:i], 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func (db *DB) encryptToken(token string) (string, error) {
	if commonstrings.IsEmpty(token) || db.encryptor == nil {
		return token, nil
	}
	return db.encryptor.Encrypt(token)
}

func (db *DB) decryptToken(token string) (string, error) {
	if commonstrings.IsEmpty(token) || db.encryptor == nil {
		return token, nil
	}
	return db.encryptor.Decrypt(token)
}

func (db *DB) DecryptCredential(value string) (string, error) {
	return db.decryptToken(value)
}

func (db *DB) ReencryptCredentials(ctx context.Context) error {
	rows, err := db.pool.Query(ctx, queryListClusterTokens)
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

		encrypted, err := db.encryptor.Encrypt(token)
		if err != nil {
			return fmt.Errorf("encrypt cluster %s token: %w", id, err)
		}
		if _, err := db.pool.Exec(ctx, queryUpdateClusterToken, id, encrypted); err != nil {
			return err
		}
	}
	return rows.Err()
}
