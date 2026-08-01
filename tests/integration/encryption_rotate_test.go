//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/require"
)

func TestRotateEncryptionKeyDryRun(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	body := `{"currentKey":"test-session-secret-key","newKey":"another-long-secret-key"}`

	unauth, err := srv.UnauthClient.Post("http://nats-consol.local/api/v1/admin/rotate-encryption-key?dryRun=true", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, unauth.StatusCode)

	resp, err := srv.Client.Post("http://nats-consol.local/api/v1/admin/rotate-encryption-key?dryRun=true", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	raw := resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))

	var envelope struct {
		Data struct {
			DryRun          bool `json:"dryRun"`
			ClustersUpdated int  `json:"clustersUpdated"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &envelope))
	require.True(t, envelope.Data.DryRun)
}
