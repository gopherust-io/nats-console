package domain

import "slices"

// Permissions summarizes effective capabilities for authorization checks.
type Permissions struct {
	ClusterIDs      []string
	AssignableRoles []string
	IsRoot          bool
	ManageUsers     bool
	ViewAudit       bool
	DeleteClusters  bool
}

func FullPermissions() Permissions {
	return Permissions{
		IsRoot:          true,
		ManageUsers:     true,
		ViewAudit:       true,
		DeleteClusters:  true,
		AssignableRoles: []string{RoleAdmin, RoleOperator, RoleViewer},
	}
}

func PermissionsFor(user User) Permissions {
	if user.IsRoot {
		return FullPermissions()
	}
	if HasRole(user.Roles, RoleAdmin) {
		if user.AccessRules == nil {
			return Permissions{
				ManageUsers:     true,
				ViewAudit:       true,
				DeleteClusters:  true,
				AssignableRoles: []string{RoleAdmin, RoleOperator, RoleViewer},
			}
		}
		rules := user.AccessRules
		return Permissions{
			ManageUsers:     rules.ManageUsers,
			ViewAudit:       rules.ViewAudit,
			DeleteClusters:  rules.DeleteClusters,
			ClusterIDs:      rules.ClusterIDs,
			AssignableRoles: append([]string(nil), rules.AssignableRoles...),
		}
	}
	return Permissions{}
}

func (p Permissions) AllowsCluster(clusterID string) bool {
	if p.IsRoot || len(p.ClusterIDs) == 0 {
		return true
	}
	return slices.Contains(p.ClusterIDs, clusterID)
}

func (p Permissions) AllowsRole(role string) bool {
	if p.IsRoot {
		return true
	}
	if len(p.AssignableRoles) == 0 {
		return false
	}
	return HasRole(p.AssignableRoles, role)
}

func (p Permissions) AllowsRoles(roles []string) bool {
	for _, role := range roles {
		if !p.AllowsRole(role) {
			return false
		}
	}
	return true
}

func (p Permissions) CanAssignAccessRules(rules *AccessRules) bool {
	if p.IsRoot {
		return true
	}
	if rules == nil {
		return true
	}
	if rules.ManageUsers && !p.ManageUsers {
		return false
	}
	if rules.ViewAudit && !p.ViewAudit {
		return false
	}
	if rules.DeleteClusters && !p.DeleteClusters {
		return false
	}
	if len(rules.ClusterIDs) > 0 {
		for _, id := range rules.ClusterIDs {
			if !p.AllowsCluster(id) {
				return false
			}
		}
	}
	for _, role := range rules.AssignableRoles {
		if !p.AllowsRole(role) {
			return false
		}
	}
	return true
}

func (p Permissions) SupersetOf(other Permissions) bool {
	if p.IsRoot {
		return true
	}
	if other.ManageUsers && !p.ManageUsers {
		return false
	}
	if other.ViewAudit && !p.ViewAudit {
		return false
	}
	if other.DeleteClusters && !p.DeleteClusters {
		return false
	}
	for _, id := range other.ClusterIDs {
		if !p.AllowsCluster(id) {
			return false
		}
	}
	for _, role := range other.AssignableRoles {
		if !p.AllowsRole(role) {
			return false
		}
	}
	return true
}
