package domain

type RotateEncryptionKeyRequest struct {
	CurrentKey string `json:"currentKey"`
	NewKey     string `json:"newKey"`
}

// goalign:ignore
type RotateEncryptionKeyResult struct {
	Message         string `json:"message"`
	ClustersUpdated int    `json:"clustersUpdated"`
	DryRun          bool   `json:"dryRun"`
}
