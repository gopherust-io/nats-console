package apikit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

func TestClusterIDFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/api/v1/clusters", want: ""},
		{path: "/api/v1/clusters/connections", want: ""},
		{path: "/api/v1/clusters/not-a-uuid/streams", want: ""},
		{
			path: "/api/v1/clusters/550e8400-e29b-41d4-a716-446655440000/streams",
			want: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			path: "/api/v1/clusters/550E8400-E29B-41D4-A716-446655440000/streams",
			want: "550e8400-e29b-41d4-a716-446655440000",
		},
		{path: "/api/v1/users", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, ClusterIDFromPath(tt.path))
		})
	}
}

func TestIsJetStreamResourcePath(t *testing.T) {
	id := "550e8400-e29b-41d4-a716-446655440000"
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/streams"))
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/streams/ORDERS/consumers"))
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/kv/buckets"))
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/objects/buckets/x"))
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/request-reply"))
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/request-reply/probes"))
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/zombies"))
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/subject-naming"))
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/event-genome"))
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/event-catalog"))
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/event-catalog/orders.created"))
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/event-wikipedia"))
	assert.False(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/account"))
	assert.False(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/nats-users"))
	assert.False(t, IsJetStreamResourcePath("/api/v1/clusters"))
}

func TestFilterClustersForActor(t *testing.T) {
	clusters := []domain.Cluster{
		{ID: "550e8400-e29b-41d4-a716-446655440000", Name: "allowed"},
		{ID: "660e8400-e29b-41d4-a716-446655440001", Name: "denied"},
	}
	actor := domain.User{
		Roles: []string{domain.RoleAdmin},
		AccessRules: &domain.AccessRules{
			ClusterIDs: []string{"550e8400-e29b-41d4-a716-446655440000"},
		},
	}

	filtered := FilterClustersForActor(clusters, actor)
	require.Len(t, filtered, 1)
	assert.Equal(t, "allowed", filtered[0].Name)
}

func TestFilterClustersForScopedOperator(t *testing.T) {
	clusters := []domain.Cluster{
		{ID: "550e8400-e29b-41d4-a716-446655440000", Name: "allowed"},
		{ID: "660e8400-e29b-41d4-a716-446655440001", Name: "denied"},
	}
	actor := domain.User{
		Roles: []string{domain.RoleOperator},
		AccessRules: &domain.AccessRules{
			ClusterIDs: []string{"550e8400-e29b-41d4-a716-446655440000"},
		},
	}

	filtered := FilterClustersForActor(clusters, actor)
	require.Len(t, filtered, 1)
	assert.Equal(t, "allowed", filtered[0].Name)
}

func TestAuditFilterForActor(t *testing.T) {
	actor := domain.User{
		Roles: []string{domain.RoleAdmin},
		AccessRules: &domain.AccessRules{
			ClusterIDs:  []string{"550e8400-e29b-41d4-a716-446655440000"},
			ViewAudit:   true,
			ManageUsers: true,
		},
	}

	filter, err := AuditFilterForActor(actor, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"550e8400-e29b-41d4-a716-446655440000"}, filter.ClusterIDs)

	_, err = AuditFilterForActor(actor, "660e8400-e29b-41d4-a716-446655440001")
	require.ErrorIs(t, err, domain.ErrForbidden)

	emptyScope := domain.User{
		Roles: []string{domain.RoleAdmin},
		AccessRules: &domain.AccessRules{
			ViewAudit:   true,
			ManageUsers: true,
		},
	}
	filter, err = AuditFilterForActor(emptyScope, "")
	require.NoError(t, err)
	require.NotNil(t, filter.ClusterIDs)
	assert.Empty(t, filter.ClusterIDs, "empty scope must not omit ClusterIDs (would leak all audit rows)")
}
