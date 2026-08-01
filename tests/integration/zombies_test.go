//go:build integration

package integration_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZombiesEmptyStreamFindings(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	base := srv.BaseURL(stack.DefaultClusterID(t))

	createBody := `{"name":"ZOMBIE_EMPTY","subjects":["zombie.empty.>"]}`
	resp, err := srv.Client.Post(base+"/streams", "application/json", strings.NewReader(createBody))
	require.NoError(t, err)
	respBody := resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create stream: %s", string(respBody))

	resp, err = srv.Client.Get(base + "/zombies?fresh=1")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "zombies: %s", string(respBody))

	var snap struct {
		Data domain.ZombieSnapshot `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &snap))
	require.NotNil(t, snap.Data.Findings)

	var emptyStream, unpublished bool
	for _, f := range snap.Data.Findings {
		if f.Stream != "ZOMBIE_EMPTY" {
			continue
		}
		switch f.Kind {
		case domain.ZombieKindEmptyStream:
			emptyStream = true
		case domain.ZombieKindUnpublishedSubject:
			unpublished = true
			assert.Equal(t, "zombie.empty.>", f.Subject)
		}
	}
	assert.True(t, emptyStream, "expected empty_stream finding: %s", string(respBody))
	assert.True(t, unpublished, "expected unpublished_subject finding: %s", string(respBody))
	assert.GreaterOrEqual(t, snap.Data.Totals.EmptyStreams, 1)
}
