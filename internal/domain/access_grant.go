package domain

import "time"

const (
	ResourceSystem   = "system"
	ResourceAccount  = "account"
	ResourceNATSUser = "nats_user"

	GrantAdmin                = "admin"
	GrantObserver             = "observer"
	GrantCredentialDownloader = "credential_downloader" //nolint:gosec // G101: role name, not a credential
)

// goalign:ignore
type AccessGrant struct {
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	ResourceType string    `json:"resourceType"`
	ResourceKey  string    `json:"resourceKey"`
	Role         string    `json:"role"`
	Username     string    `json:"username,omitempty"`
	Email        string    `json:"email,omitempty"`
}

// AccountResourceKey builds "{clusterId}:{accountName}".
func AccountResourceKey(clusterID, accountName string) string {
	return clusterID + ":" + accountName
}

// NATSUserResourceKey builds "{clusterId}:{accountName}:{natsUserId}".
func NATSUserResourceKey(clusterID, accountName, natsUserID string) string {
	return clusterID + ":" + accountName + ":" + natsUserID
}
