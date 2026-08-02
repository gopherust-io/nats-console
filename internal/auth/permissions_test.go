package auth

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestCanManageUsersAndViewAudit(t *testing.T) {
	t.Parallel()
	assert.True(t, CanManageUsers(store.User{IsRoot: true}))
	assert.True(t, CanViewAudit(store.User{IsRoot: true}))

	legacyAdmin := store.User{Roles: []string{store.RoleAdmin}}
	assert.True(t, CanManageUsers(legacyAdmin))
	assert.True(t, CanViewAudit(legacyAdmin))

	scoped := store.User{
		Roles: []string{store.RoleAdmin},
		AccessRules: &store.AccessRules{
			ClusterIDs:  []string{"c1"},
			ManageUsers: true,
			ViewAudit:   false,
		},
	}
	assert.True(t, CanManageUsers(scoped))
	assert.False(t, CanViewAudit(scoped))

	viewer := store.User{Roles: []string{store.RoleViewer}}
	assert.False(t, CanManageUsers(viewer))
	assert.False(t, CanViewAudit(viewer))
}

func TestCanDeleteCluster(t *testing.T) {
	t.Parallel()
	assert.True(t, CanDeleteCluster(store.User{IsRoot: true}))

	legacyAdmin := store.User{Roles: []string{store.RoleAdmin}}
	assert.True(t, CanDeleteCluster(legacyAdmin))

	scoped := store.User{
		Roles: []string{store.RoleAdmin},
		AccessRules: &store.AccessRules{
			ClusterIDs:     []string{"c1"},
			DeleteClusters: false,
		},
	}
	assert.False(t, CanDeleteCluster(scoped))

	scopedDelete := store.User{
		Roles: []string{store.RoleAdmin},
		AccessRules: &store.AccessRules{
			ClusterIDs:     []string{"c1"},
			DeleteClusters: true,
		},
	}
	assert.True(t, CanDeleteCluster(scopedDelete))
}

func TestCanAccessClusterRulesAndGrants(t *testing.T) {
	t.Parallel()
	clusterID := "550e8400-e29b-41d4-a716-446655440000"

	assert.True(t, CanAccessCluster(store.User{IsRoot: true}, clusterID))
	assert.True(t, CanAccessCluster(store.User{Roles: []string{store.RoleAdmin}}, clusterID))
	assert.True(t, CanAccessCluster(store.User{
		Roles:       []string{store.RoleViewer},
		AccessRules: &store.AccessRules{ClusterIDs: []string{clusterID}},
	}, clusterID))
	assert.False(t, CanAccessCluster(store.User{
		Roles:       []string{store.RoleViewer},
		AccessRules: &store.AccessRules{ClusterIDs: []string{"other"}},
	}, clusterID))
	assert.True(t, CanAccessCluster(store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceSystem,
			ResourceKey:  clusterID,
			Role:         store.GrantAdmin,
		}},
	}, clusterID))
	// H1: account/NATS-user scoped grants must not unlock cluster-wide access.
	assert.False(t, CanAccessCluster(store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceAccount,
			ResourceKey:  clusterID + ":Default",
			Role:         store.GrantAdmin,
		}},
	}, clusterID))
	assert.False(t, CanAccessCluster(store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceNATSUser,
			ResourceKey:  clusterID + ":Default:user-1",
			Role:         store.GrantAdmin,
		}},
	}, clusterID))
}

func TestCanAccessClusterOrAccount(t *testing.T) {
	t.Parallel()
	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	otherCluster := "660e8400-e29b-41d4-a716-446655440001"

	// System-level access still satisfies the broader check.
	assert.True(t, CanAccessClusterOrAccount(store.User{IsRoot: true}, clusterID))
	assert.True(t, CanAccessClusterOrAccount(store.User{
		Roles:       []string{store.RoleViewer},
		AccessRules: &store.AccessRules{ClusterIDs: []string{clusterID}},
	}, clusterID))

	// Account/NATS-user scoped grants unlock the "any access" check but not CanAccessCluster.
	accountUser := store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceAccount,
			ResourceKey:  clusterID + ":Default",
			Role:         store.GrantAdmin,
		}},
	}
	assert.False(t, CanAccessCluster(accountUser, clusterID))
	assert.True(t, CanAccessClusterOrAccount(accountUser, clusterID))
	assert.False(t, CanAccessClusterOrAccount(accountUser, otherCluster))

	natsUserGrant := store.User{
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceNATSUser,
			ResourceKey:  clusterID + ":Default:user-1",
			Role:         store.GrantObserver,
		}},
	}
	assert.True(t, CanAccessClusterOrAccount(natsUserGrant, clusterID))

	assert.False(t, CanAccessClusterOrAccount(store.User{Roles: []string{store.RoleViewer}}, clusterID))
}

func TestCanAccessAccount(t *testing.T) {
	t.Parallel()
	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	accountA := "Default"
	accountB := "Other"

	assert.True(t, CanAccessAccount(store.User{IsRoot: true}, clusterID, accountA))
	assert.True(t, CanAccessAccount(store.User{
		Roles:       []string{store.RoleViewer},
		AccessRules: &store.AccessRules{ClusterIDs: []string{clusterID}},
	}, clusterID, accountB))

	accountGrant := store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceAccount,
			ResourceKey:  clusterID + ":" + accountA,
			Role:         store.GrantObserver,
		}},
	}
	assert.True(t, CanAccessAccount(accountGrant, clusterID, accountA))
	assert.False(t, CanAccessAccount(accountGrant, clusterID, accountB))

	natsUserGrant := store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceNATSUser,
			ResourceKey:  clusterID + ":" + accountA + ":user-1",
			Role:         store.GrantAdmin,
		}},
	}
	assert.True(t, CanAccessAccount(natsUserGrant, clusterID, accountA))
	assert.False(t, CanAccessAccount(natsUserGrant, clusterID, accountB))
}

func TestCanAccessAccountColonDelimiter(t *testing.T) {
	t.Parallel()
	clusterID := "550e8400-e29b-41d4-a716-446655440000"

	// Account grant for APP:payments must NOT authorize account APP.
	accountGrant := store.User{
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceAccount,
			ResourceKey:  clusterID + ":APP:payments",
			Role:         store.GrantObserver,
		}},
	}
	assert.False(t, CanAccessAccount(accountGrant, clusterID, "APP"))
	assert.True(t, CanAccessAccount(accountGrant, clusterID, "APP:payments"))

	// NATS-user grant under account APP:payments must not authorize account APP.
	legacyNATSUserGrant := store.User{
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceNATSUser,
			ResourceKey:  clusterID + ":APP:payments:user-1",
			Role:         store.GrantObserver,
		}},
	}
	assert.False(t, CanAccessAccount(legacyNATSUserGrant, clusterID, "APP"))
	assert.True(t, CanAccessAccount(legacyNATSUserGrant, clusterID, "APP:payments"))

	// Grant for exact APP user DOES authorize APP account.
	appUserGrant := store.User{
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceNATSUser,
			ResourceKey:  clusterID + ":APP:user-1",
			Role:         store.GrantObserver,
		}},
	}
	assert.True(t, CanAccessAccount(appUserGrant, clusterID, "APP"))
}

func TestCanManageSystemAccessAndMintAdmin(t *testing.T) {
	t.Parallel()
	clusterID := "550e8400-e29b-41d4-a716-446655440000"

	manageUsersOnly := store.User{
		Roles: []string{store.RoleViewer},
		AccessRules: &store.AccessRules{
			ManageUsers: true,
			ClusterIDs:  []string{clusterID},
		},
	}
	assert.True(t, CanManageSystemAccess(manageUsersOnly, clusterID))
	assert.False(t, CanMintAdminGrant(manageUsersOnly, store.ResourceSystem, clusterID))
	assert.False(t, CanMintAdminGrant(manageUsersOnly, store.ResourceAccount, clusterID+":Default"))

	manageUsersNoCluster := store.User{
		Roles: []string{store.RoleViewer},
		AccessRules: &store.AccessRules{
			ManageUsers: true,
		},
	}
	assert.False(t, CanManageSystemAccess(manageUsersNoCluster, clusterID))

	systemAdmin := store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceSystem,
			ResourceKey:  clusterID,
			Role:         store.GrantAdmin,
		}},
	}
	assert.True(t, CanManageSystemAccess(systemAdmin, clusterID))
	assert.True(t, CanMintAdminGrant(systemAdmin, store.ResourceSystem, clusterID))
}

func TestCanDownloadCredsExactKeys(t *testing.T) {
	t.Parallel()
	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	account := "Default"
	userA := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	userB := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	assert.True(t, CanDownloadCreds(store.User{IsRoot: true}, clusterID, account, userA))

	accountAdmin := store.User{
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceAccount,
			ResourceKey:  clusterID + ":" + account,
			Role:         store.GrantAdmin,
		}},
	}
	assert.True(t, CanDownloadCreds(accountAdmin, clusterID, account, userA))
	assert.True(t, CanDownloadCreds(accountAdmin, clusterID, account, userB))

	natsUserGrant := store.User{
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceNATSUser,
			ResourceKey:  clusterID + ":" + account + ":" + userA,
			Role:         store.GrantAdmin,
		}},
	}
	assert.True(t, CanDownloadCreds(natsUserGrant, clusterID, account, userA))
	assert.False(t, CanDownloadCreds(natsUserGrant, clusterID, account, userB))

	credDownloader := store.User{
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceNATSUser,
			ResourceKey:  clusterID + ":" + account + ":" + userA,
			Role:         store.GrantCredentialDownloader,
		}},
	}
	assert.True(t, CanDownloadCreds(credDownloader, clusterID, account, userA))
	assert.False(t, CanDownloadCreds(credDownloader, clusterID, account, userB))
}
