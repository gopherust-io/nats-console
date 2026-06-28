package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/jackc/pgx/v5"
)

type MetricSampleRow struct {
	Metric string
	Value  float64
}

func (s *Store) InsertMetricSamples(ctx context.Context, clusterID string, capturedAt time.Time, samples []MetricSampleRow) error {
	if len(samples) == 0 {
		return nil
	}
	capturedAt = capturedAt.UTC().Truncate(time.Second)

	batch := &pgx.Batch{}
	for _, sample := range samples {
		batch.Queue(`
			INSERT INTO cluster_metric_samples (cluster_id, captured_at, metric, value)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (cluster_id, captured_at, metric) DO UPDATE SET value = EXCLUDED.value`,
			clusterID, capturedAt, sample.Metric, sample.Value)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()
	for range samples {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return br.Close()
}

func (s *Store) DeleteMetricSamplesOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM cluster_metric_samples
		WHERE captured_at < $1`, cutoff.UTC())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *Store) QueryMetricSeries(
	ctx context.Context,
	clusterID string,
	metrics []string,
	from, to time.Time,
	step time.Duration,
) (map[string][]domain.MetricPoint, error) {
	if len(metrics) == 0 {
		return map[string][]domain.MetricPoint{}, nil
	}
	from = from.UTC()
	to = to.UTC()

	var query string
	var args []any
	if step <= 0 {
		query = `
			SELECT metric, captured_at, value
			FROM cluster_metric_samples
			WHERE cluster_id = $1
			  AND metric = ANY($2)
			  AND captured_at >= $3
			  AND captured_at <= $4
			ORDER BY metric, captured_at`
		args = []any{clusterID, metrics, from, to}
	} else {
		secs := int64(step.Seconds())
		if secs <= 0 {
			secs = 60
		}
		query = `
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
		args = []any{clusterID, metrics, from, to, secs, counterMetricNames(metrics)}
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]domain.MetricPoint, len(metrics))
	for rows.Next() {
		var metric string
		var ts time.Time
		var value float64
		if err := rows.Scan(&metric, &ts, &value); err != nil {
			return nil, err
		}
		out[metric] = append(out[metric], domain.MetricPoint{T: ts.UTC(), V: value})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func counterMetricNames(metrics []string) []string {
	out := make([]string, 0)
	for _, m := range metrics {
		if domain.IsCounterMetric(m) {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

func ParseMetricsStep(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return 0, nil
	}
	switch raw {
	case "1m":
		return time.Minute, nil
	case "5m":
		return 5 * time.Minute, nil
	case "15m":
		return 15 * time.Minute, nil
	case "1h":
		return time.Hour, nil
	case "1d":
		return 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported step %q", raw)
	}
}

func DefaultMetricsStep(from, to time.Time) time.Duration {
	rangeDur := to.Sub(from)
	switch {
	case rangeDur <= 2*time.Hour:
		return time.Minute
	case rangeDur <= 24*time.Hour:
		return 5 * time.Minute
	case rangeDur <= 7*24*time.Hour:
		return time.Hour
	default:
		return 24 * time.Hour
	}
}
