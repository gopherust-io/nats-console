package store

const (
	queryInsertIncidentAnnotation = `
		INSERT INTO incident_annotations (id, cluster_id, occurred_at, type, title, details)
		VALUES ($1, $2, $3, $4, $5, $6)`

	queryListIncidentAnnotations = `
		SELECT id, cluster_id, occurred_at, type, title, details
		FROM incident_annotations
		WHERE cluster_id = $1
		  AND occurred_at >= $2
		  AND occurred_at <= $3
		ORDER BY occurred_at ASC`

	queryInsertIncidentConsumerSamplesPrefix = `INSERT INTO incident_consumer_samples
		(cluster_id, captured_at, stream_name, consumer_name, lag, num_redelivered, delivered_seq, ack_floor_seq)
		VALUES `

	queryInsertIncidentConsumerSamplesSuffix = ` ON CONFLICT (cluster_id, captured_at, stream_name, consumer_name) DO UPDATE SET
		lag = EXCLUDED.lag,
		num_redelivered = EXCLUDED.num_redelivered,
		delivered_seq = EXCLUDED.delivered_seq,
		ack_floor_seq = EXCLUDED.ack_floor_seq`

	queryListIncidentConsumerSamples = `
		SELECT captured_at, stream_name, consumer_name, lag, num_redelivered, delivered_seq, ack_floor_seq
		FROM incident_consumer_samples
		WHERE cluster_id = $1
		  AND stream_name = $2
		  AND consumer_name = $3
		  AND captured_at >= $4
		  AND captured_at <= $5
		ORDER BY captured_at ASC`

	queryInsertIncidentNodeEventsPrefix = `INSERT INTO incident_node_events (cluster_id, occurred_at, node_name, event_type) VALUES `

	queryInsertIncidentNodeEventsSuffix = ` ON CONFLICT (cluster_id, occurred_at, node_name, event_type) DO NOTHING`

	queryListIncidentNodeEvents = `
		SELECT occurred_at, node_name, event_type
		FROM incident_node_events
		WHERE cluster_id = $1
		  AND occurred_at >= $2
		  AND occurred_at <= $3
		ORDER BY occurred_at ASC`

	queryListAuditInRange = `
		SELECT timestamp, actor, action, resource_type, resource_name
		FROM audit_log
		WHERE cluster_id = $1
		  AND timestamp >= $2
		  AND timestamp <= $3
		ORDER BY timestamp ASC
		LIMIT $4`

	queryDeleteIncidentNodeEventsOlderThan = `DELETE FROM incident_node_events WHERE occurred_at < $1`

	queryDeleteIncidentAnnotationsOlderThan = `DELETE FROM incident_annotations WHERE occurred_at < $1`
)
