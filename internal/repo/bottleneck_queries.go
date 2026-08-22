package repo

const (
	queryUpsertBottleneckHourRollupsPrefix = `INSERT INTO bottleneck_hour_rollups
		(cluster_id, stream_name, consumer_name, bucket_hour, avg_lag, max_lag, avg_payload_bytes, avg_processing_ms, samples)
		VALUES `

	queryUpsertBottleneckHourRollupsSuffix = ` ON CONFLICT (cluster_id, stream_name, consumer_name, bucket_hour) DO UPDATE SET
		avg_lag = CASE
			WHEN bottleneck_hour_rollups.samples + EXCLUDED.samples = 0 THEN 0
			ELSE (bottleneck_hour_rollups.avg_lag * bottleneck_hour_rollups.samples + EXCLUDED.avg_lag * EXCLUDED.samples)
				/ (bottleneck_hour_rollups.samples + EXCLUDED.samples)
		END,
		max_lag = GREATEST(bottleneck_hour_rollups.max_lag, EXCLUDED.max_lag),
		avg_payload_bytes = CASE
			WHEN EXCLUDED.avg_payload_bytes <= 0 THEN bottleneck_hour_rollups.avg_payload_bytes
			WHEN bottleneck_hour_rollups.avg_payload_bytes <= 0 THEN EXCLUDED.avg_payload_bytes
			WHEN bottleneck_hour_rollups.samples + EXCLUDED.samples = 0 THEN 0
			ELSE (bottleneck_hour_rollups.avg_payload_bytes * bottleneck_hour_rollups.samples
				+ EXCLUDED.avg_payload_bytes * EXCLUDED.samples)
				/ (bottleneck_hour_rollups.samples + EXCLUDED.samples)
		END,
		avg_processing_ms = CASE
			WHEN EXCLUDED.avg_processing_ms IS NULL THEN bottleneck_hour_rollups.avg_processing_ms
			WHEN bottleneck_hour_rollups.avg_processing_ms IS NULL THEN EXCLUDED.avg_processing_ms
			WHEN bottleneck_hour_rollups.samples + EXCLUDED.samples = 0 THEN EXCLUDED.avg_processing_ms
			ELSE (bottleneck_hour_rollups.avg_processing_ms * bottleneck_hour_rollups.samples
				+ EXCLUDED.avg_processing_ms * EXCLUDED.samples)
				/ (bottleneck_hour_rollups.samples + EXCLUDED.samples)
		END,
		samples = bottleneck_hour_rollups.samples + EXCLUDED.samples`

	queryListBottleneckHourRollups = `
		SELECT cluster_id, stream_name, consumer_name, bucket_hour,
		       avg_lag, max_lag, avg_payload_bytes, avg_processing_ms, samples
		FROM bottleneck_hour_rollups
		WHERE cluster_id = $1
		  AND bucket_hour >= $2
		  AND bucket_hour <= $3
		ORDER BY bucket_hour ASC`

	queryDeleteBottleneckHourRollupsOlderThan = `DELETE FROM bottleneck_hour_rollups WHERE bucket_hour < $1`
)
