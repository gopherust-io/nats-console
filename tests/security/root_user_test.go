//go:build integration

package security_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootUserSeededAtBootstrap(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()

	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})

	users, err := stack.Store.ListUsers(ctx)
	require.NoError(t, err)
	var root *store.User
	for i := range users {
		if users[i].IsRoot {
			root = &users[i]
			break
		}
	}
	require.NotNil(t, root, "expected seeded root user")
	require.Equal(t, "admin", root.Username)

	req, _ := http.NewRequest(http.MethodGet, "http://nats-consol.local/api/v1/auth/me", nil)
	req.Header.Set("Authorization", basicAuth("admin", "admin"))
	resp, err := srv.Client.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "me status body = %s", string(body))
	assert.Contains(t, string(body), `"isRoot":true`, "expected isRoot in response")
}

func TestDelegatedAdminCannotModifyRoot(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()

	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})

	rootUsers, err := stack.Store.ListUsers(ctx)
	require.NoError(t, err)
	var rootID string
	for _, u := range rootUsers {
		if u.IsRoot {
			rootID = u.ID
			break
		}
	}
	require.NotEmpty(t, rootID, "missing root user")

	delegate, err := stack.Store.CreateUser(ctx, store.UserCreate{
		Username: "delegate-admin",
		Email:    "delegate@example.com",
		Password: "delegate-pass",
		Roles:    []string{store.RoleAdmin},
		AccessRules: &store.AccessRules{
			ClusterIDs:      []string{stack.DefaultClusterID(t)},
			ManageUsers:     true,
			ViewAudit:       false,
			DeleteClusters:  false,
			AssignableRoles: []string{store.RoleOperator, store.RoleViewer},
		},
	})
	require.NoError(t, err)
	_ = delegate

	req, _ := http.NewRequest(http.MethodDelete,
		"http://nats-consol.local/api/v1/users/"+rootID, nil)
	req.Header.Set("Authorization", basicAuth("delegate-admin", "delegate-pass"))
	resp, err := srv.Client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "delete root status")
}

func TestRootCanCreateDelegatedAdmin(t *testing.T) {
	stack := testutil.SetupStack(t)

	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})

	body := `{
		"username":"scoped-admin",
		"email":"scoped@example.com",
		"password":"scoped-pass",
		"roles":["admin"],
		"accessRules":{
			"clusterIds":["` + stack.DefaultClusterID(t) + `"],
			"manageUsers":true,
			"viewAudit":false,
			"deleteClusters":false,
			"assignableRoles":["operator","viewer"]
		}
	}`
	req, _ := http.NewRequest(http.MethodPost, "http://nats-consol.local/api/v1/users", strings.NewReader(body))
	req.Header.Set("Authorization", basicAuth("admin", "admin"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client.Do(req)
	require.NoError(t, err)
	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create admin body = %s", string(respBody))
	assert.Contains(t, string(respBody), `"username":"scoped-admin"`)
}

func TestDelegatedAdminCannotEscalateRoles(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()

	_, err := stack.Store.CreateUser(ctx, store.UserCreate{
		Username: "delegate-admin",
		Email:    "delegate@example.com",
		Password: "delegate-pass",
		Roles:    []string{store.RoleAdmin},
		AccessRules: &store.AccessRules{
			ClusterIDs:      []string{stack.DefaultClusterID(t)},
			ManageUsers:     true,
			AssignableRoles: []string{store.RoleViewer},
		},
	})
	require.NoError(t, err)

	target, err := stack.Store.CreateUser(ctx, store.UserCreate{
		Username:    "target-viewer",
		Email:       "target@example.com",
		Password:    "target-pass",
		Roles:       []string{store.RoleViewer},
		AccessRules: stack.ClusterAccessRules(t),
	})
	require.NoError(t, err)

	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})

	req, _ := http.NewRequest(http.MethodPut,
		"http://nats-consol.local/api/v1/users/"+target.ID+"/roles",
		strings.NewReader(`{"roles":["admin"]}`))
	req.Header.Set("Authorization", basicAuth("delegate-admin", "delegate-pass"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "escalate roles status")
}
