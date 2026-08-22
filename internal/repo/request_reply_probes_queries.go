package repo

const (
	queryListRequestReplyProbes = `
		SELECT id::text, cluster_id::text, subject, payload, payload_format, timeout_ms, enabled, created_at, updated_at
		FROM request_reply_probes
		WHERE cluster_id = $1
		ORDER BY subject ASC`

	queryListEnabledRequestReplyProbes = `
		SELECT id::text, cluster_id::text, subject, payload, payload_format, timeout_ms, enabled, created_at, updated_at
		FROM request_reply_probes
		WHERE cluster_id = $1 AND enabled = TRUE
		ORDER BY subject ASC`

	queryGetRequestReplyProbe = `
		SELECT id::text, cluster_id::text, subject, payload, payload_format, timeout_ms, enabled, created_at, updated_at
		FROM request_reply_probes
		WHERE cluster_id = $1 AND id = $2`

	queryInsertRequestReplyProbe = `
		INSERT INTO request_reply_probes (
			id, cluster_id, subject, payload, payload_format, timeout_ms, enabled
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	queryUpdateRequestReplyProbe = `
		UPDATE request_reply_probes
		SET subject = $3, payload = $4, payload_format = $5, timeout_ms = $6, enabled = $7, updated_at = NOW()
		WHERE cluster_id = $1 AND id = $2`

	queryDeleteRequestReplyProbe = `
		DELETE FROM request_reply_probes WHERE cluster_id = $1 AND id = $2`
)
