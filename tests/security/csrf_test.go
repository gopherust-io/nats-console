//go:build integration

package security_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/repo"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/tests/testutil"
)

func TestCSRFBlocksSessionCookieMutations(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	loginResp, err := srv.Client.Post(
		"http://nats-consol.local/api/v1/auth/login",
		"application/json",
		strings.NewReader(`{"username":"admin","password":"admin"}`),
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, loginResp.StatusCode, "login status")

	for _, c := range loginResp.Cookies() {
		if c.Name == "nats_consol_session" {
			clusterID := stack.DefaultClusterID(t)
			resp, err := srv.UnauthClient.Do(&testutil.Request{
				Method:  http.MethodPost,
				URL:     "http://nats-consol.local/api/v1/clusters/" + clusterID + "/test",
				Cookies: []*testutil.Cookie{c},
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusForbidden, resp.StatusCode, "missing csrf")
			var envelope struct {
				Error httpstatus.ErrorBody `json:"error"`
			}
			require.NoError(t, sonic.Unmarshal(resp.Body, &envelope))
			assert.Equal(t, httpstatus.CodeCSRFInvalid, envelope.Error.Code)
			assert.Contains(t, envelope.Error.Message, "csrf")
			return
		}
	}
	require.Fail(t, "session cookie not set")
}

func TestExpiredInviteReturnsGone(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()

	pending, err := stack.Store.CreateUser(ctx, repo.UserCreate{
		Username: "expired-invitee",
		Email:    "expired-invite@example.com",
		Roles:    []string{repo.RoleViewer},
	})
	require.NoError(t, err)

	inv, err := stack.Store.CreateUserInvite(ctx, pending.ID, time.Nanosecond)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	srv := stack.NewServer(t, nil)
	resp, err := srv.Client.Get("http://nats-consol.local/api/v1/auth/invite/" + inv.Token)
	require.NoError(t, err)
	require.Equal(t, http.StatusGone, resp.StatusCode, commonstrings.BytesToString(resp.Body))

	var envelope struct {
		Error httpstatus.ErrorBody `json:"error"`
	}
	require.NoError(t, sonic.Unmarshal(resp.Body, &envelope))
	assert.Equal(t, httpstatus.CodeGone, envelope.Error.Code)
	assert.Contains(t, envelope.Error.Message, "invite")
}
