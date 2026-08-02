//go:build integration

package security_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gopherust-io/nats-consol/internal/store"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootUserSeededAtBootstrap(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()

	srv := stack.NewServer(t, nil)

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

	resp, err := srv.Client.Do(&testutil.Request{
		Method: http.MethodGet,
		URL:    "http://nats-consol.local/api/v1/auth/me",
		Header: http.Header{
			"Authorization": {basicAuth("admin", "admin")},
		},
	})
	require.NoError(t, err)
	body := resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "me status body = %s", commonstrings.BytesToString(body))
	assert.Contains(t, commonstrings.BytesToString(body), `"isRoot":true`, "expected isRoot in response")
}

func TestDelegatedAdminCannotModifyRoot(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()

	srv := stack.NewServer(t, nil)

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

	resp, err := srv.Client.Do(&testutil.Request{
		Method: http.MethodDelete,
		URL:    "http://nats-consol.local/api/v1/users/" + rootID,
		Header: http.Header{
			"Authorization": {basicAuth("delegate-admin", "delegate-pass")},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "delete root status")
}

func TestRootCanCreateDelegatedAdmin(t *testing.T) {
	stack := testutil.SetupStack(t)

	srv := stack.NewServer(t, nil)

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
	resp, err := srv.Client.Do(&testutil.Request{
		Method: http.MethodPost,
		URL:    "http://nats-consol.local/api/v1/users",
		Body:   strings.NewReader(body),
		Header: http.Header{
			"Authorization": {basicAuth("admin", "admin")},
			"Content-Type":  {"application/json"},
		},
	})
	require.NoError(t, err)
	respBody := resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create admin body = %s", commonstrings.BytesToString(respBody))
	assert.Contains(t, commonstrings.BytesToString(respBody), `"username":"scoped-admin"`)
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

	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Do(&testutil.Request{
		Method: http.MethodPut,
		URL:    "http://nats-consol.local/api/v1/users/" + target.ID + "/roles",
		Body:   strings.NewReader(`{"roles":["admin"]}`),
		Header: http.Header{
			"Authorization": {basicAuth("delegate-admin", "delegate-pass")},
			"Content-Type":  {"application/json"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "escalate roles status")
}
