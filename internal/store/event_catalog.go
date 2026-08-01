package store

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

var ErrEventCatalogEntryNotFound = domain.ErrEventCatalogEntryNotFound

// EventCatalogEntryRow is a persisted catalog enrichment.
type EventCatalogEntryRow struct {
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Description      string
	ClusterID        string
	Subject          string
	Owner            string
	ID               string
	SuccessorSubject string
	DeprecationNote  string
	UpdatedBy        string
	Schema           []byte
	Example          []byte
	Deprecated       bool
}

func (s *Store) ListEventCatalogEntries(ctx context.Context, clusterID string) ([]domain.EventCatalogDoc, error) {
	rows, err := s.pool.Query(ctx, queryListEventCatalogEntries, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.EventCatalogDoc, 0)
	for rows.Next() {
		var doc domain.EventCatalogDoc
		var schema, example []byte
		if err := rows.Scan(
			&doc.Subject, &doc.Owner, &doc.Description, &schema, &example,
			&doc.Deprecated, &doc.SuccessorSubject, &doc.DeprecationNote,
			&doc.UpdatedBy, &doc.CreatedAt, &doc.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if len(schema) > 0 {
			doc.Schema = schema
		}
		if len(example) > 0 {
			doc.Example = example
		}
		out = append(out, doc)
	}
	return out, rows.Err()
}

func (s *Store) GetEventCatalogEntry(ctx context.Context, clusterID, subject string) (EventCatalogEntryRow, error) {
	row := s.pool.QueryRow(ctx, queryGetEventCatalogEntry, clusterID, subject)
	var r EventCatalogEntryRow
	var schema, example []byte
	err := row.Scan(
		&r.ID, &r.ClusterID, &r.Subject, &r.Owner, &r.Description, &schema, &example,
		&r.Deprecated, &r.SuccessorSubject, &r.DeprecationNote,
		&r.UpdatedBy, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return EventCatalogEntryRow{}, ErrEventCatalogEntryNotFound
	}
	if err != nil {
		return EventCatalogEntryRow{}, err
	}
	if len(schema) > 0 {
		r.Schema = schema
	}
	if len(example) > 0 {
		r.Example = example
	}
	return r, nil
}

func (s *Store) UpsertEventCatalogEntry(
	ctx context.Context,
	clusterID, subject, updatedBy string,
	in domain.EventCatalogUpsert,
) (EventCatalogEntryRow, error) {
	canonical, err := domain.CanonicalEventCatalogSubject(subject)
	if err != nil {
		return EventCatalogEntryRow{}, err
	}
	if err := domain.ValidateEventCatalogSchema(in.Schema); err != nil {
		return EventCatalogEntryRow{}, err
	}
	if err := domain.ValidateEventCatalogExample(in.Example); err != nil {
		return EventCatalogEntryRow{}, err
	}

	owner := truncateRunes(strings.TrimSpace(in.Owner), 256)
	description := truncateRunes(strings.TrimSpace(in.Description), 4000)
	successor := truncateRunes(strings.TrimSpace(in.SuccessorSubject), 512)
	note := truncateRunes(strings.TrimSpace(in.DeprecationNote), 4000)
	schema := normalizeStoredJSON(in.Schema)
	example := normalizeStoredJSON(in.Example)

	id := newID()
	var updatedByArg any
	if !commonstrings.IsEmpty(updatedBy) {
		updatedByArg = updatedBy
	}

	_, err = s.pool.Exec(ctx, queryUpsertEventCatalogEntry,
		id, clusterID, canonical, owner, description, schema, example,
		in.Deprecated, successor, note, updatedByArg,
	)
	if err != nil {
		return EventCatalogEntryRow{}, err
	}
	return s.GetEventCatalogEntry(ctx, clusterID, canonical)
}

func (s *Store) DeleteEventCatalogEntry(ctx context.Context, clusterID, subject string) error {
	canonical, err := domain.CanonicalEventCatalogSubject(subject)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, queryDeleteEventCatalogEntry, clusterID, canonical)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrEventCatalogEntryNotFound
	}
	return nil
}

func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

func normalizeStoredJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(commonstrings.BytesToString(raw))
	if commonstrings.IsEmpty(trimmed) || trimmed == "null" {
		return nil
	}
	return commonstrings.StringToBytes(trimmed)
}
