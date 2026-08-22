package repo

const (
	queryUpsertArchitectureScoreDaily = `INSERT INTO architecture_score_daily
		(cluster_id, score_day, score, factors, avg_lag, captured_at)
		VALUES ($1, $2::date, $3, $4::jsonb, $5, $6)
		ON CONFLICT (cluster_id, score_day) DO UPDATE SET
			score = EXCLUDED.score,
			factors = EXCLUDED.factors,
			avg_lag = EXCLUDED.avg_lag,
			captured_at = EXCLUDED.captured_at`

	queryListArchitectureScoreDaily = `
		SELECT cluster_id, score_day, score, factors, avg_lag, captured_at
		FROM architecture_score_daily
		WHERE cluster_id = $1
		  AND score_day >= $2::date
		  AND score_day <= $3::date
		ORDER BY score_day ASC`

	queryGetArchitectureScoreDaily = `
		SELECT cluster_id, score_day, score, factors, avg_lag, captured_at
		FROM architecture_score_daily
		WHERE cluster_id = $1 AND score_day = $2::date`

	queryDeleteArchitectureScoreDailyOlderThan = `DELETE FROM architecture_score_daily WHERE score_day < $1::date`
)
