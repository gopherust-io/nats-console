package domain

import "slices"

// AccessRules scopes delegated admin users created by the root account.
// Root users ignore access rules and have full permissions.
// Legacy admin users without access rules retain full admin permissions.
type AccessRules struct {
	ClusterIDs      []string `json:"clusterIds,omitempty"`
	AssignableRoles []string `json:"assignableRoles,omitempty"`
	ManageUsers     bool     `json:"manageUsers"`
	ViewAudit       bool     `json:"viewAudit"`
	DeleteClusters  bool     `json:"deleteClusters"`
}

func (r *AccessRules) AllowsCluster(clusterID string) bool {
	if r == nil || len(r.ClusterIDs) == 0 {
		return true
	}
	return slices.Contains(r.ClusterIDs, clusterID)
}

func (r *AccessRules) AllowsRole(role string) bool {
	if r == nil || len(r.AssignableRoles) == 0 {
		return false
	}
	return HasRole(r.AssignableRoles, role)
}
