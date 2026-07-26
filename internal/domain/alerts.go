package domain

import (
	"slices"
	"time"
)

const (
	AlertSeverityInfo     = "info"
	AlertSeverityWarning  = "warning"
	AlertSeverityCritical = "critical"

	AlertStatusOpen   = "open"
	AlertStatusClosed = "closed"

	AlertComparatorGT  = "gt"
	AlertComparatorGTE = "gte"
	AlertComparatorLT  = "lt"
	AlertComparatorLTE = "lte"
)

var AlertSeverities = []string{AlertSeverityInfo, AlertSeverityWarning, AlertSeverityCritical}
var AlertComparators = []string{AlertComparatorGT, AlertComparatorGTE, AlertComparatorLT, AlertComparatorLTE}

// goalign:ignore
type AlertRule struct {
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Metric      string    `json:"metric"`
	Name        string    `json:"name"`
	Message     string    `json:"message"`
	Severity    string    `json:"severity"`
	ID          string    `json:"id"`
	Comparator  string    `json:"comparator"`
	CreatedBy   string    `json:"createdBy,omitempty"`
	AccountName string    `json:"accountName,omitempty"`
	ClusterID   string    `json:"clusterId,omitempty"`
	Threshold   float64   `json:"threshold"`
	Enabled     bool      `json:"enabled"`
}

type AlertRuleCreate struct {
	Enabled     *bool   `json:"enabled"`
	ClusterID   string  `json:"clusterId"`
	AccountName string  `json:"accountName"`
	Name        string  `json:"name"`
	Message     string  `json:"message"`
	Severity    string  `json:"severity"`
	Metric      string  `json:"metric"`
	Comparator  string  `json:"comparator"`
	Threshold   float64 `json:"threshold"`
}

// goalign:ignore
type AlertRuleUpdate struct {
	ClusterID    *string  `json:"clusterId"`
	AccountName  *string  `json:"accountName"`
	Name         *string  `json:"name"`
	Message      *string  `json:"message"`
	Severity     *string  `json:"severity"`
	Metric       *string  `json:"metric"`
	Comparator   *string  `json:"comparator"`
	Threshold    *float64 `json:"threshold"`
	Enabled      *bool    `json:"enabled"`
	ClearCluster bool     `json:"clearCluster"`
}

type Alert struct {
	FirstSeenAt    time.Time  `json:"firstSeenAt"`
	LastSeenAt     time.Time  `json:"lastSeenAt"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
	ClosedAt       *time.Time `json:"closedAt,omitempty"`
	Status         string     `json:"status"`
	Severity       string     `json:"severity"`
	Metric         string     `json:"metric"`
	Message        string     `json:"message"`
	ID             string     `json:"id"`
	AccountName    string     `json:"accountName,omitempty"`
	ClusterID      string     `json:"clusterId"`
	RuleID         string     `json:"ruleId"`
	AcknowledgedBy string     `json:"acknowledgedBy,omitempty"`
	RuleName       string     `json:"ruleName,omitempty"`
	FiringValue    float64    `json:"firingValue"`
	Threshold      float64    `json:"threshold"`
}

type AlertFilter struct {
	Status     string
	ClusterID  string
	Severity   string
	ClusterIDs []string
	Limit      int
	Offset     int
}

type AlertOpenSummary struct {
	Alerts []Alert `json:"alerts"`
	Count  int     `json:"count"`
}

func ValidAlertSeverity(v string) bool {
	return slices.Contains(AlertSeverities, v)
}

func ValidAlertComparator(v string) bool {
	return slices.Contains(AlertComparators, v)
}

func ThresholdMet(comparator string, value, threshold float64) bool {
	switch comparator {
	case AlertComparatorGT:
		return value > threshold
	case AlertComparatorGTE:
		return value >= threshold
	case AlertComparatorLT:
		return value < threshold
	case AlertComparatorLTE:
		return value <= threshold
	default:
		return false
	}
}
