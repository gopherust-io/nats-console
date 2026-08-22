package auth

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestCanManageUsersAndViewAudit(t *testing.T) {
	t.Parallel()
	assert.True(t, CanManageUsers(domain.User{IsRoot: true}))
	assert.True(t, CanViewAudit(domain.User{IsRoot: true}))

	legacyAdmin := domain.User{Roles: []string{domain.RoleAdmin}}
	assert.True(t, CanManageUsers(legacyAdmin))
	assert.True(t, CanViewAudit(legacyAdmin))

	scoped := domain.User{
		Roles: []string{domain.RoleAdmin},
		AccessRules: &domain.AccessRules{
			ClusterIDs:  []string{"c1"},
			ManageUsers: true,
			ViewAudit:   false,
		},
	}
	assert.True(t, CanManageUsers(scoped))
	assert.False(t, CanViewAudit(scoped))

	viewer := domain.User{Roles: []string{domain.RoleViewer}}
	assert.False(t, CanManageUsers(viewer))
	assert.False(t, CanViewAudit(viewer))
}

func TestCanDeleteCluster(t *testing.T) {
	t.Parallel()
	assert.True(t, CanDeleteCluster(domain.User{IsRoot: true}))

	legacyAdmin := domain.User{Roles: []string{domain.RoleAdmin}}
	assert.True(t, CanDeleteCluster(legacyAdmin))

	scoped := domain.User{
		Roles: []string{domain.RoleAdmin},
		AccessRules: &domain.AccessRules{
			ClusterIDs:     []string{"c1"},
			DeleteClusters: false,
		},
	}
	assert.False(t, CanDeleteCluster(scoped))

	scopedDelete := domain.User{
		Roles: []string{domain.RoleAdmin},
		AccessRules: &domain.AccessRules{
			ClusterIDs:     []string{"c1"},
			DeleteClusters: true,
		},
	}
	assert.True(t, CanDeleteCluster(scopedDelete))
}

func TestCanAccessClusterRulesAndGrants(t *testing.T) {
	t.Parallel()
	clusterID := "550e8400-e29b-41d4-a716-446655440000"

	assert.True(t, CanAccessCluster(domain.User{IsRoot: true}, clusterID))
	assert.True(t, CanAccessCluster(domain.User{Roles: []string{domain.RoleAdmin}}, clusterID))
	assert.True(t, CanAccessCluster(domain.User{
		Roles:       []string{domain.RoleViewer},
		AccessRules: &domain.AccessRules{ClusterIDs: []string{clusterID}},
	}, clusterID))
	assert.False(t, CanAccessCluster(domain.User{
		Roles:       []string{domain.RoleViewer},
		AccessRules: &domain.AccessRules{ClusterIDs: []string{"other"}},
	}, clusterID))
	assert.True(t, CanAccessCluster(domain.User{
		Roles: []string{domain.RoleViewer},
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceSystem,
			ResourceKey:  clusterID,
			Role:         domain.GrantAdmin,
		}},
	}, clusterID))
	// H1: account/NATS-user scoped grants must not unlock cluster-wide access.
	assert.False(t, CanAccessCluster(domain.User{
		Roles: []string{domain.RoleViewer},
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceAccount,
			ResourceKey:  clusterID + ":Default",
			Role:         domain.GrantAdmin,
		}},
	}, clusterID))
	assert.False(t, CanAccessCluster(domain.User{
		Roles: []string{domain.RoleViewer},
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceNATSUser,
			ResourceKey:  clusterID + ":Default:user-1",
			Role:         domain.GrantAdmin,
		}},
	}, clusterID))
}

func TestCanAccessClusterOrAccount(t *testing.T) {
	t.Parallel()
	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	otherCluster := "660e8400-e29b-41d4-a716-446655440001"

	// System-level access still satisfies the broader check.
	assert.True(t, CanAccessClusterOrAccount(domain.User{IsRoot: true}, clusterID))
	assert.True(t, CanAccessClusterOrAccount(domain.User{
		Roles:       []string{domain.RoleViewer},
		AccessRules: &domain.AccessRules{ClusterIDs: []string{clusterID}},
	}, clusterID))

	// Account/NATS-user scoped grants unlock the "any access" check but not CanAccessCluster.
	accountUser := domain.User{
		Roles: []string{domain.RoleViewer},
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceAccount,
			ResourceKey:  clusterID + ":Default",
			Role:         domain.GrantAdmin,
		}},
	}
	assert.False(t, CanAccessCluster(accountUser, clusterID))
	assert.True(t, CanAccessClusterOrAccount(accountUser, clusterID))
	assert.False(t, CanAccessClusterOrAccount(accountUser, otherCluster))

	natsUserGrant := domain.User{
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceNATSUser,
			ResourceKey:  clusterID + ":Default:user-1",
			Role:         domain.GrantObserver,
		}},
	}
	assert.True(t, CanAccessClusterOrAccount(natsUserGrant, clusterID))

	assert.False(t, CanAccessClusterOrAccount(domain.User{Roles: []string{domain.RoleViewer}}, clusterID))
}

func TestCanAccessAccount(t *testing.T) {
	t.Parallel()
	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	accountA := "Default"
	accountB := "Other"

	assert.True(t, CanAccessAccount(domain.User{IsRoot: true}, clusterID, accountA))
	assert.True(t, CanAccessAccount(domain.User{
		Roles:       []string{domain.RoleViewer},
		AccessRules: &domain.AccessRules{ClusterIDs: []string{clusterID}},
	}, clusterID, accountB))

	accountGrant := domain.User{
		Roles: []string{domain.RoleViewer},
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceAccount,
			ResourceKey:  clusterID + ":" + accountA,
			Role:         domain.GrantObserver,
		}},
	}
	assert.True(t, CanAccessAccount(accountGrant, clusterID, accountA))
	assert.False(t, CanAccessAccount(accountGrant, clusterID, accountB))

	natsUserGrant := domain.User{
		Roles: []string{domain.RoleViewer},
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceNATSUser,
			ResourceKey:  clusterID + ":" + accountA + ":user-1",
			Role:         domain.GrantAdmin,
		}},
	}
	assert.True(t, CanAccessAccount(natsUserGrant, clusterID, accountA))
	assert.False(t, CanAccessAccount(natsUserGrant, clusterID, accountB))
}

func TestCanAccessAccountColonDelimiter(t *testing.T) {
	t.Parallel()
	clusterID := "550e8400-e29b-41d4-a716-446655440000"

	// Account grant for APP:payments must NOT authorize account APP.
	accountGrant := domain.User{
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceAccount,
			ResourceKey:  clusterID + ":APP:payments",
			Role:         domain.GrantObserver,
		}},
	}
	assert.False(t, CanAccessAccount(accountGrant, clusterID, "APP"))
	assert.True(t, CanAccessAccount(accountGrant, clusterID, "APP:payments"))

	// NATS-user grant under account APP:payments must not authorize account APP.
	legacyNATSUserGrant := domain.User{
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceNATSUser,
			ResourceKey:  clusterID + ":APP:payments:user-1",
			Role:         domain.GrantObserver,
		}},
	}
	assert.False(t, CanAccessAccount(legacyNATSUserGrant, clusterID, "APP"))
	assert.True(t, CanAccessAccount(legacyNATSUserGrant, clusterID, "APP:payments"))

	// Grant for exact APP user DOES authorize APP account.
	appUserGrant := domain.User{
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceNATSUser,
			ResourceKey:  clusterID + ":APP:user-1",
			Role:         domain.GrantObserver,
		}},
	}
	assert.True(t, CanAccessAccount(appUserGrant, clusterID, "APP"))
}

func TestCanManageSystemAccessAndMintAdmin(t *testing.T) {
	t.Parallel()
	clusterID := "550e8400-e29b-41d4-a716-446655440000"

	manageUsersOnly := domain.User{
		Roles: []string{domain.RoleViewer},
		AccessRules: &domain.AccessRules{
			ManageUsers: true,
			ClusterIDs:  []string{clusterID},
		},
	}
	assert.True(t, CanManageSystemAccess(manageUsersOnly, clusterID))
	assert.False(t, CanMintAdminGrant(manageUsersOnly, domain.ResourceSystem, clusterID))
	assert.False(t, CanMintAdminGrant(manageUsersOnly, domain.ResourceAccount, clusterID+":Default"))

	manageUsersNoCluster := domain.User{
		Roles: []string{domain.RoleViewer},
		AccessRules: &domain.AccessRules{
			ManageUsers: true,
		},
	}
	assert.False(t, CanManageSystemAccess(manageUsersNoCluster, clusterID))

	systemAdmin := domain.User{
		Roles: []string{domain.RoleViewer},
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceSystem,
			ResourceKey:  clusterID,
			Role:         domain.GrantAdmin,
		}},
	}
	assert.True(t, CanManageSystemAccess(systemAdmin, clusterID))
	assert.True(t, CanMintAdminGrant(systemAdmin, domain.ResourceSystem, clusterID))
}

func TestCanDownloadCredsExactKeys(t *testing.T) {
	t.Parallel()
	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	account := "Default"
	userA := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	userB := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	assert.True(t, CanDownloadCreds(domain.User{IsRoot: true}, clusterID, account, userA))

	accountAdmin := domain.User{
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceAccount,
			ResourceKey:  clusterID + ":" + account,
			Role:         domain.GrantAdmin,
		}},
	}
	assert.True(t, CanDownloadCreds(accountAdmin, clusterID, account, userA))
	assert.True(t, CanDownloadCreds(accountAdmin, clusterID, account, userB))

	natsUserGrant := domain.User{
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceNATSUser,
			ResourceKey:  clusterID + ":" + account + ":" + userA,
			Role:         domain.GrantAdmin,
		}},
	}
	assert.True(t, CanDownloadCreds(natsUserGrant, clusterID, account, userA))
	assert.False(t, CanDownloadCreds(natsUserGrant, clusterID, account, userB))

	credDownloader := domain.User{
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceNATSUser,
			ResourceKey:  clusterID + ":" + account + ":" + userA,
			Role:         domain.GrantCredentialDownloader,
		}},
	}
	assert.True(t, CanDownloadCreds(credDownloader, clusterID, account, userA))
	assert.False(t, CanDownloadCreds(credDownloader, clusterID, account, userB))
}
