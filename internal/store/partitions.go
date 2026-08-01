package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	samplePartitionForwardDays = 2
	metricSamplesParent        = "cluster_metric_samples"
	incidentSamplesParent      = "incident_consumer_samples"
)

// EnsureSamplePartitions creates daily partitions covering
// [now-retention, now+forward] for metrics and incident consumer samples.
func (s *Store) EnsureSamplePartitions(ctx context.Context, now time.Time, retention time.Duration) error {
	now = now.UTC()
	start := now.Add(-retention).Truncate(24 * time.Hour)
	if retention <= 0 {
		start = now.Truncate(24 * time.Hour)
	}
	end := now.Truncate(24 * time.Hour).Add(samplePartitionForwardDays * 24 * time.Hour)
	for d := start; !d.After(end); d = d.Add(24 * time.Hour) {
		if err := s.ensureDayPartition(ctx, metricSamplesParent, d); err != nil {
			return err
		}
		if err := s.ensureDayPartition(ctx, incidentSamplesParent, d); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureDayPartition(ctx context.Context, parent string, day time.Time) error {
	day = day.UTC().Truncate(24 * time.Hour)
	key := parent + "|" + day.Format("2006-01-02")
	if _, ok := s.ensuredPartitions.Load(key); ok {
		return nil
	}
	name := fmt.Sprintf("%s_%s", parent, day.Format("2006_01_02"))
	from := day
	to := day.Add(24 * time.Hour)
	_, err := s.pool.Exec(ctx, fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM (%s) TO (%s)`,
		quoteIdent(name),
		quoteIdent(parent),
		quoteTimestamp(from),
		quoteTimestamp(to),
	))
	if err != nil {
		return err
	}
	s.ensuredPartitions.Store(key, struct{}{})
	return nil
}

// DropSamplePartitionsOlderThan drops daily partitions whose upper bound is <= cutoff.
// Returns an estimate of dropped partition count (not row count).
func (s *Store) DropSamplePartitionsOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	cutoff = cutoff.UTC()
	var dropped int64
	for _, parent := range []string{metricSamplesParent, incidentSamplesParent} {
		n, err := s.dropParentPartitionsOlderThan(ctx, parent, cutoff)
		if err != nil {
			return dropped, err
		}
		dropped += n
	}
	return dropped, nil
}

func (s *Store) dropParentPartitionsOlderThan(ctx context.Context, parent string, cutoff time.Time) (int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.relname::text AS partition_name,
		       pg_get_expr(c.relpartbound, c.oid) AS bound
		FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		JOIN pg_class p ON p.oid = i.inhparent
		WHERE p.relname = $1
		  AND c.relkind = 'r'`, parent)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var dropped int64
	for rows.Next() {
		var name, bound string
		if err := rows.Scan(&name, &bound); err != nil {
			return dropped, err
		}
		upper, ok := partitionUpperBound(bound)
		if !ok {
			continue
		}
		// Drop when the partition's exclusive upper bound is at or before cutoff
		// (entire partition is older than retention).
		if !upper.After(cutoff) {
			if _, err := s.pool.Exec(ctx, "DROP TABLE IF EXISTS "+quoteIdent(name)); err != nil {
				return dropped, err
			}
			dropped++
		}
	}
	return dropped, rows.Err()
}

func partitionUpperBound(bound string) (time.Time, bool) {
	// Example: FOR VALUES FROM ('2026-07-30 00:00:00+00') TO ('2026-07-31 00:00:00+00')
	const marker = " TO ("
	_, after, ok := strings.Cut(bound, marker)
	if !ok {
		return time.Time{}, false
	}
	rest := after
	before0, _, ok0 := strings.Cut(rest, ")")
	if !ok0 {
		return time.Time{}, false
	}
	raw := strings.TrimSpace(before0)
	raw = strings.Trim(raw, "'\"")
	raw = normalizePGTimestamptz(raw)
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05-07",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func normalizePGTimestamptz(raw string) string {
	if strings.HasSuffix(raw, "+00") || strings.HasSuffix(raw, "-00") {
		return raw + ":00"
	}
	return raw
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func quoteTimestamp(t time.Time) string {
	return "'" + t.UTC().Format("2006-01-02 15:04:05+00") + "'::timestamptz"
}
