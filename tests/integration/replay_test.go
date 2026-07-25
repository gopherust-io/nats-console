//go:build integration

package integration_test

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplayConsumerResetAndSidecar(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)
	base := srv.BaseURL(clusterID)

	createBody := `{"name":"REPLAY_TEST","subjects":["replay.test"]}`
	resp, err := srv.Client.Post(base+"/streams", "application/json", strings.NewReader(createBody))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(body))

	for i := 0; i < 3; i++ {
		payload := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"n":%d}`, i+1)))
		publishBody := fmt.Sprintf(`{"subject":"replay.test","data":%q}`, payload)
		resp, err = srv.Client.Post(base+"/streams/REPLAY_TEST/messages", "application/json", strings.NewReader(publishBody))
		require.NoError(t, err)
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode, string(body))
	}

	consumerBody := `{"durableName":"replay-worker","deliverPolicy":"new","ackPolicy":"explicit"}`
	resp, err = srv.Client.Post(base+"/streams/REPLAY_TEST/consumers", "application/json", strings.NewReader(consumerBody))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(body))

	resetBody := `{"mode":"reset","from":"seq","seq":1,"replayPolicy":"instant"}`
	resp, err = srv.Client.Post(
		base+"/streams/REPLAY_TEST/consumers/replay-worker/replay",
		"application/json",
		strings.NewReader(resetBody),
	)
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	var resetResult struct {
		Durable string `json:"durable"`
		Mode    string `json:"mode"`
	}
	require.NoError(t, sonic.Unmarshal(body, &resetResult))
	assert.Equal(t, "replay-worker", resetResult.Durable)
	assert.Equal(t, "reset", resetResult.Mode)

	resp, err = srv.Client.Get(base + "/streams/REPLAY_TEST/consumers/replay-worker")
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	var consumer struct {
		Config struct {
			DeliverPolicy string `json:"deliverPolicy"`
		} `json:"config"`
		NumPending uint64 `json:"numPending"`
	}
	require.NoError(t, sonic.Unmarshal(body, &consumer))
	assert.Equal(t, "by_start_sequence", consumer.Config.DeliverPolicy)
	assert.GreaterOrEqual(t, consumer.NumPending, uint64(1))

	sidecarBody := `{"mode":"sidecar","from":"seq","seq":1,"durable":"replay-worker-backfill","replayPolicy":"instant"}`
	resp, err = srv.Client.Post(
		base+"/streams/REPLAY_TEST/consumers/replay-worker/replay",
		"application/json",
		strings.NewReader(sidecarBody),
	)
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	var sidecarResult struct {
		Durable string `json:"durable"`
		Mode    string `json:"mode"`
	}
	require.NoError(t, sonic.Unmarshal(body, &sidecarResult))
	assert.Equal(t, "replay-worker-backfill", sidecarResult.Durable)
	assert.Equal(t, "sidecar", sidecarResult.Mode)

	resp, err = srv.Client.Get(base + "/streams/REPLAY_TEST/consumers/replay-worker-backfill")
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	// Source durable still exists after sidecar create.
	resp, err = srv.Client.Get(base + "/streams/REPLAY_TEST/consumers/replay-worker")
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCreateConsumerWithOptStartSeq(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)
	base := srv.BaseURL(clusterID)

	createBody := `{"name":"STARTSEQ_TEST","subjects":["startseq.test"]}`
	resp, err := srv.Client.Post(base+"/streams", "application/json", strings.NewReader(createBody))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(body))

	payload := base64.StdEncoding.EncodeToString([]byte(`{"ok":true}`))
	publishBody := fmt.Sprintf(`{"subject":"startseq.test","data":%q}`, payload)
	resp, err = srv.Client.Post(base+"/streams/STARTSEQ_TEST/messages", "application/json", strings.NewReader(publishBody))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	consumerBody := `{"durableName":"start-worker","deliverPolicy":"by_start_sequence","optStartSeq":1,"ackPolicy":"explicit","replayPolicy":"instant"}`
	resp, err = srv.Client.Post(base+"/streams/STARTSEQ_TEST/consumers", "application/json", strings.NewReader(consumerBody))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(body))

	var created struct {
		Config struct {
			DeliverPolicy string `json:"deliverPolicy"`
		} `json:"config"`
	}
	require.NoError(t, sonic.Unmarshal(body, &created))
	assert.Equal(t, "by_start_sequence", created.Config.DeliverPolicy)
}
