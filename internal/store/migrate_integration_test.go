//go:build integration

package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestGooseMigrateFreshAndLegacyBridge(t *testing.T) {
	if !commonstrings.IsEmpty(os.Getenv("SKIP_TESTCONTAINERS")) {
		t.Skip("SKIP_TESTCONTAINERS set")
	}
	if _, err := testcontainers.NewDockerProvider(); err != nil {
		t.Skipf("docker unavailable: %v", err)
	}

	ctx := context.Background()
	pgURL := startTestPostgres(t, ctx)
	dir := migrationsDirForTest(t)

	st, err := Open(ctx, pgURL, dir, nil, DefaultPoolConfig())
	require.NoError(t, err)
	defer st.Close()

	var gooseCount int
	require.NoError(t, st.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM goose_db_version WHERE version_id > 0 AND is_applied`).Scan(&gooseCount))
	require.GreaterOrEqual(t, gooseCount, 23)
	assertPartitionedSamples(t, ctx, st.pool)
	assertUUIDv7Default(t, ctx, st.pool)

	var legacyExists bool
	require.NoError(t, st.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'schema_migrations'
		)`).Scan(&legacyExists))
	require.False(t, legacyExists)

	var clustersExists bool
	require.NoError(t, st.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'clusters'
		)`).Scan(&clustersExists))
	require.True(t, clustersExists)

	// Re-open is idempotent.
	st2, err := Open(ctx, pgURL, dir, nil, DefaultPoolConfig())
	require.NoError(t, err)
	st2.Close()

	// Legacy bridge: recreate schema_migrations as if an old DB was upgraded mid-flight.
	pgURL2 := startTestPostgres(t, ctx)
	db, err := sql.Open("pgx", pgURL2)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.ExecContext(ctx, `
		CREATE TABLE schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE clusters (
			id UUID PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			nats_url TEXT NOT NULL,
			monitoring_url TEXT NOT NULL DEFAULT '',
			creds_file_path TEXT NOT NULL DEFAULT '',
			token TEXT NOT NULL DEFAULT '',
			is_default BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		INSERT INTO schema_migrations (version) VALUES
			('000_schema_migrations'),
			('001_clusters'),
			('002_encrypt_credentials');
	`)
	require.NoError(t, err)

	st3, err := Open(ctx, pgURL2, dir, nil, DefaultPoolConfig())
	require.NoError(t, err)
	defer st3.Close()

	require.NoError(t, st3.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'schema_migrations'
		)`).Scan(&legacyExists))
	require.False(t, legacyExists)

	var applied1, applied2 bool
	require.NoError(t, st3.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM goose_db_version WHERE version_id = 1 AND is_applied)`).Scan(&applied1))
	require.NoError(t, st3.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM goose_db_version WHERE version_id = 2 AND is_applied)`).Scan(&applied2))
	require.True(t, applied1)
	require.True(t, applied2)

	var maxVersion int64
	require.NoError(t, st3.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied`).Scan(&maxVersion))
	require.GreaterOrEqual(t, maxVersion, int64(23))
}

func assertPartitionedSamples(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, table := range []string{"cluster_metric_samples", "incident_consumer_samples"} {
		var partStrategy string
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT COALESCE(pt.partstrat::text, '')
			FROM pg_class c
			LEFT JOIN pg_partitioned_table pt ON pt.partrelid = c.oid
			WHERE c.relname = $1`, table).Scan(&partStrategy))
		require.Equal(t, "r", partStrategy, "%s should be RANGE partitioned", table)

		var hasID bool
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = $1 AND column_name = 'id'
			)`, table).Scan(&hasID))
		require.False(t, hasID, "%s should not have surrogate id", table)
	}
}

func assertUUIDv7Default(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var def string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT column_default::text
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'alert_rules' AND column_name = 'id'
	`).Scan(&def))
	require.Contains(t, def, "uuid_generate_v7")

	var id uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT uuid_generate_v7()`).Scan(&id))
	require.Equal(t, uuid.Version(7), id.Version())
}

func startTestPostgres(t *testing.T, ctx context.Context) string {
	t.Helper()
	c, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("natsconsol"),
		postgres.WithUsername("natsconsol"),
		postgres.WithPassword("natsconsol"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	pgURL, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	backoff := 200 * time.Millisecond
	var lastErr error
	for range 30 {
		pool, err := pgxpool.New(ctx, pgURL)
		if err != nil {
			lastErr = err
			time.Sleep(backoff)
			continue
		}
		err = pool.Ping(ctx)
		pool.Close()
		if err == nil {
			return pgURL
		}
		lastErr = err
		time.Sleep(backoff)
	}
	t.Fatalf("postgres ready: %v", lastErr)
	return ""
}

func migrationsDirForTest(t *testing.T) string {
	t.Helper()
	for _, dir := range []string{"migrations", filepath.Join("..", "..", "migrations")} {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	t.Fatal("migrations dir not found")
	return ""
}
