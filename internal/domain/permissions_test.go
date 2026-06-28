package domain_test

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermissionsForRoot(t *testing.T) {
	perms := domain.PermissionsFor(domain.User{IsRoot: true, Roles: []string{domain.RoleAdmin}})
	assert.True(t, perms.ManageUsers, "root should have full permissions: %+v", perms)
	assert.True(t, perms.ViewAudit, "root should have full permissions: %+v", perms)
	assert.True(t, perms.DeleteClusters, "root should have full permissions: %+v", perms)
	assert.True(t, perms.AllowsCluster("any-cluster"), "root should access all clusters")
}

func TestDelegatedAdminPermissions(t *testing.T) {
	user := domain.User{
		Roles: []string{domain.RoleAdmin},
		AccessRules: &domain.AccessRules{
			ClusterIDs:      []string{"cluster-a"},
			ManageUsers:     true,
			ViewAudit:       false,
			DeleteClusters:  false,
			AssignableRoles: []string{domain.RoleOperator, domain.RoleViewer},
		},
	}
	perms := domain.PermissionsFor(user)
	assert.True(t, perms.ManageUsers, "expected manage users")
	assert.False(t, perms.ViewAudit, "audit should be denied")
	assert.False(t, perms.AllowsCluster("cluster-b"), "cluster-b should be denied")
	assert.True(t, perms.AllowsRole(domain.RoleOperator), "should assign operator")
	assert.False(t, perms.AllowsRole(domain.RoleAdmin), "should not assign admin")
}

func TestCannotEscalateAccessRules(t *testing.T) {
	actor := domain.User{
		Roles: []string{domain.RoleAdmin},
		AccessRules: &domain.AccessRules{
			ManageUsers:     true,
			ViewAudit:       false,
			AssignableRoles: []string{domain.RoleViewer},
		},
	}
	perms := domain.PermissionsFor(actor)
	rules := &domain.AccessRules{
		ManageUsers:     true,
		ViewAudit:       true,
		AssignableRoles: []string{domain.RoleAdmin},
	}
	assert.False(t, perms.CanAssignAccessRules(rules), "should not allow escalating audit access")
}

func TestValidateAccessRulesRequiresAssignableRoles(t *testing.T) {
	err := domain.ValidateAccessRules(&domain.AccessRules{ManageUsers: true})
	require.Error(t, err, "expected validation error")
}
