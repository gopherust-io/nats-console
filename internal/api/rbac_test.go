package api

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{path: "/api/v1/users", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, clusterIDFromPath(tt.path))
		})
	}
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

	filtered := filterClustersForActor(clusters, actor)
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

	filter, err := auditFilterForActor(actor, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"550e8400-e29b-41d4-a716-446655440000"}, filter.ClusterIDs)

	_, err = auditFilterForActor(actor, "660e8400-e29b-41d4-a716-446655440001")
	require.ErrorIs(t, err, domain.ErrForbidden)
}
