package auth

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestCanManageJetStream(t *testing.T) {
	t.Parallel()
	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	access := &domain.AccessRules{ClusterIDs: []string{clusterID}}

	assert.True(t, CanManageJetStream(domain.User{IsRoot: true}, clusterID))
	assert.True(t, CanManageJetStream(domain.User{
		Roles:       []string{domain.RoleAdmin},
		AccessRules: access,
	}, clusterID))
	assert.False(t, CanManageJetStream(domain.User{
		Roles:       []string{domain.RoleOperator},
		AccessRules: access,
	}, clusterID))
	assert.False(t, CanManageJetStream(domain.User{
		Roles:       []string{domain.RoleViewer},
		AccessRules: access,
	}, clusterID))
	assert.True(t, CanManageJetStream(domain.User{
		Roles: []string{domain.RoleViewer},
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceSystem,
			ResourceKey:  clusterID,
			Role:         domain.GrantAdmin,
		}},
	}, clusterID))
	assert.False(t, CanManageJetStream(domain.User{
		Roles: []string{domain.RoleViewer},
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceAccount,
			ResourceKey:  clusterID + ":Default",
			Role:         domain.GrantAdmin,
		}},
	}, clusterID), "account admin must not manage JetStream cluster-wide")
	assert.True(t, CanManageJetStreamAccount(domain.User{
		Roles: []string{domain.RoleViewer},
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceAccount,
			ResourceKey:  clusterID + ":Default",
			Role:         domain.GrantAdmin,
		}},
	}, clusterID, "Default"))
	assert.False(t, CanManageJetStream(domain.User{
		Roles: []string{domain.RoleAdmin},
		AccessRules: &domain.AccessRules{
			ClusterIDs: []string{"660e8400-e29b-41d4-a716-446655440001"},
		},
	}, clusterID))
}
