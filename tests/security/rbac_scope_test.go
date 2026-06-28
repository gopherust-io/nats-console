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

func TestScopedViewerCannotAccessOtherCluster(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()
	clusterID := stack.DefaultClusterID(t)

	_, err := stack.Store.CreateUser(ctx, store.UserCreate{
		Username: "scoped-viewer",
		Email:    "scoped@example.com",
		Password: "scoped-pass",
		Roles:    []string{store.RoleViewer},
		AccessRules: &store.AccessRules{
			ClusterIDs: []string{clusterID},
		},
	})
	require.NoError(t, err)

	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})

	req, _ := http.NewRequest(http.MethodGet, "http://nats-consol.local/api/v1/clusters", nil)
	req.Header.Set("Authorization", basicAuth("scoped-viewer", "scoped-pass"))
	resp, err := srv.Client.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), clusterID)

	otherCluster := "660e8400-e29b-41d4-a716-446655440001"
	req, _ = http.NewRequest(http.MethodGet, srv.BaseURL(otherCluster)+"/streams", nil)
	req.Header.Set("Authorization", basicAuth("scoped-viewer", "scoped-pass"))
	resp, err = srv.Client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	req, _ = http.NewRequest(http.MethodGet, srv.BaseURL(otherCluster)+"/metrics/history", nil)
	req.Header.Set("Authorization", basicAuth("scoped-viewer", "scoped-pass"))
	resp, err = srv.Client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestScopedAdminCannotCreateCluster(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()

	_, err := stack.Store.CreateUser(ctx, store.UserCreate{
		Username: "scoped-admin",
		Email:    "scoped@example.com",
		Password: "scoped-pass",
		Roles:    []string{store.RoleAdmin},
		AccessRules: &store.AccessRules{
			ClusterIDs:      []string{stack.DefaultClusterID(t)},
			ManageUsers:     true,
			AssignableRoles: []string{store.RoleViewer},
		},
	})
	require.NoError(t, err)

	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})

	req, _ := http.NewRequest(http.MethodPost, "http://nats-consol.local/api/v1/clusters",
		strings.NewReader(`{"name":"blocked","natsUrl":"nats://nats.example:4222","monitoringUrl":"http://nats.example:8222"}`))
	req.Header.Set("Authorization", basicAuth("scoped-admin", "scoped-pass"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestViewerWithoutClusterAccessGetsEmptyList(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()

	_, err := stack.Store.CreateUser(ctx, store.UserCreate{
		Username: "no-cluster-viewer",
		Email:    "none@example.com",
		Password: "none-pass",
		Roles:    []string{store.RoleViewer},
		AccessRules: &store.AccessRules{
			ClusterIDs: []string{},
		},
	})
	require.NoError(t, err)

	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})

	req, _ := http.NewRequest(http.MethodGet, "http://nats-consol.local/api/v1/clusters", nil)
	req.Header.Set("Authorization", basicAuth("no-cluster-viewer", "none-pass"))
	resp, err := srv.Client.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), `"total":0`)
}
