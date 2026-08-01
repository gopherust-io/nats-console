//go:build integration

package integration_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/require"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/tests/testutil"
)

func TestClusterStreamConsumerLifecycle(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)
	base := srv.BaseURL(clusterID)

	createBody := `{"name":"ORDERS","subjects":["orders.>"]}`
	resp, err := srv.Client.Post(base+"/streams", "application/json", strings.NewReader(createBody))
	require.NoError(t, err)
	respBody := resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create stream: %s", commonstrings.BytesToString(respBody))

	consumerBody := `{"durableName":"orders-worker","deliverPolicy":"all","ackPolicy":"explicit"}`
	resp, err = srv.Client.Post(base+"/streams/ORDERS/consumers", "application/json", strings.NewReader(consumerBody))
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create consumer: %s", commonstrings.BytesToString(respBody))

	resp, err = srv.Client.Get(base + "/streams/ORDERS/consumers")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "list consumers")

	var consumerList struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &consumerList))
	require.Equal(t, 1, consumerList.Meta.Total, "consumer total")

	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodDelete,
		URL:    base+"/streams/ORDERS/consumers/orders-worker",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "delete consumer")
}

func TestHealthEndpoint(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Get("http://nats-consol.local/api/health")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "health status")
}
