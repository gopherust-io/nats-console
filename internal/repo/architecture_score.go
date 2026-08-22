package repo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// UpsertArchitectureScoreDaily overwrites today's score for a cluster.
func (db *DB) UpsertArchitectureScoreDaily(ctx context.Context, row domain.ArchitectureScoreDailyRow) error {
	factors := domain.NormalizeArchitectureScoreFactors(row.Factors)
	raw, err := json.Marshal(factors)
	if err != nil {
		return err
	}
	day := row.ScoreDay.UTC().Truncate(24 * time.Hour)
	captured := row.CapturedAt.UTC()
	if captured.IsZero() {
		captured = time.Now().UTC()
	}
	_, err = db.pool.Exec(ctx, queryUpsertArchitectureScoreDaily,
		row.ClusterID,
		day,
		row.Score,
		commonstrings.BytesToString(raw),
		row.AvgLag,
		captured,
	)
	return err
}

// GetArchitectureScoreDaily returns one day, or false if missing.
func (db *DB) GetArchitectureScoreDaily(ctx context.Context, clusterID string, day time.Time) (domain.ArchitectureScoreDailyRow, bool, error) {
	var row domain.ArchitectureScoreDailyRow
	var factorsRaw []byte
	err := db.pool.QueryRow(ctx, queryGetArchitectureScoreDaily, clusterID, day.UTC()).Scan(
		&row.ClusterID,
		&row.ScoreDay,
		&row.Score,
		&factorsRaw,
		&row.AvgLag,
		&row.CapturedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ArchitectureScoreDailyRow{}, false, nil
		}
		return domain.ArchitectureScoreDailyRow{}, false, err
	}
	row.ScoreDay = row.ScoreDay.UTC()
	row.CapturedAt = row.CapturedAt.UTC()
	row.Factors = decodeScoreFactors(factorsRaw)
	return row, true, nil
}

// ListArchitectureScoreDaily returns rows in [from, to] inclusive by date.
func (db *DB) ListArchitectureScoreDaily(
	ctx context.Context,
	clusterID string,
	from, to time.Time,
) ([]domain.ArchitectureScoreDailyRow, error) {
	rows, err := db.pool.Query(ctx, queryListArchitectureScoreDaily, clusterID, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.ArchitectureScoreDailyRow, 0)
	for rows.Next() {
		var row domain.ArchitectureScoreDailyRow
		var factorsRaw []byte
		if err := rows.Scan(
			&row.ClusterID,
			&row.ScoreDay,
			&row.Score,
			&factorsRaw,
			&row.AvgLag,
			&row.CapturedAt,
		); err != nil {
			return nil, err
		}
		row.ScoreDay = row.ScoreDay.UTC()
		row.CapturedAt = row.CapturedAt.UTC()
		row.Factors = decodeScoreFactors(factorsRaw)
		out = append(out, row)
	}
	return out, rows.Err()
}

// DeleteArchitectureScoreDailyOlderThan removes scores older than cutoff date.
func (db *DB) DeleteArchitectureScoreDailyOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := db.pool.Exec(ctx, queryDeleteArchitectureScoreDailyOlderThan, cutoff.UTC())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func decodeScoreFactors(raw []byte) []domain.ArchitectureScoreFactor {
	if len(raw) == 0 {
		return []domain.ArchitectureScoreFactor{}
	}
	var factors []domain.ArchitectureScoreFactor
	if err := json.Unmarshal(raw, &factors); err != nil {
		return []domain.ArchitectureScoreFactor{}
	}
	return domain.NormalizeArchitectureScoreFactors(factors)
}
