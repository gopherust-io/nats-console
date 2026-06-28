package domain

import "time"

type JWTAccount struct {
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	ID        string     `json:"id"`
	ClusterID string     `json:"clusterId"`
	Name      string     `json:"name"`
	HasJWT    bool       `json:"hasJwt"`
}

type JWTAccountImport struct {
	JWT  string `json:"jwt"`
	Name string `json:"name,omitempty"`
}

type JWTResolverExport struct {
	Accounts []JWTExportEntry `json:"accounts"`
}

type JWTExportEntry struct {
	Name string `json:"name"`
	JWT  string `json:"jwt"`
}

type RotateEncryptionKeyRequest struct {
	CurrentKey string `json:"currentKey"`
	NewKey     string `json:"newKey"`
}

type RotateEncryptionKeyResult struct {
	Message         string `json:"message"`
	ClustersUpdated int    `json:"clustersUpdated"`
	JWTUpdated      int    `json:"jwtUpdated"`
	DryRun          bool   `json:"dryRun"`
}
