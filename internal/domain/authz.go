package domain

import (
	"slices"
	"strings"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// Action identifies an authorization decision.
type Action string

const (
	ActionWrite                  Action = "write"
	ActionDeleteCluster          Action = "delete_cluster"
	ActionManageUsers            Action = "manage_users"
	ActionViewAudit              Action = "view_audit"
	ActionViewProfiling          Action = "view_profiling"
	ActionManageAlertRules       Action = "manage_alert_rules"
	ActionAccessCluster          Action = "access_cluster"
	ActionAccessClusterOrAccount Action = "access_cluster_or_account"
	ActionAccessAccount          Action = "access_account"
	ActionWriteCluster           Action = "write_cluster"
	ActionManageJetStream        Action = "manage_jetstream"
	ActionManageJetStreamAccount Action = "manage_jetstream_account"
	ActionDownloadCreds          Action = "download_creds"
	ActionManageSystemAccess     Action = "manage_system_access"
	ActionManageAccountAccess    Action = "manage_account_access"
	ActionMintAdminGrant         Action = "mint_admin_grant"
)

// Resource scopes an Action. ClusterID / Account / NATSUserID are used when
// Type/Key alone are insufficient (account and creds checks).
type Resource struct {
	Type       string
	Key        string
	ClusterID  string
	Account    string
	NATSUserID string
}

// Can evaluates whether user may perform action on resource.
func Can(user User, action Action, resource Resource) bool {
	switch action {
	case ActionWrite:
		return CanWrite(user)
	case ActionDeleteCluster:
		return CanDeleteCluster(user)
	case ActionManageUsers:
		return CanManageUsers(user)
	case ActionViewAudit:
		return CanViewAudit(user)
	case ActionViewProfiling:
		return CanViewProfiling(user)
	case ActionManageAlertRules:
		return CanManageAlertRules(user)
	case ActionAccessCluster:
		return CanAccessCluster(user, resource.ClusterID)
	case ActionAccessClusterOrAccount:
		return CanAccessClusterOrAccount(user, resource.ClusterID)
	case ActionAccessAccount:
		return CanAccessAccount(user, resource.ClusterID, resource.Account)
	case ActionWriteCluster:
		return CanWriteCluster(user, resource.ClusterID)
	case ActionManageJetStream:
		return CanManageJetStream(user, resource.ClusterID)
	case ActionManageJetStreamAccount:
		return CanManageJetStreamAccount(user, resource.ClusterID, resource.Account)
	case ActionDownloadCreds:
		return CanDownloadCreds(user, resource.ClusterID, resource.Account, resource.NATSUserID)
	case ActionManageSystemAccess:
		return CanManageSystemAccess(user, resource.ClusterID)
	case ActionManageAccountAccess:
		return CanManageAccountAccess(user, resource.ClusterID, resource.Account)
	case ActionMintAdminGrant:
		return CanMintAdminGrant(user, resource.Type, resource.Key)
	default:
		return false
	}
}

func isLegacyFullAdmin(user User) bool {
	return HighestRole(user.Roles) == RoleAdmin && user.AccessRules == nil
}

func allowsClusterRules(user User, clusterID string) bool {
	if user.IsRoot || isLegacyFullAdmin(user) {
		return true
	}
	if user.AccessRules == nil {
		return false
	}
	return slices.Contains(user.AccessRules.ClusterIDs, clusterID)
}

func CanWrite(user User) bool {
	if user.IsRoot {
		return true
	}
	role := HighestRole(user.Roles)
	return role == RoleAdmin || role == RoleOperator
}

// CanDeleteCluster allows deleting clusters for root, unscoped admins, or scoped admins with DeleteClusters.
func CanDeleteCluster(user User) bool {
	if user.IsRoot || isLegacyFullAdmin(user) {
		return true
	}
	if user.AccessRules != nil {
		return user.AccessRules.DeleteClusters
	}
	return false
}

func CanManageUsers(user User) bool {
	if user.IsRoot || isLegacyFullAdmin(user) {
		return true
	}
	if user.AccessRules != nil {
		return user.AccessRules.ManageUsers
	}
	return false
}

func CanViewAudit(user User) bool {
	if user.IsRoot || isLegacyFullAdmin(user) {
		return true
	}
	if user.AccessRules != nil {
		return user.AccessRules.ViewAudit
	}
	return false
}

func CanViewProfiling(user User) bool {
	if user.IsRoot {
		return true
	}
	return HighestRole(user.Roles) == RoleAdmin
}

// CanManageAlertRules allows creating and editing metric alert rules.
func CanManageAlertRules(user User) bool {
	if user.IsRoot {
		return true
	}
	return HighestRole(user.Roles) == RoleAdmin || CanManageUsers(user)
}

// CanAccessCluster reports whether the user has cluster-wide (system-level)
// access to clusterID. Only root, legacy unscoped admins, access-rule scoped
// users, and holders of a system resource grant qualify. Account- or
// NATS-user-scoped grants intentionally do NOT satisfy this check.
func CanAccessCluster(user User, clusterID string) bool {
	if allowsClusterRules(user, clusterID) {
		return true
	}
	return hasAdminOrObserverGrant(user, ResourceSystem, clusterID)
}

// CanAccessClusterOrAccount reports whether the user has any grant touching
// clusterID: full cluster-wide access OR an account/NATS-user scoped grant.
func CanAccessClusterOrAccount(user User, clusterID string) bool {
	if CanAccessCluster(user, clusterID) {
		return true
	}
	for _, g := range user.Grants {
		switch g.ResourceType {
		case ResourceAccount, ResourceNATSUser:
			if g.ResourceKey == clusterID || strings.HasPrefix(g.ResourceKey, clusterID+":") {
				return true
			}
		}
	}
	return false
}

// CanAccessAccount reports whether the user may read/mutate resources scoped
// to a specific NATS account within clusterID.
func CanAccessAccount(user User, clusterID, accountName string) bool {
	if CanAccessCluster(user, clusterID) {
		return true
	}
	accountKey := AccountResourceKey(clusterID, accountName)
	for _, g := range user.Grants {
		switch g.ResourceType {
		case ResourceAccount:
			if g.ResourceKey == accountKey {
				return true
			}
		case ResourceNATSUser:
			if natsUserGrantCoversAccount(g.ResourceKey, clusterID, accountName) {
				return true
			}
		}
	}
	return false
}

func hasAdminOrObserverGrant(user User, resourceType, resourceKey string) bool {
	for _, g := range user.Grants {
		if g.ResourceType == resourceType && g.ResourceKey == resourceKey {
			return true
		}
	}
	return false
}

// CanWriteCluster allows mutations on a cluster when the user has legacy write
// capability and access via access rules / system grant.
func CanWriteCluster(user User, clusterID string) bool {
	if user.IsRoot {
		return true
	}
	if hasAdminGrant(user, ResourceSystem, clusterID) {
		return true
	}
	return CanWrite(user) && allowsClusterRules(user, clusterID)
}

// CanManageJetStream allows create/update/delete of streams, consumers, KV, and
// object stores at cluster scope.
func CanManageJetStream(user User, clusterID string) bool {
	if user.IsRoot {
		return true
	}
	if hasAdminGrant(user, ResourceSystem, clusterID) {
		return true
	}
	return HighestRole(user.Roles) == RoleAdmin && allowsClusterRules(user, clusterID)
}

// CanManageJetStreamAccount allows JetStream mutations scoped to one account.
func CanManageJetStreamAccount(user User, clusterID, accountName string) bool {
	if CanManageJetStream(user, clusterID) {
		return true
	}
	return hasAdminGrant(user, ResourceAccount, AccountResourceKey(clusterID, accountName))
}

// CanDownloadCreds reports whether the user may download/rotate/mint credentials
// for natsUserID in accountName.
func CanDownloadCreds(user User, clusterID, accountName, natsUserID string) bool {
	if user.IsRoot {
		return true
	}
	if hasAdminGrant(user, ResourceSystem, clusterID) {
		return true
	}
	accountKey := AccountResourceKey(clusterID, accountName)
	natsUserKey := ""
	if !commonstrings.IsEmpty(natsUserID) {
		natsUserKey = NATSUserResourceKey(clusterID, accountName, natsUserID)
	}
	for _, g := range user.Grants {
		switch g.Role {
		case GrantAdmin, GrantCredentialDownloader:
		default:
			continue
		}
		switch g.ResourceType {
		case ResourceAccount:
			if g.ResourceKey == accountKey {
				return true
			}
		case ResourceNATSUser:
			if !commonstrings.IsEmpty(natsUserKey) && g.ResourceKey == natsUserKey {
				return true
			}
		}
	}
	return HighestRole(user.Roles) == RoleAdmin && allowsClusterRules(user, clusterID)
}

func CanManageSystemAccess(user User, clusterID string) bool {
	if user.IsRoot {
		return true
	}
	if hasAdminGrant(user, ResourceSystem, clusterID) {
		return true
	}
	return CanManageUsers(user) && CanAccessCluster(user, clusterID)
}

func CanManageAccountAccess(user User, clusterID, accountName string) bool {
	if CanManageSystemAccess(user, clusterID) {
		return true
	}
	return hasAdminGrant(user, ResourceAccount, AccountResourceKey(clusterID, accountName))
}

// CanMintAdminGrant reports whether the actor may assign the admin grant role
// on the given resource.
func CanMintAdminGrant(user User, resourceType, resourceKey string) bool {
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
	return !commonstrings.IsEmpty(id) && !strings.Contains(id, ":")
}

func hasAdminGrant(user User, resourceType, resourceKey string) bool {
	for _, g := range user.Grants {
		if g.ResourceType == resourceType && g.ResourceKey == resourceKey && g.Role == GrantAdmin {
			return true
		}
	}
	return false
}
