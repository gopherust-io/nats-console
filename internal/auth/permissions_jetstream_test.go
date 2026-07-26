package auth

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestCanManageJetStream(t *testing.T) {
	t.Parallel()
	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	access := &store.AccessRules{ClusterIDs: []string{clusterID}}

	assert.True(t, CanManageJetStream(store.User{IsRoot: true}, clusterID))
	assert.True(t, CanManageJetStream(store.User{
		Roles:       []string{store.RoleAdmin},
		AccessRules: access,
	}, clusterID))
	assert.False(t, CanManageJetStream(store.User{
		Roles:       []string{store.RoleOperator},
		AccessRules: access,
	}, clusterID))
	assert.False(t, CanManageJetStream(store.User{
		Roles:       []string{store.RoleViewer},
		AccessRules: access,
	}, clusterID))
	assert.True(t, CanManageJetStream(store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceSystem,
			ResourceKey:  clusterID,
			Role:         store.GrantAdmin,
		}},
	}, clusterID))
	assert.False(t, CanManageJetStream(store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceAccount,
			ResourceKey:  clusterID + ":Default",
			Role:         store.GrantAdmin,
		}},
	}, clusterID), "account admin must not manage JetStream cluster-wide")
	assert.True(t, CanManageJetStreamAccount(store.User{
		Roles: []string{store.RoleViewer},
		Grants: []store.AccessGrant{{
			ResourceType: store.ResourceAccount,
			ResourceKey:  clusterID + ":Default",
			Role:         store.GrantAdmin,
		}},
	}, clusterID, "Default"))
	assert.False(t, CanManageJetStream(store.User{
		Roles: []string{store.RoleAdmin},
		AccessRules: &store.AccessRules{
			ClusterIDs: []string{"660e8400-e29b-41d4-a716-446655440001"},
		},
	}, clusterID))
}
