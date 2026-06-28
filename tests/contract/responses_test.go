//go:build integration

package contract_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Contract tests ensure API JSON responses use camelCase keys expected by web/src/lib/api.ts.

func TestClustersListContract(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Get("http://nats-consol.local/api/v1/clusters")
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "status body = %s", string(body))

	testutil.AssertCamelCaseKeys(t, body)
	testutil.AssertHasKeys(t, body, "clusters", "total")
	testutil.AssertJSONArrayNotNull(t, body, "clusters")
	testutil.AssertNoKeys(t, body, "token", "credsFilePath", "password_hash", "nats_url")

	var list struct {
		Clusters []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			NatsURL       string `json:"natsUrl"`
			MonitoringURL string `json:"monitoringUrl"`
			HasCreds      bool   `json:"hasCreds"`
			HasToken      bool   `json:"hasToken"`
			IsDefault     bool   `json:"isDefault"`
		} `json:"clusters"`
		Total int `json:"total"`
	}
	require.NoError(t, sonic.Unmarshal(body, &list))
	require.GreaterOrEqual(t, list.Total, 1, "expected at least one cluster")
	require.NotEmpty(t, list.Clusters, "expected at least one cluster")
	c := list.Clusters[0]
	assert.NotEmpty(t, c.ID, "cluster fields missing: %+v", c)
	assert.NotEmpty(t, c.NatsURL, "cluster fields missing: %+v", c)
}

func TestAuthConfigContract(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, func(cfg *config.Config) {
		cfg.AuthEnabled = true
	})

	resp, err := srv.Client.Get("http://nats-consol.local/api/v1/auth/config")
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	testutil.AssertCamelCaseKeys(t, body)
	testutil.AssertHasKeys(t, body, "oidcEnabled", "basicEnabled", "authEnabled", "oidcProviders", "aiEnabled")
}

func TestHealthContract(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Get("http://nats-consol.local/api/health")
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	testutil.AssertCamelCaseKeys(t, body)
}

func TestStreamsListContract(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)

	resp, err := srv.Client.Get(srv.BaseURL(clusterID) + "/streams")
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "status")

	testutil.AssertCamelCaseKeys(t, body)
	testutil.AssertHasKeys(t, body, "streams", "total", "offset", "limit")
}

func TestConnectionsListContract(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)

	resp, err := srv.Client.Get("http://nats-consol.local/api/v1/clusters/connections")
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "status body = %s", string(body))

	testutil.AssertCamelCaseKeys(t, body)
	testutil.AssertHasKeys(t, body, "connections", "total")
	testutil.AssertJSONArrayNotNull(t, body, "connections")

	var list struct {
		Connections []struct {
			ClusterID string `json:"clusterId"`
			Connected bool   `json:"connected"`
		} `json:"connections"`
		Total int `json:"total"`
	}
	require.NoError(t, sonic.Unmarshal(body, &list))
	assert.GreaterOrEqual(t, list.Total, 0)
}

func TestSuperclusterContract(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)

	resp, err := srv.Client.Get(srv.BaseURL(clusterID) + "/supercluster")
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "status body = %s", string(body))

	testutil.AssertCamelCaseKeys(t, body)
	testutil.AssertHasKeys(t, body, "serverName", "fetchedAt", "gateways", "routes", "leafnodes", "streamReplication")

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &payload))
	for _, key := range []string{"gateways", "routes", "leafnodes", "streamReplication"} {
		assert.NotEqual(t, "null", string(payload[key]), "%s should be an array, not null", key)
	}
	for _, key := range []string{"sourceErrors", "warnings"} {
		raw, ok := payload[key]
		if !ok {
			continue
		}
		assert.NotEqual(t, "null", string(raw), "%s should not be null when present", key)
		switch key {
		case "sourceErrors":
			var errs map[string]string
			require.NoError(t, json.Unmarshal(raw, &errs))
		case "warnings":
			var warns []string
			require.NoError(t, json.Unmarshal(raw, &warns))
		}
	}
}
