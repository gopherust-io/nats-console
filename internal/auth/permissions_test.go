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

func TestCanCreateAndDeleteCluster(t *testing.T) {
	t.Parallel()
	assert.True(t, CanCreateCluster(store.User{IsRoot: true}))
	assert.True(t, CanDeleteCluster(store.User{IsRoot: true}))

	legacyAdmin := store.User{Roles: []string{store.RoleAdmin}}
	assert.True(t, CanCreateCluster(legacyAdmin))
	assert.True(t, CanDeleteCluster(legacyAdmin))

	scoped := store.User{
		Roles: []string{store.RoleAdmin},
		AccessRules: &store.AccessRules{
			ClusterIDs:     []string{"c1"},
			DeleteClusters: false,
		},
	}
	assert.False(t, CanCreateCluster(scoped))
	assert.False(t, CanDeleteCluster(scoped))

	scopedDelete := store.User{
		Roles: []string{store.RoleAdmin},
		AccessRules: &store.AccessRules{
			ClusterIDs:     []string{"c1"},
			DeleteClusters: true,
		},
	}
	assert.False(t, CanCreateCluster(scopedDelete))
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
	assert.True(t, CanAccessCluster(store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceAccount,
			ResourceKey:  clusterID + ":Default",
			Role:         store.GrantAdmin,
		}},
	}, clusterID))
}
