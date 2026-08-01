package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

// InsertIncidentAnnotation stores a deploy/change annotation for a cluster.
func (s *Store) InsertIncidentAnnotation(ctx context.Context, clusterID string, in domain.IncidentAnnotationCreate) (domain.IncidentAnnotation, error) {
	if err := in.Validate(); err != nil {
		return domain.IncidentAnnotation{}, err
	}
	occurredAt := time.Now().UTC()
	if in.OccurredAt != nil && !in.OccurredAt.IsZero() {
		occurredAt = in.OccurredAt.UTC()
	}
	id := newID()
	typ := in.NormalizedType()
	title := strings.TrimSpace(in.Title)
	details := strings.TrimSpace(in.Details)
	_, err := s.pool.Exec(ctx, queryInsertIncidentAnnotation,
		id, clusterID, occurredAt, typ, title, details)
	if err != nil {
		return domain.IncidentAnnotation{}, err
	}
	return domain.IncidentAnnotation{
		ID:         id,
		ClusterID:  clusterID,
		OccurredAt: occurredAt,
		Type:       typ,
		Title:      title,
		Details:    details,
	}, nil
}

// ListIncidentAnnotations returns annotations in [from, to] for a cluster.
func (s *Store) ListIncidentAnnotations(ctx context.Context, clusterID string, from, to time.Time) ([]domain.IncidentAnnotation, error) {
	rows, err := s.pool.Query(ctx, queryListIncidentAnnotations, clusterID, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.IncidentAnnotation
	for rows.Next() {
		var a domain.IncidentAnnotation
		if err := rows.Scan(&a.ID, &a.ClusterID, &a.OccurredAt, &a.Type, &a.Title, &a.Details); err != nil {
			return nil, err
		}
		a.OccurredAt = a.OccurredAt.UTC()
		out = append(out, a)
	}
	if out == nil {
		out = []domain.IncidentAnnotation{}
	}
	return out, rows.Err()
}

// InsertIncidentConsumerSamples bulk-inserts per-consumer scrape rows.
func (s *Store) InsertIncidentConsumerSamples(ctx context.Context, clusterID string, capturedAt time.Time, samples []domain.IncidentConsumerSample) error {
	if len(samples) == 0 {
		return nil
	}
	capturedAt = capturedAt.UTC().Truncate(time.Second)
	if err := s.ensureDayPartition(ctx, incidentSamplesParent, capturedAt); err != nil {
		return err
	}

	var b strings.Builder
	args := make([]any, 0, 2+6*len(samples))
	args = append(args, clusterID, capturedAt)
	b.WriteString(queryInsertIncidentConsumerSamplesPrefix)
	for i, sample := range samples {
		if i > 0 {
			b.WriteByte(',')
		}
		base := i*6 + 3
		fmt.Fprintf(&b, "($1,$2,$%d,$%d,$%d,$%d,$%d,$%d)", base, base+1, base+2, base+3, base+4, base+5)
		args = append(args,
			sample.StreamName,
			sample.ConsumerName,
			sample.Lag,
			sample.NumRedelivered,
			sample.DeliveredSeq,
			sample.AckFloorSeq,
		)
	}
	b.WriteString(queryInsertIncidentConsumerSamplesSuffix)
	_, err := s.pool.Exec(ctx, b.String(), args...)
	return err
}

// ListIncidentConsumerSamples returns samples for one consumer in [from, to].
func (s *Store) ListIncidentConsumerSamples(
	ctx context.Context,
	clusterID, stream, consumer string,
	from, to time.Time,
) ([]domain.IncidentConsumerSample, error) {
	rows, err := s.pool.Query(ctx, queryListIncidentConsumerSamples,
		clusterID, stream, consumer, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.IncidentConsumerSample
	for rows.Next() {
		var srow domain.IncidentConsumerSample
		if err := rows.Scan(
			&srow.CapturedAt,
			&srow.StreamName,
			&srow.ConsumerName,
			&srow.Lag,
			&srow.NumRedelivered,
			&srow.DeliveredSeq,
			&srow.AckFloorSeq,
		); err != nil {
			return nil, err
		}
		srow.CapturedAt = srow.CapturedAt.UTC()
		out = append(out, srow)
	}
	if out == nil {
		out = []domain.IncidentConsumerSample{}
	}
	return out, rows.Err()
}

// InsertIncidentNodeEvents persists disconnect/reconnect transitions.
func (s *Store) InsertIncidentNodeEvents(ctx context.Context, clusterID string, events []domain.IncidentNodeEvent) error {
	if len(events) == 0 {
		return nil
	}
	var b strings.Builder
	args := make([]any, 0, 1+3*len(events))
	args = append(args, clusterID)
	b.WriteString(queryInsertIncidentNodeEventsPrefix)
	for i, ev := range events {
		if i > 0 {
			b.WriteByte(',')
		}
		base := i*3 + 2
		fmt.Fprintf(&b, "($1,$%d,$%d,$%d)", base, base+1, base+2)
		args = append(args, ev.OccurredAt.UTC().Truncate(time.Second), strings.TrimSpace(ev.NodeName), ev.EventType)
	}
	b.WriteString(queryInsertIncidentNodeEventsSuffix)
	_, err := s.pool.Exec(ctx, b.String(), args...)
	return err
}

// ListIncidentNodeEvents returns node transitions in [from, to].
func (s *Store) ListIncidentNodeEvents(ctx context.Context, clusterID string, from, to time.Time) ([]domain.IncidentNodeEvent, error) {
	rows, err := s.pool.Query(ctx, queryListIncidentNodeEvents, clusterID, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.IncidentNodeEvent
	for rows.Next() {
		var ev domain.IncidentNodeEvent
		if err := rows.Scan(&ev.OccurredAt, &ev.NodeName, &ev.EventType); err != nil {
			return nil, err
		}
		ev.OccurredAt = ev.OccurredAt.UTC()
		out = append(out, ev)
	}
	if out == nil {
		out = []domain.IncidentNodeEvent{}
	}
	return out, rows.Err()
}

// ListAuditInRange returns consol mutations for a cluster in [from, to].
func (s *Store) ListAuditInRange(ctx context.Context, clusterID string, from, to time.Time, limit int) ([]domain.IncidentAuditChange, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := s.pool.Query(ctx, queryListAuditInRange, clusterID, from.UTC(), to.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.IncidentAuditChange
	for rows.Next() {
		var a domain.IncidentAuditChange
		if err := rows.Scan(&a.Timestamp, &a.Actor, &a.Action, &a.ResourceType, &a.ResourceName); err != nil {
			return nil, err
		}
		a.Timestamp = a.Timestamp.UTC()
		out = append(out, a)
	}
	if out == nil {
		out = []domain.IncidentAuditChange{}
	}
	return out, rows.Err()
}

// DeleteIncidentDataOlderThan purges reconstruction tables older than cutoff.
func (s *Store) DeleteIncidentDataOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	cutoff = cutoff.UTC()
	total, err := s.dropParentPartitionsOlderThan(ctx, incidentSamplesParent, cutoff)
	if err != nil {
		return total, err
	}
	for _, q := range []string{
		queryDeleteIncidentNodeEventsOlderThan,
		queryDeleteIncidentAnnotationsOlderThan,
	} {
		tag, err := s.pool.Exec(ctx, q, cutoff)
		if err != nil {
			return total, err
		}
		total += tag.RowsAffected()
	}
	return total, nil
}
