package policy

import "github.com/gopherust-io/nats-consol/internal/domain"

// Authorize evaluates a domain Action against a Resource for user.
func Authorize(user domain.User, action domain.Action, resource domain.Resource) bool {
	return domain.Can(user, action, resource)
}

func AuthorizeWrite(user domain.User) bool {
	return domain.CanWrite(user)
}

func AuthorizeDeleteCluster(user domain.User) bool {
	return domain.CanDeleteCluster(user)
}

func AuthorizeManageUsers(user domain.User) bool {
	return domain.CanManageUsers(user)
}

func AuthorizeViewAudit(user domain.User) bool {
	return domain.CanViewAudit(user)
}

func AuthorizeViewProfiling(user domain.User) bool {
	return domain.CanViewProfiling(user)
}

func AuthorizeManageAlertRules(user domain.User) bool {
	return domain.CanManageAlertRules(user)
}

func AuthorizeAccessCluster(user domain.User, clusterID string) bool {
	return domain.CanAccessCluster(user, clusterID)
}

func AuthorizeAccessClusterOrAccount(user domain.User, clusterID string) bool {
	return domain.CanAccessClusterOrAccount(user, clusterID)
}

func AuthorizeAccessAccount(user domain.User, clusterID, accountName string) bool {
	return domain.CanAccessAccount(user, clusterID, accountName)
}

func AuthorizeWriteCluster(user domain.User, clusterID string) bool {
	return domain.CanWriteCluster(user, clusterID)
}

func AuthorizeManageJetStream(user domain.User, clusterID string) bool {
	return domain.CanManageJetStream(user, clusterID)
}

func AuthorizeManageJetStreamAccount(user domain.User, clusterID, accountName string) bool {
	return domain.CanManageJetStreamAccount(user, clusterID, accountName)
}

func AuthorizeDownloadCreds(user domain.User, clusterID, accountName, natsUserID string) bool {
	return domain.CanDownloadCreds(user, clusterID, accountName, natsUserID)
}

func AuthorizeManageSystemAccess(user domain.User, clusterID string) bool {
	return domain.CanManageSystemAccess(user, clusterID)
}

func AuthorizeManageAccountAccess(user domain.User, clusterID, accountName string) bool {
	return domain.CanManageAccountAccess(user, clusterID, accountName)
}

func AuthorizeMintAdminGrant(user domain.User, resourceType, resourceKey string) bool {
	return domain.CanMintAdminGrant(user, resourceType, resourceKey)
}
