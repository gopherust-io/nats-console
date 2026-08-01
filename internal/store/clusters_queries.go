package store

const (
	queryCountClusters = `SELECT COUNT(*) FROM clusters`

	queryListClusters = `
		SELECT id, name, nats_url, monitoring_url, creds_file_path, token, is_default, created_at, updated_at
		FROM clusters
		ORDER BY is_default DESC, name ASC`

	queryGetClusterByID = `
		SELECT id, name, nats_url, monitoring_url, creds_file_path, token, is_default, created_at, updated_at
		FROM clusters WHERE id = $1`

	queryGetDefaultCluster = `
		SELECT id, name, nats_url, monitoring_url, creds_file_path, token, is_default, created_at, updated_at
		FROM clusters WHERE is_default = TRUE LIMIT 1`

	queryClearDefaultClusters = `UPDATE clusters SET is_default = FALSE, updated_at = $1`

	queryClearDefaultClustersExcept = `UPDATE clusters SET is_default = FALSE, updated_at = $1 WHERE id <> $2`

	queryInsertCluster = `
		INSERT INTO clusters (id, name, nats_url, monitoring_url, creds_file_path, token, is_default, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, name, nats_url, monitoring_url, creds_file_path, token, is_default, created_at, updated_at`

	queryUpdateCluster = `
		UPDATE clusters
		SET name = $2, nats_url = $3, monitoring_url = $4, creds_file_path = $5, token = $6,
		    is_default = $7, updated_at = $8
		WHERE id = $1
		RETURNING id, name, nats_url, monitoring_url, creds_file_path, token, is_default, created_at, updated_at`

	queryDeleteCluster = `DELETE FROM clusters WHERE id = $1`

	queryListClusterTokens = `SELECT id, token FROM clusters WHERE token <> ''` //nolint:gosec // G101: SQL column name, not a credential

	queryUpdateClusterToken = `UPDATE clusters SET token = $2, updated_at = NOW() WHERE id = $1`
)
