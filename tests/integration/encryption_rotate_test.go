//go:build integration

package integration_test

import (
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
	resp, err := srv.Client.Post("http://nats-consol.local/api/v1/admin/rotate-encryption-key?dryRun=true", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}
