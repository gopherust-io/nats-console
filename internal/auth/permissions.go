package auth

import (
	"slices"
	"strings"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
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
		grants = append(grants, domain.AccessGrant(g))
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

// CanAccessCluster reports whether the user has cluster-wide (system-level)
// access to clusterID. Only root, legacy unscoped admins, access-rule scoped
// users, and holders of a system resource grant qualify. Account- or
// NATS-user-scoped grants intentionally do NOT satisfy this check: such
// grants authorize access to one account within the cluster, not the whole
// cluster (its JetStream data, server monitoring, topology, etc.). Callers
// that only need "does this user have some presence in the cluster" (for
// example, listing clusters to display) should use
// CanAccessClusterOrAccount instead.
func CanAccessCluster(user store.User, clusterID string) bool {
	if allowsClusterRules(user, clusterID) {
		return true
	}
	return hasAdminOrObserverGrant(user, store.ResourceSystem, clusterID)
}

// CanAccessClusterOrAccount reports whether the user has any grant touching
// clusterID: full cluster-wide access (see CanAccessCluster) OR an
// account/NATS-user scoped grant within that cluster. Use this for coarse
// "does the user have any business seeing this cluster" checks (e.g. listing
// clusters, or gating account-management routes) - it must never be used to
// authorize cluster-wide capabilities like JetStream data, monitoring, or
// topology. Handlers that take an account name must further narrow with
// CanAccessAccount.
func CanAccessClusterOrAccount(user store.User, clusterID string) bool {
	if CanAccessCluster(user, clusterID) {
		return true
	}
	for _, g := range user.Grants {
		switch g.ResourceType {
		case store.ResourceAccount, store.ResourceNATSUser:
			if g.ResourceKey == clusterID || strings.HasPrefix(g.ResourceKey, clusterID+":") {
				return true
			}
		}
	}
	return false
}

// CanAccessAccount reports whether the user may read/mutate resources scoped
// to a specific NATS account within clusterID. Cluster-wide access implies
// all accounts. Otherwise the user needs an account grant for that account
// or a nats_user grant under that account key prefix.
func CanAccessAccount(user store.User, clusterID, accountName string) bool {
	if CanAccessCluster(user, clusterID) {
		return true
	}
	accountKey := domain.AccountResourceKey(clusterID, accountName)
	for _, g := range user.Grants {
		switch g.ResourceType {
		case store.ResourceAccount:
			if g.ResourceKey == accountKey {
				return true
			}
		case store.ResourceNATSUser:
			if natsUserGrantCoversAccount(g.ResourceKey, clusterID, accountName) {
				return true
			}
		}
	}
	return false
}

// hasAdminOrObserverGrant reports whether the user holds any grant (of any
// role) for the given resource type/key. Unlike hasAdminGrant, this is not
// limited to admin-role grants.
func hasAdminOrObserverGrant(user store.User, resourceType, resourceKey string) bool {
	for _, g := range user.Grants {
		if g.ResourceType == resourceType && g.ResourceKey == resourceKey {
			return true
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

// CanDownloadCreds reports whether the user may download/rotate/mint credentials
// for natsUserID in accountName. Account-level Admin/CredentialDownloader grants
// authorize every user in that account. A nats_user grant only authorizes that
// exact NATS user (prefix matching is intentionally rejected).
func CanDownloadCreds(user store.User, clusterID, accountName, natsUserID string) bool {
	if user.IsRoot {
		return true
	}
	if hasAdminGrant(user, store.ResourceSystem, clusterID) {
		return true
	}
	accountKey := domain.AccountResourceKey(clusterID, accountName)
	natsUserKey := ""
	if !commonstrings.IsEmpty(natsUserID) {
		natsUserKey = domain.NATSUserResourceKey(clusterID, accountName, natsUserID)
	}
	for _, g := range user.Grants {
		switch g.Role {
		case store.GrantAdmin, store.GrantCredentialDownloader:
		default:
			continue
		}
		switch g.ResourceType {
		case store.ResourceAccount:
			if g.ResourceKey == accountKey {
				return true
			}
		case store.ResourceNATSUser:
			if !commonstrings.IsEmpty(natsUserKey) && g.ResourceKey == natsUserKey {
				return true
			}
		}
	}
	// Global admins with explicit cluster access may download; operators cannot
	// without a dedicated grant.
	return store.HighestRole(user.Roles) == store.RoleAdmin && allowsClusterRules(user, clusterID)
}

func CanManageSystemAccess(user store.User, clusterID string) bool {
	if user.IsRoot {
		return true
	}
	if hasAdminGrant(user, store.ResourceSystem, clusterID) {
		return true
	}
	// ManageUsers may manage access only on clusters they can already reach.
	return CanManageUsers(user) && CanAccessCluster(user, clusterID)
}

func CanManageAccountAccess(user store.User, clusterID, accountName string) bool {
	if CanManageSystemAccess(user, clusterID) {
		return true
	}
	return hasAdminGrant(user, store.ResourceAccount, domain.AccountResourceKey(clusterID, accountName))
}

// CanMintAdminGrant reports whether the actor may assign the admin grant role
// on the given resource. ManageUsers alone is not enough — that would bypass
// AssignableRoles and escalate to CanDownloadCreds / CanManageJetStream.
func CanMintAdminGrant(user store.User, resourceType, resourceKey string) bool {
	if user.IsRoot || isLegacyFullAdmin(user) {
		return true
	}
	return hasAdminGrant(user, resourceType, resourceKey)
}

func natsUserGrantCoversAccount(resourceKey, clusterID, accountName string) bool {
	prefix := clusterID + ":" + accountName + ":"
	if !strings.HasPrefix(resourceKey, prefix) {
		return false
	}
	id := strings.TrimPrefix(resourceKey, prefix)
	return id != "" && !strings.Contains(id, ":")
}

func hasAdminGrant(user store.User, resourceType, resourceKey string) bool {
	for _, g := range user.Grants {
		if g.ResourceType == resourceType && g.ResourceKey == resourceKey && g.Role == store.GrantAdmin {
			return true
		}
	}
	return false
}
