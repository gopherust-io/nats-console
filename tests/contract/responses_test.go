//go:build integration

package contract_test

import (
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/tests/testutil"
)

// Contract tests ensure API JSON responses use camelCase keys expected by web/src/lib/api.ts.

func TestClustersListContract(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Get("http://nats-consol.local/api/v1/clusters")
	require.NoError(t, err)
	body := resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "status body = %s", strings.BytesToString(body))

	testutil.AssertCamelCaseKeys(t, body)
	testutil.AssertHasKeys(t, body, "data", "meta")
	testutil.AssertJSONArrayNotNull(t, body, "data")
	testutil.AssertNoKeys(t, body, "clusters", "token", "credsFilePath", "password_hash", "nats_url")

	var list struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			NatsURL       string `json:"natsUrl"`
			MonitoringURL string `json:"monitoringUrl"`
			HasCreds      bool   `json:"hasCreds"`
			HasToken      bool   `json:"hasToken"`
			IsDefault     bool   `json:"isDefault"`
		} `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, sonic.Unmarshal(body, &list))
	require.GreaterOrEqual(t, list.Meta.Total, 1, "expected at least one cluster")
	require.NotEmpty(t, list.Data, "expected at least one cluster")
	c := list.Data[0]
	assert.NotEmpty(t, c.ID, "cluster fields missing: %+v", c)
	assert.NotEmpty(t, c.NatsURL, "cluster fields missing: %+v", c)
}

func TestAuthConfigContract(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Get("http://nats-consol.local/api/v1/auth/config")
	require.NoError(t, err)
	body := resp.Body

	testutil.AssertCamelCaseKeys(t, body)
	testutil.AssertHasKeys(t, body, "data")
	var env struct {
		Data struct {
			BasicEnabled bool `json:"basicEnabled"`
			AuthEnabled  bool `json:"authEnabled"`
			AIEnabled    bool `json:"aiEnabled"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(body, &env))
	assert.True(t, env.Data.AuthEnabled)
}

func TestHealthContract(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Get("http://nats-consol.local/api/health")
	require.NoError(t, err)
	body := resp.Body

	testutil.AssertCamelCaseKeys(t, body)
	testutil.AssertHasKeys(t, body, "data")
}

func TestStreamsListContract(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)

	resp, err := srv.Client.Get(srv.BaseURL(clusterID) + "/streams")
	require.NoError(t, err)
	body := resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "status")

	testutil.AssertCamelCaseKeys(t, body)
	testutil.AssertHasKeys(t, body, "data", "meta")
	testutil.AssertJSONArrayNotNull(t, body, "data")
	testutil.AssertNoKeys(t, body, "streams")
	var env struct {
		Meta struct {
			Total  int `json:"total"`
			Offset int `json:"offset"`
			Limit  int `json:"limit"`
		} `json:"meta"`
	}
	require.NoError(t, sonic.Unmarshal(body, &env))
	assert.GreaterOrEqual(t, env.Meta.Total, 0)
	assert.GreaterOrEqual(t, env.Meta.Limit, 1)
}

func TestConnectionsListContract(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Get("http://nats-consol.local/api/v1/clusters/connections")
	require.NoError(t, err)
	body := resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "status body = %s", strings.BytesToString(body))

	testutil.AssertCamelCaseKeys(t, body)
	testutil.AssertHasKeys(t, body, "data", "meta")
	testutil.AssertJSONArrayNotNull(t, body, "data")
	testutil.AssertNoKeys(t, body, "connections")

	var list struct {
		Data []struct {
			ClusterID string `json:"clusterId"`
			Connected bool   `json:"connected"`
		} `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, sonic.Unmarshal(body, &list))
	assert.GreaterOrEqual(t, list.Meta.Total, 0)
}

func TestUnauthorizedErrorEnvelopeContract(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	resp, err := srv.UnauthClient.Get("http://nats-consol.local/api/v1/clusters")
	require.NoError(t, err)
	body := resp.Body
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode, strings.BytesToString(body))

	testutil.AssertCamelCaseKeys(t, body)
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, sonic.Unmarshal(body, &envelope))
	assert.Equal(t, "unauthorized", envelope.Error.Code)
	assert.NotEmpty(t, envelope.Error.Message)
}

func TestNotFoundErrorEnvelopeContract(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Get("http://nats-consol.local/api/v1/clusters/01900000-0000-7000-8000-000000000001")
	require.NoError(t, err)
	body := resp.Body
	require.Equal(t, http.StatusNotFound, resp.StatusCode, strings.BytesToString(body))

	testutil.AssertCamelCaseKeys(t, body)
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, sonic.Unmarshal(body, &envelope))
	assert.Equal(t, "not_found", envelope.Error.Code)
	assert.NotEmpty(t, envelope.Error.Message)
}
