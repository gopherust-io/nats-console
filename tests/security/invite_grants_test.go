//go:build integration

package security_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/repo"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/tests/testutil"
)

func TestInviteAcceptAndSystemGrant(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()
	clusterID := stack.DefaultClusterID(t)

	pending, err := stack.Store.CreateUser(ctx, repo.UserCreate{
		Username: "invited-person",
		Email:    "invited@example.com",
		Roles:    []string{repo.RoleViewer},
		AccessRules: &repo.AccessRules{
			ClusterIDs: []string{},
		},
	})
	require.NoError(t, err)

	inv, err := stack.Store.CreateUserInvite(ctx, pending.ID, 0)
	require.NoError(t, err)

	_, err = stack.Store.UpsertAccessGrant(ctx, repo.AccessGrantUpsert{
		UserID:       pending.ID,
		ResourceType: repo.ResourceSystem,
		ResourceKey:  clusterID,
		Role:         repo.GrantObserver,
	})
	require.NoError(t, err)

	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Do(&testutil.Request{
		Method: http.MethodPost,
		URL:    "http://nats-consol.local/api/v1/auth/login",
		Body:   strings.NewReader(`{"username":"invited-person","password":"secret-pass"}`),
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodPost,
		URL:    "http://nats-consol.local/api/v1/auth/invite/accept",
		Body:   strings.NewReader(`{"token":"` + inv.Token + `","password":"secret-pass"}`),
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	})
	require.NoError(t, err)
	body := resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, commonstrings.BytesToString(body))

	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodGet,
		URL:    "http://nats-consol.local/api/v1/clusters",
		Header: http.Header{
			"Authorization": {basicAuth("invited-person", "secret-pass")},
		},
	})
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, commonstrings.BytesToString(body), clusterID)

	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodPost,
		URL:    srv.BaseURL(clusterID) + "/streams",
		Body:   strings.NewReader(`{"name":"X","subjects":["x.>"]}`),
		Header: http.Header{
			"Authorization": {basicAuth("invited-person", "secret-pass")},
			"Content-Type":  {"application/json"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestCredentialDownloaderGate(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()
	clusterID := stack.DefaultClusterID(t)

	downloader, err := stack.Store.CreateUser(ctx, repo.UserCreate{
		Username: "cred-dl",
		Email:    "cred@example.com",
		Password: "cred-pass",
		Roles:    []string{repo.RoleViewer},
		AccessRules: &repo.AccessRules{
			ClusterIDs: []string{},
		},
	})
	require.NoError(t, err)

	_, err = stack.Store.UpsertAccessGrant(ctx, repo.AccessGrantUpsert{
		UserID:       downloader.ID,
		ResourceType: repo.ResourceAccount,
		ResourceKey:  clusterID + ":Default",
		Role:         repo.GrantCredentialDownloader,
	})
	require.NoError(t, err)

	natsUser, err := stack.Store.CreateNATSAccountUser(ctx, repo.NATSAccountUserCreate{
		ClusterID:   clusterID,
		AccountName: "Default",
		Name:        "app-user",
	})
	require.NoError(t, err)

	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Do(&testutil.Request{
		Method: http.MethodGet,
		URL:    srv.BaseURL(clusterID) + "/nats-users/" + natsUser.ID + "/creds?account=Default",
		Header: http.Header{
			"Authorization": {basicAuth("cred-dl", "cred-pass")},
		},
	})
	require.NoError(t, err)
	body := resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var payload map[string]any
	require.NoError(t, serializer.Unmarshal(body, &payload))
	data, _ := payload["data"].(map[string]any)
	require.NotNil(t, data)
	assert.NotEmpty(t, data["seed"])
}
