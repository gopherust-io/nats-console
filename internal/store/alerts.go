package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

var (
	ErrAlertRuleNotFound = domain.ErrAlertRuleNotFound
	ErrAlertNotFound     = domain.ErrAlertNotFound
)

func (s *Store) ListAlertRules(ctx context.Context, clusterID string, enabledOnly bool) ([]domain.AlertRule, error) {
	args := []any{}
	where := "WHERE 1=1"
	if !commonstrings.IsEmpty(clusterID) {
		args = append(args, clusterID)
		where += fmt.Sprintf(" AND (cluster_id IS NULL OR cluster_id = $%d)", len(args))
	}
	if enabledOnly {
		where += " AND enabled = true"
	}
	query := queryListAlertRulesBase + where + queryListAlertRulesOrder
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []domain.AlertRule
	for rows.Next() {
		var r domain.AlertRule
		if err := rows.Scan(
			&r.ID, &r.ClusterID, &r.AccountName,
			&r.Name, &r.Message, &r.Severity, &r.Metric, &r.Comparator, &r.Threshold, &r.Enabled,
			&r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	if rules == nil {
		rules = []domain.AlertRule{}
	}
	return rules, rows.Err()
}

func (s *Store) GetAlertRule(ctx context.Context, id string) (domain.AlertRule, error) {
	var r domain.AlertRule
	err := s.pool.QueryRow(ctx, queryGetAlertRule, id).Scan(
		&r.ID, &r.ClusterID, &r.AccountName,
		&r.Name, &r.Message, &r.Severity, &r.Metric, &r.Comparator, &r.Threshold, &r.Enabled,
		&r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AlertRule{}, ErrAlertRuleNotFound
	}
	return r, err
}

func (s *Store) CreateAlertRule(ctx context.Context, in domain.AlertRuleCreate, createdBy string) (domain.AlertRule, error) {
	id := newID()
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	message := in.Message
	if commonstrings.IsEmpty(message) {
		message = in.Name
	}
	var clusterArg any
	if !commonstrings.IsEmpty(in.ClusterID) {
		clusterArg = in.ClusterID
	}
	var accountArg any
	if !commonstrings.IsEmpty(in.AccountName) {
		accountArg = in.AccountName
	}
	_, err := s.pool.Exec(ctx, queryInsertAlertRule,
		id, clusterArg, accountArg, in.Name, message, in.Severity, in.Metric, in.Comparator, in.Threshold, enabled, createdBy,
	)
	if err != nil {
		return domain.AlertRule{}, err
	}
	return s.GetAlertRule(ctx, id)
}

func (s *Store) UpdateAlertRule(ctx context.Context, id string, in domain.AlertRuleUpdate) (domain.AlertRule, error) {
	cur, err := s.GetAlertRule(ctx, id)
	if err != nil {
		return domain.AlertRule{}, err
	}
	if in.Name != nil {
		cur.Name = *in.Name
	}
	if in.Message != nil {
		cur.Message = *in.Message
	}
	if in.Severity != nil {
		cur.Severity = *in.Severity
	}
	if in.Metric != nil {
		cur.Metric = *in.Metric
	}
	if in.Comparator != nil {
		cur.Comparator = *in.Comparator
	}
	if in.Threshold != nil {
		cur.Threshold = *in.Threshold
	}
	if in.Enabled != nil {
		cur.Enabled = *in.Enabled
	}
	if in.ClearCluster {
		cur.ClusterID = ""
	} else if in.ClusterID != nil {
		cur.ClusterID = *in.ClusterID
	}
	if in.AccountName != nil {
		cur.AccountName = *in.AccountName
	}

	var clusterArg any
	if !commonstrings.IsEmpty(cur.ClusterID) {
		clusterArg = cur.ClusterID
	}
	var accountArg any
	if !commonstrings.IsEmpty(cur.AccountName) {
		accountArg = cur.AccountName
	}
	_, err = s.pool.Exec(ctx, queryUpdateAlertRule,
		id, clusterArg, accountArg, cur.Name, cur.Message, cur.Severity,
		cur.Metric, cur.Comparator, cur.Threshold, cur.Enabled,
	)
	if err != nil {
		return domain.AlertRule{}, err
	}
	return s.GetAlertRule(ctx, id)
}

func (s *Store) DeleteAlertRule(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, queryDeleteAlertRule, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAlertRuleNotFound
	}
	return nil
}

func (s *Store) ListAlerts(ctx context.Context, f domain.AlertFilter) ([]domain.Alert, int, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 500 {
		f.Limit = 500
	}

	args := []any{}
	where := "WHERE 1=1"
	if !commonstrings.IsEmpty(f.Status) {
		args = append(args, f.Status)
		where += fmt.Sprintf(" AND a.status = $%d", len(args))
	}
	if !commonstrings.IsEmpty(f.Severity) {
		args = append(args, f.Severity)
		where += fmt.Sprintf(" AND a.severity = $%d", len(args))
	}
	if !commonstrings.IsEmpty(f.ClusterID) {
		args = append(args, f.ClusterID)
		where += fmt.Sprintf(" AND a.cluster_id = $%d", len(args))
	} else if len(f.ClusterIDs) > 0 {
		placeholders := make([]string, len(f.ClusterIDs))
		for i, id := range f.ClusterIDs {
			args = append(args, id)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		where += " AND a.cluster_id IN (" + strings.Join(placeholders, ",") + ")"
	}

	var total int
	if err := s.pool.QueryRow(ctx, queryCountAlerts+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, f.Limit, f.Offset)
	query := fmt.Sprintf(queryListAlerts, where, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	alerts, err := scanAlerts(rows)
	if err != nil {
		return nil, 0, err
	}
	return alerts, total, nil
}

func (s *Store) GetAlert(ctx context.Context, id string) (domain.Alert, error) {
	row := s.pool.QueryRow(ctx, queryGetAlert, id)
	a, err := scanAlert(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Alert{}, ErrAlertNotFound
	}
	return a, err
}

func (s *Store) ListOpenUnacknowledged(ctx context.Context, clusterIDs []string, limit int) ([]domain.Alert, int, error) {
	if limit <= 0 {
		limit = 10
	}
	args := []any{}
	where := "WHERE a.status = 'open' AND a.acknowledged_at IS NULL"
	if len(clusterIDs) > 0 {
		placeholders := make([]string, len(clusterIDs))
		for i, id := range clusterIDs {
			args = append(args, id)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		where += " AND a.cluster_id IN (" + strings.Join(placeholders, ",") + ")"
	}

	var total int
	if err := s.pool.QueryRow(ctx, queryCountAlerts+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit)
	query := fmt.Sprintf(queryListOpenUnacknowledgedAlerts, where, len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	alerts, err := scanAlerts(rows)
	return alerts, total, err
}

func (s *Store) AcknowledgeAlert(ctx context.Context, id, actor string) (domain.Alert, error) {
	tag, err := s.pool.Exec(ctx, queryAcknowledgeAlert, id, actor)
	if err != nil {
		return domain.Alert{}, err
	}
	if tag.RowsAffected() == 0 {
		// Still return current alert if it exists (already ack'd or closed)
		a, getErr := s.GetAlert(ctx, id)
		if getErr != nil {
			return domain.Alert{}, getErr
		}
		return a, nil
	}
	return s.GetAlert(ctx, id)
}

// UpsertOpenAlert opens or refreshes an open alert for the rule+cluster pair.
// newlyOpened is true when a new open row was inserted (not a refresh of an existing open alert).
func (s *Store) UpsertOpenAlert(ctx context.Context, rule domain.AlertRule, clusterID string, value float64, at time.Time) (alertID string, newlyOpened bool, err error) {
	message := rule.Message
	if commonstrings.IsEmpty(message) {
		message = rule.Name
	}
	err = s.pool.QueryRow(ctx, queryUpsertOpenAlert,
		newID(), rule.ID, clusterID, rule.AccountName, rule.Severity, rule.Metric, message,
		value, rule.Threshold, at,
	).Scan(&alertID, &newlyOpened)
	return alertID, newlyOpened, err
}

// ClaimAlertEmailNotify marks an alert as email-notified if not already claimed.
// Returns true when this caller won the claim and should send mail.
func (s *Store) ClaimAlertEmailNotify(ctx context.Context, alertID string) (bool, error) {
	tag, err := s.pool.Exec(ctx, queryClaimAlertEmailNotify, alertID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ReleaseAlertEmailNotify clears a prior claim so a failed send can be retried.
func (s *Store) ReleaseAlertEmailNotify(ctx context.Context, alertID string) error {
	_, err := s.pool.Exec(ctx, queryReleaseAlertEmailNotify, alertID)
	return err
}

func (s *Store) CloseOpenAlert(ctx context.Context, ruleID, clusterID string, at time.Time) error {
	_, err := s.pool.Exec(ctx, queryCloseOpenAlert, ruleID, clusterID, at)
	return err
}

// ListOpenAlertRuleIDs returns rule IDs with an open alert for the cluster.
func (s *Store) ListOpenAlertRuleIDs(ctx context.Context, clusterID string) (map[string]struct{}, error) {
	rows, err := s.pool.Query(ctx, queryListOpenAlertRuleIDs, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var ruleID string
		if err := rows.Scan(&ruleID); err != nil {
			return nil, err
		}
		out[ruleID] = struct{}{}
	}
	return out, rows.Err()
}

// DeleteClosedAlertsOlderThan removes closed alert rows older than cutoff.
func (s *Store) DeleteClosedAlertsOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, queryDeleteClosedAlertsOlderThan, cutoff.UTC())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

type alertScanner interface {
	Scan(dest ...any) error
}

func scanAlert(row alertScanner) (domain.Alert, error) {
	var a domain.Alert
	err := row.Scan(
		&a.ID, &a.RuleID, &a.ClusterID, &a.AccountName,
		&a.Status, &a.Severity, &a.Metric, &a.Message, &a.FiringValue, &a.Threshold,
		&a.FirstSeenAt, &a.LastSeenAt, &a.ClosedAt, &a.AcknowledgedAt, &a.AcknowledgedBy,
		&a.RuleName,
	)
	return a, err
}

func scanAlerts(rows pgx.Rows) ([]domain.Alert, error) {
	var alerts []domain.Alert
	for rows.Next() {
		a, err := scanAlert(rows)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	if alerts == nil {
		alerts = []domain.Alert{}
	}
	return alerts, rows.Err()
}
