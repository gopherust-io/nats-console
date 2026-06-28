package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AuditEntry struct {
	ID           string          `json:"id"`
	Timestamp    time.Time       `json:"timestamp"`
	Actor        string          `json:"actor"`
	Action       string          `json:"action"`
	ClusterID    string          `json:"cluster_id"`
	ResourceType string          `json:"resource_type"`
	ResourceName string          `json:"resource_name"`
	RequestID    string          `json:"request_id"`
	Details      json.RawMessage `json:"details"`
	IP           string          `json:"ip"`
}

type AuditCreate struct {
	Actor        string
	Action       string
	ClusterID    string
	ResourceType string
	ResourceName string
	RequestID    string
	Details      map[string]any
	IP           string
}

type AuditFilter struct {
	ClusterID string
	Limit     int
	Offset    int
}

func (s *Store) InsertAudit(ctx context.Context, in AuditCreate) error {
	details, err := json.Marshal(in.Details)
	if err != nil {
		details = []byte("{}")
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_log (id, actor, action, cluster_id, resource_type, resource_name, request_id, details, ip)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		uuid.New().String(), in.Actor, in.Action, in.ClusterID, in.ResourceType, in.ResourceName, in.RequestID, details, in.IP)
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
	if f.ClusterID != "" {
		args = append(args, f.ClusterID)
		where += fmt.Sprintf(" AND cluster_id = $%d", len(args))
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM audit_log " + where
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, f.Limit, f.Offset)
	query := fmt.Sprintf(`
		SELECT id, timestamp, actor, action, cluster_id, resource_type, resource_name, request_id, details, ip
		FROM audit_log %s
		ORDER BY timestamp DESC
		LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Actor, &e.Action, &e.ClusterID, &e.ResourceType, &e.ResourceName, &e.RequestID, &e.Details, &e.IP); err != nil {
			return nil, 0, err
		}
		if len(e.Details) == 0 {
			e.Details = json.RawMessage("{}")
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []AuditEntry{}
	}
	return entries, total, rows.Err()
}

func scanAuditRow(row pgx.Row) (AuditEntry, error) {
	var e AuditEntry
	err := row.Scan(&e.ID, &e.Timestamp, &e.Actor, &e.Action, &e.ClusterID, &e.ResourceType, &e.ResourceName, &e.RequestID, &e.Details, &e.IP)
	return e, err
}
