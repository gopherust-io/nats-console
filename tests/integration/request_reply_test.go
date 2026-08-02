//go:build integration

package integration_test

import (
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestReplySnapshotEndpoint(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	base := srv.BaseURL(stack.DefaultClusterID(t))

	resp, err := srv.Client.Get(base + "/request-reply")
	require.NoError(t, err)
	respBody := resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "request-reply snapshot: %s", commonstrings.BytesToString(respBody))

	var snap struct {
		Data domain.RequestReplySnapshot `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &snap))
	require.NotNil(t, snap.Data.Patterns)
	require.NotNil(t, snap.Data.Connections)

	resp, err = srv.Client.Get(base + "/request-reply?fresh=1")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "request-reply fresh: %s", commonstrings.BytesToString(respBody))
	require.NoError(t, sonic.Unmarshal(respBody, &snap))
	assert.GreaterOrEqual(t, snap.Data.Requesters, 0)
	assert.GreaterOrEqual(t, snap.Data.Responders, 0)
}
