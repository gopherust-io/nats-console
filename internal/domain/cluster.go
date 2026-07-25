package domain

import "time"

// goalign:ignore
type Cluster struct {
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	NATSURL       string    `json:"natsUrl"`
	MonitoringURL string    `json:"monitoringUrl"`
	HasCreds      bool      `json:"hasCreds"`
	HasToken      bool      `json:"hasToken"`
	IsDefault     bool      `json:"isDefault"`
}

// goalign:ignore
type ClusterCreate struct {
	Name          string
	NATSURL       string
	MonitoringURL string
	CredsFilePath string
	Token         string
	IsDefault     bool
}

type ClusterUpdate struct {
	Name          *string
	NATSURL       *string
	MonitoringURL *string
	CredsFilePath *string
	Token         *string
	IsDefault     *bool
}

// goalign:ignore
type ClusterTestResult struct {
	Message    string `json:"message"`
	ServerName string `json:"serverName,omitempty"`
	OK         bool   `json:"ok"`
	JetStream  bool   `json:"jetstream,omitempty"`
}
