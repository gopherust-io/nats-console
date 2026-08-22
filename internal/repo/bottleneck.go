package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

// UpsertBottleneckHourBuckets merges scrape observations into hourly rollups.
func (db *DB) UpsertBottleneckHourBuckets(ctx context.Context, rows []domain.BottleneckHourBucket) error {
	if len(rows) == 0 {
		return nil
	}
	var b strings.Builder
	args := make([]any, 0, 9*len(rows))
	b.WriteString(queryUpsertBottleneckHourRollupsPrefix)
	for i, row := range rows {
		if i > 0 {
			b.WriteByte(',')
		}
		base := i*9 + 1
		fmt.Fprintf(&b, "($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base, base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8)
		samples := row.Samples
		if samples <= 0 {
			samples = 1
		}
		var proc any
		if row.AvgProcessingMs != nil {
			proc = *row.AvgProcessingMs
		}
		args = append(args,
			row.ClusterID,
			row.StreamName,
			row.ConsumerName,
			row.BucketHour.UTC().Truncate(time.Hour),
			row.AvgLag,
			row.MaxLag,
			row.AvgPayloadBytes,
			proc,
			samples,
		)
	}
	b.WriteString(queryUpsertBottleneckHourRollupsSuffix)
	_, err := db.pool.Exec(ctx, b.String(), args...)
	return err
}

// ListBottleneckHourBuckets returns rollups for a cluster in [from, to].
func (db *DB) ListBottleneckHourBuckets(
	ctx context.Context,
	clusterID string,
	from, to time.Time,
) ([]domain.BottleneckHourBucket, error) {
	rows, err := db.pool.Query(ctx, queryListBottleneckHourRollups, clusterID, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.BottleneckHourBucket
	for rows.Next() {
		var row domain.BottleneckHourBucket
		var proc *float64
		if err := rows.Scan(
			&row.ClusterID,
			&row.StreamName,
			&row.ConsumerName,
			&row.BucketHour,
			&row.AvgLag,
			&row.MaxLag,
			&row.AvgPayloadBytes,
			&proc,
			&row.Samples,
		); err != nil {
			return nil, err
		}
		row.BucketHour = row.BucketHour.UTC()
		row.AvgProcessingMs = proc
		out = append(out, row)
	}
	if out == nil {
		out = []domain.BottleneckHourBucket{}
	}
	return out, rows.Err()
}

// DeleteBottleneckHourBucketsOlderThan removes rollups older than cutoff.
func (db *DB) DeleteBottleneckHourBucketsOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := db.pool.Exec(ctx, queryDeleteBottleneckHourRollupsOlderThan, cutoff.UTC())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
