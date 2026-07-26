package auth

import (
	"slices"
	"strings"

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
	grants := make([]domain.AccessGrant, 0, len(user.Grants))
	for _, g := range user.Grants {
		grants = append(grants, domain.AccessGrant{
			ID:           g.ID,
			UserID:       g.UserID,
			ResourceType: g.ResourceType,
			ResourceKey:  g.ResourceKey,
			Role:         g.Role,
			CreatedAt:    g.CreatedAt,
			UpdatedAt:    g.UpdatedAt,
			Username:     g.Username,
			Email:        g.Email,
		})
	}
	return domain.User{
		ID:          user.ID,
		Username:    user.Username,
		Email:       user.Email,
		OIDCSub:     user.OIDCSub,
		Roles:       user.Roles,
		IsRoot:      user.IsRoot,
		AccessRules: rules,
		Grants:      grants,
		CreatedAt:   user.CreatedAt,
	}
}

func isLegacyFullAdmin(user store.User) bool {
	return store.HighestRole(user.Roles) == store.RoleAdmin && user.AccessRules == nil
}

func allowsClusterRules(user store.User, clusterID string) bool {
	if user.IsRoot || isLegacyFullAdmin(user) {
		return true
	}
	if user.AccessRules == nil {
		return false
	}
	return slices.Contains(user.AccessRules.ClusterIDs, clusterID)
}

func CanWrite(user store.User) bool {
	if user.IsRoot {
		return true
	}
	role := store.HighestRole(user.Roles)
	return role == store.RoleAdmin || role == store.RoleOperator
}

// CanCreateCluster allows registering new clusters for root and legacy unscoped admins only.
func CanCreateCluster(user store.User) bool {
	if user.IsRoot {
		return true
	}
	return store.HighestRole(user.Roles) == store.RoleAdmin && user.AccessRules == nil
}

// CanDeleteCluster allows deleting clusters for root, unscoped admins, or scoped admins with DeleteClusters.
func CanDeleteCluster(user store.User) bool {
	if user.IsRoot || isLegacyFullAdmin(user) {
		return true
	}
	if user.AccessRules != nil {
		return user.AccessRules.DeleteClusters
	}
	return false
}

func CanManageUsers(user store.User) bool {
	if user.IsRoot || isLegacyFullAdmin(user) {
		return true
	}
	if user.AccessRules != nil {
		return user.AccessRules.ManageUsers
	}
	return false
}

func CanViewAudit(user store.User) bool {
	if user.IsRoot || isLegacyFullAdmin(user) {
		return true
	}
	if user.AccessRules != nil {
		return user.AccessRules.ViewAudit
	}
	return false
}

func CanViewProfiling(user store.User) bool {
	if user.IsRoot {
		return true
	}
	return store.HighestRole(user.Roles) == store.RoleAdmin
}

// CanManageAlertRules allows creating and editing metric alert rules.
func CanManageAlertRules(user store.User) bool {
	if user.IsRoot {
		return true
	}
	return store.HighestRole(user.Roles) == store.RoleAdmin || CanManageUsers(user)
}

func CanAccessCluster(user store.User, clusterID string) bool {
	if allowsClusterRules(user, clusterID) {
		return true
	}
	for _, g := range user.Grants {
		switch g.ResourceType {
		case store.ResourceSystem:
			if g.ResourceKey == clusterID {
				return true
			}
		case store.ResourceAccount, store.ResourceNATSUser:
			if g.ResourceKey == clusterID || strings.HasPrefix(g.ResourceKey, clusterID+":") {
				return true
			}
		}
	}
	return false
}

// CanWriteCluster allows mutations on a cluster when the user has legacy write
// capability and access via access rules / system grant. Account-scoped grants
// do not elevate to cluster-wide write.
func CanWriteCluster(user store.User, clusterID string) bool {
	if user.IsRoot {
		return true
	}
	if hasAdminGrant(user, store.ResourceSystem, clusterID) {
		return true
	}
	return CanWrite(user) && allowsClusterRules(user, clusterID)
}

// CanManageJetStream allows create/update/delete of streams, consumers, KV, and
// object stores. Operators are excluded; only root, global admin with cluster
// access rules, or a system admin grant may mutate JetStream at cluster scope.
// Account-scoped admin grants do not imply whole-cluster JetStream manage.
func CanManageJetStream(user store.User, clusterID string) bool {
	if user.IsRoot {
		return true
	}
	if hasAdminGrant(user, store.ResourceSystem, clusterID) {
		return true
	}
	return store.HighestRole(user.Roles) == store.RoleAdmin && allowsClusterRules(user, clusterID)
}

// CanManageJetStreamAccount allows JetStream mutations scoped to one account.
func CanManageJetStreamAccount(user store.User, clusterID, accountName string) bool {
	if CanManageJetStream(user, clusterID) {
		return true
	}
	return hasAdminGrant(user, store.ResourceAccount, domain.AccountResourceKey(clusterID, accountName))
}

func CanDownloadCreds(user store.User, clusterID, accountName string) bool {
	if user.IsRoot {
		return true
	}
	if hasAdminGrant(user, store.ResourceSystem, clusterID) {
		return true
	}
	accountKey := domain.AccountResourceKey(clusterID, accountName)
	for _, g := range user.Grants {
		if g.ResourceKey != accountKey && !strings.HasPrefix(g.ResourceKey, accountKey+":") {
			continue
		}
		switch g.Role {
		case store.GrantAdmin, store.GrantCredentialDownloader:
			return true
		}
	}
	// Global admins with explicit cluster access may download; operators cannot
	// without a dedicated grant.
	return store.HighestRole(user.Roles) == store.RoleAdmin && allowsClusterRules(user, clusterID)
}

func CanManageSystemAccess(user store.User, clusterID string) bool {
	if user.IsRoot || CanManageUsers(user) {
		return true
	}
	return hasAdminGrant(user, store.ResourceSystem, clusterID)
}

func CanManageAccountAccess(user store.User, clusterID, accountName string) bool {
	if CanManageSystemAccess(user, clusterID) {
		return true
	}
	return hasAdminGrant(user, store.ResourceAccount, domain.AccountResourceKey(clusterID, accountName))
}

func hasAdminGrant(user store.User, resourceType, resourceKey string) bool {
	for _, g := range user.Grants {
		if g.ResourceType == resourceType && g.ResourceKey == resourceKey && g.Role == store.GrantAdmin {
			return true
		}
	}
	return false
}
