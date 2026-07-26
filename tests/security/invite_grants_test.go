//go:build integration

package security_test

import (
	"context"
	"encoding/json"
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

func TestInviteAcceptAndSystemGrant(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()
	clusterID := stack.DefaultClusterID(t)

	pending, err := stack.Store.CreateUser(ctx, store.UserCreate{
		Username: "invited-person",
		Email:    "invited@example.com",
		Roles:    []string{store.RoleViewer},
		AccessRules: &store.AccessRules{
			ClusterIDs: []string{},
		},
	})
	require.NoError(t, err)

	inv, err := stack.Store.CreateUserInvite(ctx, pending.ID, 0)
	require.NoError(t, err)

	_, err = stack.Store.UpsertAccessGrant(ctx, store.AccessGrantUpsert{
		UserID:       pending.ID,
		ResourceType: store.ResourceSystem,
		ResourceKey:  clusterID,
		Role:         store.GrantObserver,
	})
	require.NoError(t, err)

	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})

	req, _ := http.NewRequest(http.MethodPost, "http://nats-consol.local/api/v1/auth/login", strings.NewReader(`{"username":"invited-person","password":"secret-pass"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	req, _ = http.NewRequest(http.MethodPost, "http://nats-consol.local/api/v1/auth/invite/accept", strings.NewReader(`{"token":"`+inv.Token+`","password":"secret-pass"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err = srv.Client.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	req, _ = http.NewRequest(http.MethodGet, "http://nats-consol.local/api/v1/clusters", nil)
	req.Header.Set("Authorization", basicAuth("invited-person", "secret-pass"))
	resp, err = srv.Client.Do(req)
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), clusterID)

	req, _ = http.NewRequest(http.MethodPost, srv.BaseURL(clusterID)+"/streams", strings.NewReader(`{"name":"X","subjects":["x.>"]}`))
	req.Header.Set("Authorization", basicAuth("invited-person", "secret-pass"))
	req.Header.Set("Content-Type", "application/json")
	resp, err = srv.Client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestCredentialDownloaderGate(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()
	clusterID := stack.DefaultClusterID(t)

	downloader, err := stack.Store.CreateUser(ctx, store.UserCreate{
		Username: "cred-dl",
		Email:    "cred@example.com",
		Password: "cred-pass",
		Roles:    []string{store.RoleViewer},
		AccessRules: &store.AccessRules{
			ClusterIDs: []string{},
		},
	})
	require.NoError(t, err)

	_, err = stack.Store.UpsertAccessGrant(ctx, store.AccessGrantUpsert{
		UserID:       downloader.ID,
		ResourceType: store.ResourceAccount,
		ResourceKey:  clusterID + ":Default",
		Role:         store.GrantCredentialDownloader,
	})
	require.NoError(t, err)

	natsUser, err := stack.Store.CreateNATSAccountUser(ctx, store.NATSAccountUserCreate{
		ClusterID:   clusterID,
		AccountName: "Default",
		Name:        "app-user",
	})
	require.NoError(t, err)

	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})

	req, _ := http.NewRequest(http.MethodGet, srv.BaseURL(clusterID)+"/nats-users/"+natsUser.ID+"/creds?account=Default", nil)
	req.Header.Set("Authorization", basicAuth("cred-dl", "cred-pass"))
	resp, err := srv.Client.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.NotEmpty(t, payload["seed"])
}
