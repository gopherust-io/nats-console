//go:build integration

package integration_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gopherust-io/nats/dlq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/tests/testutil"
)

func TestDLQListAndRetry(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)
	base := srv.BaseURL(clusterID)

	workBody := `{"name":"ORDERS","subjects":["orders.work.>"]}`
	resp, err := srv.Client.Post(base+"/streams", "application/json", strings.NewReader(workBody))
	require.NoError(t, err)
	body := resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, commonstrings.BytesToString(body))

	dlqBody := `{"name":"ORDERS_DLQ","subjects":["orders.dlq.>"]}`
	resp, err = srv.Client.Post(base+"/streams", "application/json", strings.NewReader(dlqBody))
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, commonstrings.BytesToString(body))

	var streamInfo struct {
		Data struct {
			IsDLQ  bool `json:"isDlq"`
			Config struct {
				Name string `json:"name"`
			} `json:"config"`
		} `json:"data"`
	}
	resp, err = srv.Client.Get(base + "/streams/ORDERS_DLQ")
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, commonstrings.BytesToString(body))
	require.NoError(t, sonic.Unmarshal(body, &streamInfo))
	assert.True(t, streamInfo.Data.IsDLQ)

	payload := base64.StdEncoding.EncodeToString(commonstrings.StringToBytes(`{"order":1}`))
	publishDLQ := fmt.Sprintf(
		`{"subject":"orders.dlq.poison","data":%q,"headers":{%q:%q,%q:%q,%q:%q,%q:%q,%q:%q,%q:%q}}`,
		payload,
		dlq.HeaderOriginalSubject, "orders.work.created",
		dlq.HeaderReason, "handler_requested",
		dlq.HeaderStream, "ORDERS",
		dlq.HeaderSequence, "9",
		dlq.HeaderConsumer, "worker",
		dlq.HeaderAutopsyError, "boom",
	)
	resp, err = srv.Client.Post(base+"/streams/ORDERS_DLQ/messages", "application/json", strings.NewReader(publishDLQ))
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, commonstrings.BytesToString(body))

	// Second poison message for batch retry
	payload2 := base64.StdEncoding.EncodeToString(commonstrings.StringToBytes(`{"order":2}`))
	publishDLQ2 := fmt.Sprintf(
		`{"subject":"orders.dlq.poison","data":%q,"headers":{%q:%q,%q:%q}}`,
		payload2,
		dlq.HeaderOriginalSubject, "orders.work.created",
		dlq.HeaderReason, "max_deliver",
	)
	resp, err = srv.Client.Post(base+"/streams/ORDERS_DLQ/messages", "application/json", strings.NewReader(publishDLQ2))
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, commonstrings.BytesToString(body))

	resp, err = srv.Client.Get(base + "/streams/ORDERS_DLQ/dlq/messages?limit=50")
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, commonstrings.BytesToString(body))

	var listed struct {
		Data struct {
			Messages []struct {
				Seq             uint64 `json:"seq"`
				OriginalSubject string `json:"originalSubject"`
				Reason          string `json:"reason"`
				AutopsyError    string `json:"autopsyError"`
			} `json:"messages"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(body, &listed))
	require.Len(t, listed.Data.Messages, 2)
	assert.Equal(t, "orders.work.created", listed.Data.Messages[0].OriginalSubject)
	assert.Equal(t, "handler_requested", listed.Data.Messages[0].Reason)
	assert.Equal(t, "boom", listed.Data.Messages[0].AutopsyError)

	firstSeq := listed.Data.Messages[0].Seq
	retryBody := fmt.Sprintf(`{"seqs":[%d]}`, firstSeq)
	resp, err = srv.Client.Post(base+"/streams/ORDERS_DLQ/dlq/retry", "application/json", strings.NewReader(retryBody))
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, commonstrings.BytesToString(body))

	var retryResult struct {
		Data struct {
			Retried int `json:"retried"`
			Failed  []struct {
				Seq   uint64 `json:"seq"`
				Error string `json:"error"`
			} `json:"failed"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(body, &retryResult))
	assert.Equal(t, 1, retryResult.Data.Retried)
	assert.Empty(t, retryResult.Data.Failed)

	resp, err = srv.Client.Get(base + "/streams/ORDERS/messages?seq=1")
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, commonstrings.BytesToString(body))
	assert.Contains(t, commonstrings.BytesToString(body), "orders.work.created")

	resp, err = srv.Client.Get(base + "/streams/ORDERS_DLQ/dlq/messages?limit=50")
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, commonstrings.BytesToString(body))
	require.NoError(t, sonic.Unmarshal(body, &listed))
	require.Len(t, listed.Data.Messages, 1)

	resp, err = srv.Client.Post(base+"/streams/ORDERS_DLQ/dlq/retry", "application/json", strings.NewReader(`{"all":true}`))
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, commonstrings.BytesToString(body))
	require.NoError(t, sonic.Unmarshal(body, &retryResult))
	assert.Equal(t, 1, retryResult.Data.Retried)

	resp, err = srv.Client.Get(base + "/streams/ORDERS_DLQ/dlq/messages?limit=50")
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, commonstrings.BytesToString(body))
	require.NoError(t, sonic.Unmarshal(body, &listed))
	assert.Empty(t, listed.Data.Messages)

	// Non-DLQ stream rejected
	resp, err = srv.Client.Get(base + "/streams/ORDERS/dlq/messages")
	require.NoError(t, err)
	body = resp.Body
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, commonstrings.BytesToString(body))
}
