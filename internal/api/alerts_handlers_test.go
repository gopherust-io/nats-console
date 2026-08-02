package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
)

func TestFilterAlertRulesForActorScopesClusters(t *testing.T) {
	t.Parallel()

	clusterA := "550e8400-e29b-41d4-a716-446655440000"
	clusterB := "550e8400-e29b-41d4-a716-446655440001"
	actor := domain.User{
		Roles: []string{domain.RoleOperator},
		AccessRules: &domain.AccessRules{
			ClusterIDs:  []string{clusterA},
			ManageUsers: true,
		},
	}
	storeUser := store.User{
		Roles: []string{store.RoleOperator},
		AccessRules: &store.AccessRules{
			ClusterIDs:  []string{clusterA},
			ManageUsers: true,
		},
	}
	rules := []domain.AlertRule{
		{ID: "1", ClusterID: clusterA, Name: "a"},
		{ID: "2", ClusterID: clusterB, Name: "b"},
		{ID: "3", ClusterID: "", Name: "global"},
	}

	got := filterAlertRulesForActor(rules, actor, storeUser)
	assert.Len(t, got, 1)
	assert.Equal(t, "1", got[0].ID)
}

func TestFilterAlertRulesForActorAllClustersUnfiltered(t *testing.T) {
	t.Parallel()

	actor := domain.User{IsRoot: true}
	storeUser := store.User{IsRoot: true}
	rules := []domain.AlertRule{
		{ID: "1", ClusterID: "a"},
		{ID: "2", ClusterID: "b"},
	}
	got := filterAlertRulesForActor(rules, actor, storeUser)
	assert.Len(t, got, 2)
}

func TestCanAccessAlertRuleClusterGlobalRequiresUnscoped(t *testing.T) {
	t.Parallel()

	clusterA := "550e8400-e29b-41d4-a716-446655440000"
	scopedActor := domain.User{
		Roles: []string{domain.RoleOperator},
		AccessRules: &domain.AccessRules{
			ClusterIDs:  []string{clusterA},
			ManageUsers: true,
		},
	}
	scopedStore := store.User{
		Roles: []string{store.RoleOperator},
		AccessRules: &store.AccessRules{
			ClusterIDs:  []string{clusterA},
			ManageUsers: true,
		},
	}
	assert.False(t, canAccessAlertRuleCluster(scopedActor, scopedStore, ""))
	assert.True(t, canAccessAlertRuleCluster(scopedActor, scopedStore, clusterA))
	assert.False(t, canManageGlobalAlertRules(scopedActor, scopedStore))

	root := domain.User{IsRoot: true}
	rootStore := store.User{IsRoot: true}
	assert.True(t, canAccessAlertRuleCluster(root, rootStore, ""))
	assert.True(t, canManageGlobalAlertRules(root, rootStore))
}
