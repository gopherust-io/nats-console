package auth

import (
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
)

func StoreUserToDomain(user store.User) domain.User {
	var rules *domain.AccessRules
	if user.AccessRules != nil {
		rules = &domain.AccessRules{
			ClusterIDs:      append([]string(nil), user.AccessRules.ClusterIDs...),
			ManageUsers:     user.AccessRules.ManageUsers,
			ViewAudit:       user.AccessRules.ViewAudit,
			DeleteClusters:  user.AccessRules.DeleteClusters,
			AssignableRoles: append([]string(nil), user.AccessRules.AssignableRoles...),
		}
	}
	return domain.User{
		ID:          user.ID,
		Username:    user.Username,
		Email:       user.Email,
		OIDCSub:     user.OIDCSub,
		Roles:       user.Roles,
		IsRoot:      user.IsRoot,
		AccessRules: rules,
		CreatedAt:   user.CreatedAt,
	}
}

func permissionsForUser(user store.User) domain.Permissions {
	return domain.PermissionsFor(StoreUserToDomain(user))
}

func CanWrite(user store.User) bool {
	if user.IsRoot {
		return true
	}
	role := store.HighestRole(user.Roles)
	return role == store.RoleAdmin || role == store.RoleOperator
}

func CanDeleteCluster(user store.User) bool {
	if user.IsRoot {
		return true
	}
	perms := permissionsForUser(user)
	if perms.DeleteClusters {
		return true
	}
	return store.HighestRole(user.Roles) == store.RoleAdmin && user.AccessRules == nil
}

func CanManageUsers(user store.User) bool {
	return permissionsForUser(user).ManageUsers
}

func CanViewAudit(user store.User) bool {
	return permissionsForUser(user).ViewAudit
}

func CanViewProfiling(user store.User) bool {
	if user.IsRoot {
		return true
	}
	return store.HighestRole(user.Roles) == store.RoleAdmin
}

func CanAccessCluster(user store.User, clusterID string) bool {
	return permissionsForUser(user).AllowsCluster(clusterID)
}

func CanCreateCluster(user store.User) bool {
	if user.IsRoot {
		return true
	}
	return store.HighestRole(user.Roles) == store.RoleAdmin && user.AccessRules == nil
}

func CanViewMetrics(user store.User) bool {
	if user.IsRoot {
		return true
	}
	return store.HighestRole(user.Roles) == store.RoleAdmin
}
