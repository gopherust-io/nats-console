package domain

import "slices"

// Permissions summarizes effective capabilities for authorization checks.
type Permissions struct {
	ClusterIDs      []string
	AssignableRoles []string
	IsRoot          bool
	AllClusters     bool
	ManageUsers     bool
	ViewAudit       bool
	DeleteClusters  bool
}

func FullPermissions() Permissions {
	return Permissions{
		IsRoot:          true,
		AllClusters:     true,
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
	if HasRole(user.Roles, RoleAdmin) && user.AccessRules == nil {
		return Permissions{
			AllClusters:     true,
			ManageUsers:     true,
			ViewAudit:       true,
			DeleteClusters:  true,
			AssignableRoles: []string{RoleAdmin, RoleOperator, RoleViewer},
		}
	}
	perms := Permissions{}
	if user.AccessRules != nil {
		rules := user.AccessRules
		perms.ClusterIDs = append([]string(nil), rules.ClusterIDs...)
		perms.ManageUsers = rules.ManageUsers
		perms.ViewAudit = rules.ViewAudit
		perms.DeleteClusters = rules.DeleteClusters
		perms.AssignableRoles = append([]string(nil), rules.AssignableRoles...)
	}
	return perms
}

func (p Permissions) AllowsCluster(clusterID string) bool {
	if p.IsRoot || p.AllClusters {
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
