//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/require"
)

func sampleAccountJWT(t *testing.T) string {
	t.Helper()
	kp, err := nkeys.CreateAccount()
	require.NoError(t, err)
	pub, err := kp.PublicKey()
	require.NoError(t, err)
	claims := jwt.NewAccountClaims(pub)
	claims.Name = "RESOLVER_TEST"
	claims.Subject = pub
	token, err := claims.Encode(kp)
	require.NoError(t, err)
	return token
}

func TestJWTResolverImportListDelete(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)
	base := srv.BaseURL(clusterID)

	token := sampleAccountJWT(t)
	body, _ := json.Marshal(map[string]string{"jwt": token})
	resp, err := srv.Client.Post(base+"/resolver/accounts", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(respBody))

	resp, err = srv.Client.Get(base + "/resolver/accounts")
	require.NoError(t, err)
	respBody, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(respBody), "RESOLVER_TEST")

	req, _ := http.NewRequest(http.MethodDelete, base+"/resolver/accounts/RESOLVER_TEST", nil)
	resp, err = srv.Client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestRotateEncryptionKeyDryRun(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	body := `{"currentKey":"test-session-secret-key","newKey":"another-long-secret-key"}`
	resp, err := srv.Client.Post("http://nats-consol.local/api/v1/admin/rotate-encryption-key?dryRun=true", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}
