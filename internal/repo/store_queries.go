package repo

const (
	querySchemaMigrationsTableExists = `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'schema_migrations'
		)`

	queryListSchemaMigrationVersions = `SELECT version FROM schema_migrations`

	queryGooseVersionApplied = `
			SELECT EXISTS (
				SELECT 1 FROM goose_db_version WHERE version_id = $1 AND is_applied = true
			)`

	queryInsertGooseVersion = `
			INSERT INTO goose_db_version (version_id, is_applied) VALUES ($1, true)`

	queryDropSchemaMigrations = `DROP TABLE schema_migrations`
)
