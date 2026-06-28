//go:build integration

package integration_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/require"
)

func TestClusterStreamConsumerLifecycle(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)
	base := srv.BaseURL(clusterID)

	createBody := `{"name":"ORDERS","subjects":["orders.>"]}`
	resp, err := srv.Client.Post(base+"/streams", "application/json", strings.NewReader(createBody))
	require.NoError(t, err)
	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create stream: %s", string(respBody))

	consumerBody := `{"durableName":"orders-worker","deliverPolicy":"all","ackPolicy":"explicit"}`
	resp, err = srv.Client.Post(base+"/streams/ORDERS/consumers", "application/json", strings.NewReader(consumerBody))
	require.NoError(t, err)
	respBody, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create consumer: %s", string(respBody))

	resp, err = srv.Client.Get(base + "/streams/ORDERS/consumers")
	require.NoError(t, err)
	respBody, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "list consumers")

	var consumerList struct {
		Total int `json:"total"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &consumerList))
	require.Equal(t, 1, consumerList.Total, "consumer total")

	req, _ := http.NewRequest(http.MethodDelete, base+"/streams/ORDERS/consumers/orders-worker", nil)
	resp, err = srv.Client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "delete consumer")
}

func TestHealthEndpoint(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Get("http://nats-consol.local/api/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode, "health status")
}
