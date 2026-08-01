//go:build integration

package integration_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/tests/testutil"
)

func TestReplayConsumerResetAndSidecar(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)
	base := srv.BaseURL(clusterID)

	createBody := `{"name":"REPLAY_TEST","subjects":["replay.test"]}`
	resp, err := srv.Client.Post(base+"/streams", "application/json", strings.NewReader(createBody))
	require.NoError(t, err)
	body := resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, commonstrings.BytesToString(body))

	for i := 0; i < 3; i++ {
		payload := base64.StdEncoding.EncodeToString(commonstrings.StringToBytes(fmt.Sprintf(`{"n":%d}`, i+1)))
		publishBody := fmt.Sprintf(`{"subject":"replay.test","data":%q}`, payload)
		resp, err = srv.Client.Post(base+"/streams/REPLAY_TEST/messages", "application/json", strings.NewReader(publishBody))
		require.NoError(t, err)
		body = resp.Body
			require.Equal(t, http.StatusCreated, resp.StatusCode, commonstrings.BytesToString(body))
	}

	consumerBody := `{"durableName":"replay-worker","deliverPolicy":"new","ackPolicy":"explicit"}`
	resp, err = srv.Client.Post(base+"/streams/REPLAY_TEST/consumers", "application/json", strings.NewReader(consumerBody))
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, commonstrings.BytesToString(body))

	resetBody := `{"mode":"reset","from":"seq","seq":1,"replayPolicy":"instant"}`
	resp, err = srv.Client.Post(
		base+"/streams/REPLAY_TEST/consumers/replay-worker/replay",
		"application/json",
		strings.NewReader(resetBody),
	)
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, commonstrings.BytesToString(body))

	var resetResult struct {
		Data struct {
			Durable  string `json:"durable"`
			Mode     string `json:"mode"`
			StartSeq uint64 `json:"startSeq"`
			UntilSeq uint64 `json:"untilSeq"`
			Limit    int    `json:"limit"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(body, &resetResult))
	assert.Equal(t, "replay-worker", resetResult.Data.Durable)
	assert.Equal(t, "reset", resetResult.Data.Mode)
	assert.Equal(t, uint64(1), resetResult.Data.StartSeq)

	oneBody := `{"mode":"reset","from":"seq","seq":2,"untilSeq":2,"limit":1,"replayPolicy":"instant"}`
	resp, err = srv.Client.Post(
		base+"/streams/REPLAY_TEST/consumers/replay-worker/replay",
		"application/json",
		strings.NewReader(oneBody),
	)
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, commonstrings.BytesToString(body))
	require.NoError(t, sonic.Unmarshal(body, &resetResult))
	assert.Equal(t, uint64(2), resetResult.Data.StartSeq)
	assert.Equal(t, uint64(2), resetResult.Data.UntilSeq)
	assert.Equal(t, 1, resetResult.Data.Limit)

	resp, err = srv.Client.Get(base + "/streams/REPLAY_TEST/messages/range?startSeq=1&endSeq=2")
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, commonstrings.BytesToString(body))
	var rangeResult struct {
		Data struct {
			Messages []struct {
				Seq uint64 `json:"seq"`
			} `json:"messages"`
			Truncated bool `json:"truncated"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(body, &rangeResult))
	require.Len(t, rangeResult.Data.Messages, 2)
	assert.False(t, rangeResult.Data.Truncated)

	resp, err = srv.Client.Get(base + "/streams/REPLAY_TEST/consumers/replay-worker")
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, commonstrings.BytesToString(body))

	var consumer struct {
		Data struct {
			Config struct {
				DeliverPolicy string `json:"deliverPolicy"`
			} `json:"config"`
			NumPending uint64 `json:"numPending"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(body, &consumer))
	assert.Equal(t, "by_start_sequence", consumer.Data.Config.DeliverPolicy)
	assert.GreaterOrEqual(t, consumer.Data.NumPending, uint64(1))

	sidecarBody := `{"mode":"sidecar","from":"seq","seq":1,"untilSeq":3,"limit":3,"durable":"replay-worker-backfill","replayPolicy":"instant"}`
	resp, err = srv.Client.Post(
		base+"/streams/REPLAY_TEST/consumers/replay-worker/replay",
		"application/json",
		strings.NewReader(sidecarBody),
	)
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, commonstrings.BytesToString(body))

	var sidecarResult struct {
		Data struct {
			Durable string `json:"durable"`
			Mode    string `json:"mode"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(body, &sidecarResult))
	assert.Equal(t, "replay-worker-backfill", sidecarResult.Data.Durable)
	assert.Equal(t, "sidecar", sidecarResult.Data.Mode)

	resp, err = srv.Client.Get(base + "/streams/REPLAY_TEST/consumers/replay-worker-backfill")
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, commonstrings.BytesToString(body))

	// Source durable still exists after sidecar create.
	resp, err = srv.Client.Get(base + "/streams/REPLAY_TEST/consumers/replay-worker")
	require.NoError(t, err)
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
	body := resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, commonstrings.BytesToString(body))

	payload := base64.StdEncoding.EncodeToString(commonstrings.StringToBytes(`{"ok":true}`))
	publishBody := fmt.Sprintf(`{"subject":"startseq.test","data":%q}`, payload)
	resp, err = srv.Client.Post(base+"/streams/STARTSEQ_TEST/messages", "application/json", strings.NewReader(publishBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	consumerBody := `{"durableName":"start-worker","deliverPolicy":"by_start_sequence","optStartSeq":1,"ackPolicy":"explicit","replayPolicy":"instant"}`
	resp, err = srv.Client.Post(base+"/streams/STARTSEQ_TEST/consumers", "application/json", strings.NewReader(consumerBody))
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, commonstrings.BytesToString(body))

	var created struct {
		Data struct {
			Config struct {
				DeliverPolicy string `json:"deliverPolicy"`
			} `json:"config"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(body, &created))
	assert.Equal(t, "by_start_sequence", created.Data.Config.DeliverPolicy)
}
