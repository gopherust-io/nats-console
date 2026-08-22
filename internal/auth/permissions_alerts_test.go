package auth

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestCanManageAlertRules(t *testing.T) {
	t.Parallel()
	assert.True(t, CanManageAlertRules(domain.User{IsRoot: true}))
	assert.True(t, CanManageAlertRules(domain.User{Roles: []string{domain.RoleAdmin}}))
	assert.False(t, CanManageAlertRules(domain.User{Roles: []string{domain.RoleViewer}}))
	assert.True(t, CanManageAlertRules(domain.User{
		Roles:       []string{domain.RoleOperator},
		AccessRules: &domain.AccessRules{ManageUsers: true},
	}))
}
