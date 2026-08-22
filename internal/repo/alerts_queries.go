package repo

const (
	queryListAlertRulesBase = `
		SELECT id, COALESCE(cluster_id::text, ''), COALESCE(account_name, ''),
		       name, message, severity, metric, comparator, threshold, enabled,
		       created_by, created_at, updated_at
		FROM alert_rules `

	queryListAlertRulesOrder = `
		ORDER BY enabled DESC, name ASC`

	queryGetAlertRule = `
		SELECT id, COALESCE(cluster_id::text, ''), COALESCE(account_name, ''),
		       name, message, severity, metric, comparator, threshold, enabled,
		       created_by, created_at, updated_at
		FROM alert_rules WHERE id = $1`

	queryInsertAlertRule = `
		INSERT INTO alert_rules (
			id, cluster_id, account_name, name, message, severity, metric, comparator, threshold, enabled, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	queryUpdateAlertRule = `
		UPDATE alert_rules SET
			cluster_id = $2, account_name = $3, name = $4, message = $5, severity = $6,
			metric = $7, comparator = $8, threshold = $9, enabled = $10, updated_at = now()
		WHERE id = $1`

	queryDeleteAlertRule = `DELETE FROM alert_rules WHERE id = $1`

	queryCountAlerts = `SELECT COUNT(*) FROM alerts a `

	alertSelectFrom = `
		SELECT a.id, a.rule_id, a.cluster_id::text, COALESCE(a.account_name, ''),
		       a.status, a.severity, a.metric, a.message, a.firing_value, a.threshold,
		       a.first_seen_at, a.last_seen_at, a.closed_at, a.acknowledged_at, COALESCE(a.acknowledged_by, ''),
		       COALESCE(r.name, '')
		FROM alerts a
		LEFT JOIN alert_rules r ON r.id = a.rule_id`

	queryListAlerts = alertSelectFrom + `
		%s
		ORDER BY a.last_seen_at DESC
		LIMIT $%d OFFSET $%d`

	queryListOpenUnacknowledgedAlerts = alertSelectFrom + `
		%s
		ORDER BY a.last_seen_at DESC
		LIMIT $%d`

	queryGetAlert = `
		SELECT a.id, a.rule_id, a.cluster_id::text, COALESCE(a.account_name, ''),
		       a.status, a.severity, a.metric, a.message, a.firing_value, a.threshold,
		       a.first_seen_at, a.last_seen_at, a.closed_at, a.acknowledged_at, COALESCE(a.acknowledged_by, ''),
		       COALESCE(r.name, '')
		FROM alerts a
		LEFT JOIN alert_rules r ON r.id = a.rule_id
		WHERE a.id = $1`

	queryAcknowledgeAlert = `
		UPDATE alerts SET acknowledged_at = now(), acknowledged_by = $2
		WHERE id = $1 AND status = 'open' AND acknowledged_at IS NULL`

	queryUpsertOpenAlert = `
		INSERT INTO alerts (
			id, rule_id, cluster_id, account_name, status, severity, metric, message,
			firing_value, threshold, first_seen_at, last_seen_at
		) VALUES (
			$1, $2, $3, NULLIF($4, ''), 'open', $5, $6, $7, $8, $9, $10, $10
		)
		ON CONFLICT (rule_id, cluster_id) WHERE status = 'open'
		DO UPDATE SET
			firing_value = EXCLUDED.firing_value,
			threshold = EXCLUDED.threshold,
			severity = EXCLUDED.severity,
			message = EXCLUDED.message,
			metric = EXCLUDED.metric,
			last_seen_at = EXCLUDED.last_seen_at
		RETURNING id, (xmax = 0)`

	queryClaimAlertEmailNotify = `
		UPDATE alerts SET email_notified_at = now()
		WHERE id = $1 AND status = 'open' AND email_notified_at IS NULL`

	queryReleaseAlertEmailNotify = `
		UPDATE alerts SET email_notified_at = NULL
		WHERE id = $1 AND status = 'open'`

	queryCloseOpenAlert = `
		UPDATE alerts SET status = 'closed', closed_at = $3, last_seen_at = $3
		WHERE rule_id = $1 AND cluster_id = $2 AND status = 'open'`

	queryListOpenAlertRuleIDs = `
		SELECT rule_id FROM alerts WHERE cluster_id = $1 AND status = 'open'`

	queryDeleteClosedAlertsOlderThan = `
		DELETE FROM alerts
		WHERE status = 'closed' AND last_seen_at < $1`
)
