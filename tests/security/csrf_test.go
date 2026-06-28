//go:build integration

package security_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/require"
)

func TestCSRFBlocksSessionCookieMutations(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})

	loginResp, err := srv.Client.Post(
		"http://nats-consol.local/api/v1/auth/login",
		"application/json",
		strings.NewReader(`{"username":"admin","password":"admin"}`),
	)
	require.NoError(t, err)
	_, _ = io.ReadAll(loginResp.Body)
	_ = loginResp.Body.Close()
	require.Equal(t, http.StatusOK, loginResp.StatusCode, "login status")

	client := &http.Client{
		Transport: srv.Client.Transport,
		Jar:       nil,
	}
	for _, c := range loginResp.Cookies() {
		if c.Name == "nats_consol_session" {
			req, _ := http.NewRequest(http.MethodPost, "http://nats-consol.local/api/v1/clusters",
				strings.NewReader(`{"name":"csrf-test","natsUrl":"nats://localhost:4222"}`))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(c)
			resp, err := client.Do(req)
			require.NoError(t, err)
			_ = resp.Body.Close()
			require.Equal(t, http.StatusForbidden, resp.StatusCode, "missing csrf")
			return
		}
	}
	require.Fail(t, "session cookie not set")
}
