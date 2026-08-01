package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type AuditEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	ID           string    `json:"id"`
	Actor        string    `json:"actor"`
	Action       string    `json:"action"`
	ClusterID    string    `json:"cluster_id"`
	ResourceType string    `json:"resource_type"`
	ResourceName string    `json:"resource_name"`
	RequestID    string    `json:"request_id"`
	IP           string    `json:"ip"`
	Details      []byte    `json:"details"`
}

type AuditRequestDetails struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"`
}

type AuditCreate struct {
	Actor        string
	Action       string
	ClusterID    string
	ResourceType string
	ResourceName string
	RequestID    string
	IP           string
	Details      AuditRequestDetails
}

type AuditFilter struct {
	ClusterID  string
	ClusterIDs []string
	Limit      int
	Offset     int
}

func (s *Store) InsertAudit(ctx context.Context, in AuditCreate) error {
	details, err := serializer.Marshal(in.Details)
	if err != nil {
		details = commonstrings.StringToBytes("{}")
	}
	_, err = s.pool.Exec(ctx, queryInsertAudit,
		newID(), in.Actor, in.Action, in.ClusterID, in.ResourceType, in.ResourceName, in.RequestID, details, in.IP)
	return err
}

func (s *Store) ListAudit(ctx context.Context, f AuditFilter) ([]AuditEntry, int, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 500 {
		f.Limit = 500
	}

	args := []any{}
	where := "WHERE 1=1"
	if !commonstrings.IsEmpty(f.ClusterID) {
		args = append(args, f.ClusterID)
		where += fmt.Sprintf(" AND cluster_id = $%d", len(args))
	} else if f.ClusterIDs != nil {
		if len(f.ClusterIDs) == 0 {
			return []AuditEntry{}, 0, nil
		}
		placeholders := make([]string, len(f.ClusterIDs))
		for i, clusterID := range f.ClusterIDs {
			args = append(args, clusterID)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		where += " AND cluster_id IN (" + strings.Join(placeholders, ",") + ")"
	}

	var total int
	if err := s.pool.QueryRow(ctx, queryCountAudit+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, f.Limit, f.Offset)
	query := fmt.Sprintf(queryListAudit, where, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err = rows.Scan(
			&e.ID,
			&e.Timestamp,
			&e.Actor,
			&e.Action,
			&e.ClusterID,
			&e.ResourceType,
			&e.ResourceName,
			&e.RequestID,
			&e.Details,
			&e.IP); err != nil {
			return nil, 0, err
		}
		if len(e.Details) == 0 {
			e.Details = commonstrings.StringToBytes("{}")
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []AuditEntry{}
	}
	return entries, total, rows.Err()
}

// DeleteAuditOlderThan removes audit_log rows older than cutoff.
func (s *Store) DeleteAuditOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, queryDeleteAuditOlderThan, cutoff.UTC())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
