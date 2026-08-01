//go:build integration

package security_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

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

	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Do(&testutil.Request{
		Method: http.MethodGet,
		URL:    "http://nats-consol.local/api/v1/clusters",
		Header: http.Header{
			"Authorization": {basicAuth("scoped-viewer", "scoped-pass")},
		},
	})
	require.NoError(t, err)
	body := resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), clusterID)

	otherCluster := "660e8400-e29b-41d4-a716-446655440001"
	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodGet,
		URL:    srv.BaseURL(otherCluster)+"/streams",
		Header: http.Header{
			"Authorization": {basicAuth("scoped-viewer", "scoped-pass")},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodGet,
		URL:    srv.BaseURL(otherCluster)+"/metrics/history",
		Header: http.Header{
			"Authorization": {basicAuth("scoped-viewer", "scoped-pass")},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestCannotCreateClusterViaAPI(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Do(&testutil.Request{
		Method: http.MethodPost,
		URL:    "http://nats-consol.local/api/v1/clusters",
		Body:   strings.NewReader(`{"name":"blocked","natsUrl":"nats://nats.example:4222","monitoringUrl":"http://nats.example:8222"}`),
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
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

	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Do(&testutil.Request{
		Method: http.MethodPost,
		URL:    "http://nats-consol.local/api/v1/clusters",
		Body:   strings.NewReader(`{"name":"blocked","natsUrl":"nats://nats.example:4222","monitoringUrl":"http://nats.example:8222"}`),
		Header: http.Header{
			"Authorization": {basicAuth("scoped-admin", "scoped-pass")},
			"Content-Type":  {"application/json"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
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

	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Do(&testutil.Request{
		Method: http.MethodGet,
		URL:    "http://nats-consol.local/api/v1/clusters",
		Header: http.Header{
			"Authorization": {basicAuth("no-cluster-viewer", "none-pass")},
		},
	})
	require.NoError(t, err)
	body := resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), `"meta":{"total":0}`)
}
