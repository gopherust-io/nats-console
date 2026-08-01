//go:build integration

package security_test

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gopherust-io/nats-consol/internal/store"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/require"
)

func TestProtectedRoutesRequireAuth(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)

	paths := []string{
		"http://nats-consol.local/api/v1/clusters",
		srv.BaseURL(clusterID) + "/streams",
		"http://nats-consol.local/api/v1/auth/me",
	}
	for _, path := range paths {
		resp, err := srv.UnauthClient.Get(path)
		require.NoError(t, err)
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "%s", path)
	}
}

func TestPublicRoutesAccessibleWithoutAuth(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	paths := []string{
		"http://nats-consol.local/api/health",
		"http://nats-consol.local/api/v1/auth/config",
	}
	for _, path := range paths {
		resp, err := srv.UnauthClient.Get(path)
		require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode, "%s", path)
	}
}

func TestBasicAuthGrantsAccess(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Get("http://nats-consol.local/api/v1/clusters")
	require.NoError(t, err)
	body := resp.Body
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

	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)

	resp, err := srv.Client.Do(&testutil.Request{
		Method: http.MethodPost,
		URL:    srv.BaseURL(clusterID)+"/streams",
		Body:   strings.NewReader(`{"name":"BLOCKED","subjects":["x.>"]}`),
		Header: http.Header{
			"Authorization": {basicAuth("viewer-user", "viewer-pass")},
			"Content-Type": {"application/json"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "viewer POST status")
}

func TestOperatorCannotMutateJetStream(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()

	_, err := stack.Store.CreateUser(ctx, store.UserCreate{
		Username:    "op-js",
		Email:       "op-js@example.com",
		Password:    "op-pass",
		Roles:       []string{store.RoleOperator},
		AccessRules: stack.ClusterAccessRules(t),
	})
	require.NoError(t, err)

	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)
	authz := basicAuth("op-js", "op-pass")

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/streams", body: `{"name":"OPBLOCK","subjects":["x.>"]}`},
		{method: http.MethodPost, path: "/kv/buckets", body: `{"bucket":"opkv"}`},
		{method: http.MethodPost, path: "/objects/buckets", body: `{"bucket":"opobj"}`},
		{method: http.MethodDelete, path: "/streams/NOSUCH", body: ""},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var body io.Reader
			hdr := http.Header{"Authorization": {authz}}
			if !commonstrings.IsEmpty(tc.body) {
				body = strings.NewReader(tc.body)
				hdr.Set("Content-Type", "application/json")
			}
			resp, err := srv.Client.Do(&testutil.Request{
				Method: tc.method,
				URL:    srv.BaseURL(clusterID) + tc.path,
				Body:   body,
				Header: hdr,
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusForbidden, resp.StatusCode)
		})
	}
}

func TestAdminCanCreateStream(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)

	resp, err := srv.Client.Do(&testutil.Request{
		Method: http.MethodPost,
		URL:    srv.BaseURL(clusterID) + "/streams",
		Body:   strings.NewReader(`{"name":"ADMINOK","subjects":["admin.>"]}`),
		Header: http.Header{
			"Authorization": {basicAuth("admin", "admin")},
			"Content-Type":  {"application/json"},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated,
		"admin POST stream status=%d", resp.StatusCode)
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

	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Do(&testutil.Request{
		Method: http.MethodPut,
		URL:    "http://nats-consol.local/api/v1/users/"+viewer.ID+"/roles",
		Body:   strings.NewReader(`{"roles":["admin"]}`),
		Header: http.Header{
			"Authorization": {basicAuth("operator-user", "op-pass")},
			"Content-Type": {"application/json"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "operator set roles status")
}

func basicAuth(user, pass string) string {
	creds := base64.StdEncoding.EncodeToString(commonstrings.StringToBytes(user + ":" + pass))
	return "Basic " + creds
}
