package auth

import (
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/repo"
)

// StoreUserToDomain converts a DB-scanned repo.User into domain.User at the
// auth/persistence boundary (includes SessionVersion, AccessRules, Grants).
func StoreUserToDomain(user repo.User) domain.User {
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
	grants := make([]domain.AccessGrant, len(user.Grants))
	copy(grants, user.Grants)

	return domain.User{
		ID:             user.ID,
		Username:       user.Username,
		Email:          user.Email,
		OIDCSub:        user.OIDCSub,
		Roles:          append([]string(nil), user.Roles...),
		IsRoot:         user.IsRoot,
		AccessRules:    rules,
		Grants:         grants,
		CreatedAt:      user.CreatedAt,
		SessionVersion: user.SessionVersion,
	}
}

func CanWrite(user domain.User) bool {
	return domain.CanWrite(user)
}

func CanDeleteCluster(user domain.User) bool {
	return domain.CanDeleteCluster(user)
}

func CanManageUsers(user domain.User) bool {
	return domain.CanManageUsers(user)
}

func CanViewAudit(user domain.User) bool {
	return domain.CanViewAudit(user)
}

func CanViewProfiling(user domain.User) bool {
	return domain.CanViewProfiling(user)
}

func CanManageAlertRules(user domain.User) bool {
	return domain.CanManageAlertRules(user)
}

func CanAccessCluster(user domain.User, clusterID string) bool {
	return domain.CanAccessCluster(user, clusterID)
}

func CanAccessClusterOrAccount(user domain.User, clusterID string) bool {
	return domain.CanAccessClusterOrAccount(user, clusterID)
}

func CanAccessAccount(user domain.User, clusterID, accountName string) bool {
	return domain.CanAccessAccount(user, clusterID, accountName)
}

func CanWriteCluster(user domain.User, clusterID string) bool {
	return domain.CanWriteCluster(user, clusterID)
}

func CanManageJetStream(user domain.User, clusterID string) bool {
	return domain.CanManageJetStream(user, clusterID)
}

func CanManageJetStreamAccount(user domain.User, clusterID, accountName string) bool {
	return domain.CanManageJetStreamAccount(user, clusterID, accountName)
}

func CanDownloadCreds(user domain.User, clusterID, accountName, natsUserID string) bool {
	return domain.CanDownloadCreds(user, clusterID, accountName, natsUserID)
}

func CanManageSystemAccess(user domain.User, clusterID string) bool {
	return domain.CanManageSystemAccess(user, clusterID)
}

func CanManageAccountAccess(user domain.User, clusterID, accountName string) bool {
	return domain.CanManageAccountAccess(user, clusterID, accountName)
}

func CanMintAdminGrant(user domain.User, resourceType, resourceKey string) bool {
	return domain.CanMintAdminGrant(user, resourceType, resourceKey)
}
