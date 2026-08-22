package repo

const (
	queryInsertMetricSamplesPrefix = `INSERT INTO cluster_metric_samples (cluster_id, captured_at, metric, value) VALUES `

	queryInsertMetricSamplesSuffix = ` ON CONFLICT (cluster_id, captured_at, metric) DO UPDATE SET value = EXCLUDED.value`

	queryMetricSeriesRaw = `
			SELECT metric, captured_at, value
			FROM cluster_metric_samples
			WHERE cluster_id = $1
			  AND metric = ANY($2)
			  AND captured_at >= $3
			  AND captured_at <= $4
			ORDER BY metric, captured_at`

	queryMetricSeriesBucketed = `
			SELECT metric,
			       to_timestamp(floor(extract(epoch from captured_at) / $5) * $5) AS bucket,
			       CASE
			         WHEN metric = ANY($6) THEN max(value)
			         ELSE avg(value)
			       END AS agg_value
			FROM cluster_metric_samples
			WHERE cluster_id = $1
			  AND metric = ANY($2)
			  AND captured_at >= $3
			  AND captured_at <= $4
			GROUP BY metric, bucket
			ORDER BY metric, bucket`
)
