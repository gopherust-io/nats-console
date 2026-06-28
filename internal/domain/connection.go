package domain

import "time"

// NATSConnectionStatus describes the console's cached NATS client for a cluster.
type NATSConnectionStatus struct {
	LastCheckedAt   time.Time  `json:"lastCheckedAt"`
	LastConnectedAt *time.Time `json:"lastConnectedAt,omitempty"`
	ClusterID       string     `json:"clusterId"`
	ServerName      string     `json:"serverName,omitempty"`
	LastError       string     `json:"lastError,omitempty"`
	Reconnects      uint64     `json:"reconnects"`
	Connected       bool       `json:"connected"`
	Cached          bool       `json:"cached"`
	JetStreamOK     bool       `json:"jetstreamOk"`
}
