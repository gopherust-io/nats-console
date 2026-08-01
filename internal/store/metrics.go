package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
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
	if err := s.ensureDayPartition(ctx, metricSamplesParent, capturedAt); err != nil {
		return err
	}

	var b strings.Builder
	args := make([]any, 0, 2+2*len(samples))
	args = append(args, clusterID, capturedAt)
	b.WriteString(queryInsertMetricSamplesPrefix)
	for i, sample := range samples {
		if i > 0 {
			b.WriteByte(',')
		}
		base := i*2 + 3
		fmt.Fprintf(&b, "($1,$2,$%d,$%d)", base, base+1)
		args = append(args, sample.Metric, sample.Value)
	}
	b.WriteString(queryInsertMetricSamplesSuffix)
	_, err := s.pool.Exec(ctx, b.String(), args...)
	return err
}

// DeleteMetricSamplesOlderThan drops daily partitions fully older than cutoff.
func (s *Store) DeleteMetricSamplesOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	return s.dropParentPartitionsOlderThan(ctx, metricSamplesParent, cutoff.UTC())
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
		query = queryMetricSeriesRaw
		args = []any{clusterID, metrics, from, to}
	} else {
		secs := int64(step.Seconds())
		if secs <= 0 {
			secs = 60
		}
		query = queryMetricSeriesBucketed
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
	return domain.ParseMetricsStep(raw)
}

func DefaultMetricsStep(from, to time.Time) time.Duration {
	return domain.DefaultMetricsStep(from, to)
}
