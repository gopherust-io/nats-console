package repo

const (
	queryInsertAudit = `
		INSERT INTO audit_log (id, actor, action, cluster_id, resource_type, resource_name, request_id, details, ip)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	queryCountAudit = `SELECT COUNT(*) FROM audit_log `

	queryListAudit = `
		SELECT id, timestamp, actor, action, cluster_id, resource_type, resource_name, request_id, details, ip
		FROM audit_log %s
		ORDER BY timestamp DESC
		LIMIT $%d OFFSET $%d`

	queryDeleteAuditOlderThan = `DELETE FROM audit_log WHERE timestamp < $1`
)
