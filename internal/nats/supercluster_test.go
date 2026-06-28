package natsclient

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyVarz(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"server_name": "n1",
		"cluster": {"name": "east"},
		"gateway": {"name": "east-gw"},
		"routes": 2,
		"leafnodes": 1
	}`)
	var out domain.SuperclusterOverview
	applyVarz(raw, &out)

	require.Equal(t, "n1", out.ServerName)
	assert.Equal(t, "east", out.ClusterName)
	assert.True(t, out.GatewayEnabled)
	assert.Equal(t, 2, out.RouteCount)
	assert.Equal(t, 1, out.LeafCount)
}

func TestParseGateways(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"outbound_gateways": [{"name": "west", "host": "10.0.0.1", "port": 7222, "connections": 1}],
		"inbound_gateways": [{"name": "central", "host": "10.0.0.2", "port": 7222, "connections": 2}]
	}`)
	gws := parseGateways(raw)
	require.Len(t, gws, 2)
	assert.Equal(t, "outbound", gws[0].Direction)
	assert.Equal(t, "inbound", gws[1].Direction)
}

func TestReplicationFromStream(t *testing.T) {
	t.Parallel()

	streams := jszStreamDetail{
		Name: "ORDERS",
		Mirror: &jszSourceInfo{
			Name:   "ORDERS",
			Domain: "west",
			Active: true,
			Lag:    3,
		},
	}
	links := replicationFromStream(streams)
	require.Len(t, links, 1)
	assert.Equal(t, "mirror", links[0].Kind)
	assert.Equal(t, "west", links[0].TargetDomain)
}

func TestApplyJSZ(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"domain":"east",
		"meta_cluster":{"leader":"n1","cluster_size":3,"replicas":[{"name":"n1","leader":true,"current":true}]},
		"account_details":[{"stream_detail":[{"name":"S","mirror":{"name":"S","domain":"west","active":true}}]}]
	}`)
	var out domain.SuperclusterOverview
	applyJSZ(raw, &out)
	assert.Equal(t, "east", out.JetStreamDomain)
	require.NotNil(t, out.MetaCluster)
	assert.Equal(t, "n1", out.MetaCluster.Leader)
	assert.Len(t, out.StreamReplication, 1)
}

func TestNormalizeSuperclusterOverviewUsesEmptySlices(t *testing.T) {
	t.Parallel()

	var out domain.SuperclusterOverview
	normalizeSuperclusterOverview(&out)
	assert.NotNil(t, out.Gateways)
	assert.NotNil(t, out.Routes)
	assert.NotNil(t, out.Leafnodes)
	assert.NotNil(t, out.StreamReplication)
	assert.Len(t, out.Gateways, 0)
}
