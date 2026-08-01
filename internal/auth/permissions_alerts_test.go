package auth

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestCanManageAlertRules(t *testing.T) {
	t.Parallel()
	assert.True(t, CanManageAlertRules(store.User{IsRoot: true}))
	assert.True(t, CanManageAlertRules(store.User{Roles: []string{store.RoleAdmin}}))
	assert.False(t, CanManageAlertRules(store.User{Roles: []string{store.RoleViewer}}))
	assert.True(t, CanManageAlertRules(store.User{
		Roles:       []string{store.RoleOperator},
		AccessRules: &store.AccessRules{ManageUsers: true},
	}))
}
