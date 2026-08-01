package store

const (
	queryListEventCatalogEntries = `
		SELECT subject, owner, description, schema, example, deprecated,
		       successor_subject, deprecation_note,
		       COALESCE(updated_by::text, ''), created_at, updated_at
		FROM event_catalog_entries
		WHERE cluster_id = $1
		ORDER BY subject ASC`

	queryGetEventCatalogEntry = `
		SELECT id::text, cluster_id::text, subject, owner, description, schema, example,
		       deprecated, successor_subject, deprecation_note,
		       COALESCE(updated_by::text, ''), created_at, updated_at
		FROM event_catalog_entries
		WHERE cluster_id = $1 AND subject = $2`

	queryUpsertEventCatalogEntry = `
		INSERT INTO event_catalog_entries (
			id, cluster_id, subject, owner, description, schema, example,
			deprecated, successor_subject, deprecation_note, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (cluster_id, subject) DO UPDATE SET
			owner = EXCLUDED.owner,
			description = EXCLUDED.description,
			schema = EXCLUDED.schema,
			example = EXCLUDED.example,
			deprecated = EXCLUDED.deprecated,
			successor_subject = EXCLUDED.successor_subject,
			deprecation_note = EXCLUDED.deprecation_note,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()`

	queryDeleteEventCatalogEntry = `
		DELETE FROM event_catalog_entries WHERE cluster_id = $1 AND subject = $2`
)
