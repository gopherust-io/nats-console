package domain

import "slices"

// AccessRules scopes non-root users to specific clusters and delegated admin capabilities.
// Root users and legacy admin users without access rules ignore these fields.
type AccessRules struct {
	ClusterIDs      []string `json:"clusterIds,omitempty"`
	AssignableRoles []string `json:"assignableRoles,omitempty"`
	ManageUsers     bool     `json:"manageUsers"`
	ViewAudit       bool     `json:"viewAudit"`
	DeleteClusters  bool     `json:"deleteClusters"`
}

func (r *AccessRules) AllowsCluster(clusterID string) bool {
	if r == nil {
		return false
	}
	return slices.Contains(r.ClusterIDs, clusterID)
}

func (r *AccessRules) AllowsRole(role string) bool {
	if r == nil || len(r.AssignableRoles) == 0 {
		return false
	}
	return HasRole(r.AssignableRoles, role)
}
