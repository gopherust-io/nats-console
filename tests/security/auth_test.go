//go:build integration

package security_test

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/require"
)

func TestProtectedRoutesRequireAuth(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})
	clusterID := stack.DefaultClusterID(t)

	paths := []string{
		"http://nats-consol.local/api/v1/clusters",
		srv.BaseURL(clusterID) + "/streams",
		"http://nats-consol.local/api/v1/auth/me",
	}
	for _, path := range paths {
		resp, err := srv.Client.Get(path)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "%s", path)
	}
}

func TestPublicRoutesAccessibleWithoutAuth(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})

	paths := []string{
		"http://nats-consol.local/api/health",
		"http://nats-consol.local/api/v1/auth/config",
	}
	for _, path := range paths {
		resp, err := srv.Client.Get(path)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "%s", path)
	}
}

func TestBasicAuthGrantsAccess(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})

	req, _ := http.NewRequest(http.MethodGet, "http://nats-consol.local/api/v1/clusters", nil)
	req.Header.Set("Authorization", basicAuth("admin", "admin"))
	resp, err := srv.Client.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "status body = %s", string(body))
	testutil.AssertNoKeys(t, body, "token", "password_hash")
}

func TestViewerCannotMutateStreams(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()

	viewer, err := stack.Store.CreateUser(ctx, store.UserCreate{
		Username:    "viewer-user",
		Email:       "viewer@example.com",
		Password:    "viewer-pass",
		Roles:       []string{store.RoleViewer},
		AccessRules: stack.ClusterAccessRules(t),
	})
	require.NoError(t, err)
	_ = viewer

	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})
	clusterID := stack.DefaultClusterID(t)

	req, _ := http.NewRequest(http.MethodPost, srv.BaseURL(clusterID)+"/streams",
		strings.NewReader(`{"name":"BLOCKED","subjects":["x.>"]}`))
	req.Header.Set("Authorization", basicAuth("viewer-user", "viewer-pass"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "viewer POST status")
}

func TestOperatorCannotManageUsers(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()

	op, err := stack.Store.CreateUser(ctx, store.UserCreate{
		Username:    "operator-user",
		Email:       "op@example.com",
		Password:    "op-pass",
		Roles:       []string{store.RoleOperator},
		AccessRules: stack.ClusterAccessRules(t),
	})
	require.NoError(t, err)

	viewer, err := stack.Store.CreateUser(ctx, store.UserCreate{
		Username:    "target-viewer",
		Email:       "target@example.com",
		Password:    "v-pass",
		Roles:       []string{store.RoleViewer},
		AccessRules: stack.ClusterAccessRules(t),
	})
	require.NoError(t, err)
	_ = op

	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})

	req, _ := http.NewRequest(http.MethodPut,
		"http://nats-consol.local/api/v1/users/"+viewer.ID+"/roles",
		strings.NewReader(`{"roles":["admin"]}`))
	req.Header.Set("Authorization", basicAuth("operator-user", "op-pass"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "operator set roles status")
}

func basicAuth(user, pass string) string {
	creds := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	return "Basic " + creds
}
