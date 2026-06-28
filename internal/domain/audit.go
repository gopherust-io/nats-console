package domain

import (
	"encoding/json"
	"time"
)

type AuditEntry struct {
	Timestamp    time.Time       `json:"timestamp"`
	ID           string          `json:"id"`
	Actor        string          `json:"actor"`
	Action       string          `json:"action"`
	ClusterID    string          `json:"clusterId"`
	ResourceType string          `json:"resourceType"`
	ResourceName string          `json:"resourceName"`
	RequestID    string          `json:"requestId"`
	IP           string          `json:"ip"`
	Details      json.RawMessage `json:"details"`
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
