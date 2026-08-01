package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestBuildReplicasSnapshotFivePeers(t *testing.T) {
	t.Parallel()

	varz := commonstrings.StringToBytes(`{
		"server_name": "nats-1",
		"version": "2.14.0",
		"uptime": "1h2m",
		"connections": 4,
		"cpu": 1.5,
		"mem": 1048576,
		"cluster": {"name": "c1"}
	}`)
	routez := commonstrings.StringToBytes(`{
		"server_name": "nats-1",
		"routes": [
			{"remote_name": "nats-2", "ip": "10.0.0.2", "uptime": "1h", "idle": "1s", "rtt": "120µs", "pending_size": 0, "in_msgs": 10, "out_msgs": 12},
			{"remote_name": "nats-3", "ip": "10.0.0.3", "uptime": "1h", "idle": "2s", "rtt": "130µs", "pending_size": 1, "in_msgs": 20, "out_msgs": 22},
			{"remote_name": "nats-4", "ip": "10.0.0.4", "uptime": "50m", "idle": "3s", "rtt": "140µs", "pending_size": 0, "in_msgs": 30, "out_msgs": 32},
			{"name": "nats-5", "ip": "10.0.0.5", "uptime": "40m", "idle": "4s", "rtt": "150µs", "pending_size": 0, "in_msgs": 40, "out_msgs": 42}
		]
	}`)
	jsz := commonstrings.StringToBytes(`{
		"server_name": "nats-1",
		"meta_cluster": {
			"name": "hub",
			"leader": "nats-2",
			"cluster_size": 5,
			"replicas": [
				{"name": "nats-2", "current": true, "offline": false},
				{"name": "nats-3", "current": true, "offline": false},
				{"name": "nats-4", "current": true, "offline": false},
				{"name": "nats-5", "current": false, "offline": true}
			]
		}
	}`)

	snap, err := buildReplicasSnapshot([][]byte{varz}, routez, jsz)
	require.NoError(t, err)
	assert.Equal(t, "c1", snap.ClusterName)
	assert.Equal(t, "nats-1", snap.MonitoredServer)
	assert.Equal(t, "nats-2", snap.JetStreamLeader)
	assert.Equal(t, 5, snap.ClusterSize)
	assert.Equal(t, 5, snap.PeerCount)
	assert.Equal(t, 4, snap.OnlineCount)
	require.Len(t, snap.Peers, 5)

	byName := map[string]replicaPeer{}
	for _, p := range snap.Peers {
		byName[p.Name] = p
	}
	assert.Equal(t, "monitored", byName["nats-1"].Role)
	assert.False(t, byName["nats-1"].Leader)
	assert.True(t, byName["nats-1"].Online)
	assert.Equal(t, "2.14.0", byName["nats-1"].Version)
	require.NotNil(t, byName["nats-1"].Connections)
	assert.Equal(t, 4, *byName["nats-1"].Connections)

	assert.Equal(t, "route", byName["nats-2"].Role)
	assert.True(t, byName["nats-2"].Leader)
	assert.True(t, byName["nats-2"].Current)
	assert.True(t, byName["nats-2"].Online)
	assert.Equal(t, "10.0.0.2", byName["nats-2"].IP)

	assert.Equal(t, "route", byName["nats-5"].Role)
	assert.False(t, byName["nats-5"].Online)
	assert.False(t, byName["nats-5"].Current)

	// Leader sorts first, then monitored, then name.
	assert.Equal(t, "nats-2", snap.Peers[0].Name)
	assert.Equal(t, "nats-1", snap.Peers[1].Name)
}

func TestBuildReplicasSnapshotMultiVarzMergesPeerMetrics(t *testing.T) {
	t.Parallel()

	varz1 := commonstrings.StringToBytes(`{
		"server_name": "nats-1",
		"version": "2.14.0",
		"uptime": "1h",
		"connections": 2,
		"cpu": 1.0,
		"mem": 1000,
		"cluster": {"name": "c1"}
	}`)
	varz2 := commonstrings.StringToBytes(`{
		"server_name": "nats-2",
		"version": "2.14.1",
		"uptime": "50m",
		"connections": 7,
		"cpu": 3.5,
		"mem": 2000
	}`)
	varz3 := commonstrings.StringToBytes(`{
		"server_name": "nats-3",
		"version": "2.14.0",
		"uptime": "40m",
		"connections": 1,
		"cpu": 0.5,
		"mem": 1500
	}`)
	routez := commonstrings.StringToBytes(`{
		"server_name": "nats-1",
		"routes": [
			{"remote_name": "nats-2", "rtt": "100µs", "in_msgs": 5, "out_msgs": 6},
			{"remote_name": "nats-3", "rtt": "110µs", "in_msgs": 1, "out_msgs": 1}
		]
	}`)
	jsz := commonstrings.StringToBytes(`{
		"server_name": "nats-1",
		"meta_cluster": {
			"name": "hub",
			"leader": "nats-2",
			"cluster_size": 3
		}
	}`)

	snap, err := buildReplicasSnapshot([][]byte{varz1, varz2, varz3}, routez, jsz)
	require.NoError(t, err)
	assert.Equal(t, 3, snap.ClusterSize)
	assert.Equal(t, 3, snap.OnlineCount)
	assert.Equal(t, "nats-1", snap.MonitoredServer)
	assert.Equal(t, "nats-2", snap.JetStreamLeader)

	byName := map[string]replicaPeer{}
	for _, p := range snap.Peers {
		byName[p.Name] = p
	}

	assert.Equal(t, "monitored", byName["nats-1"].Role)
	require.NotNil(t, byName["nats-1"].CPU)
	assert.Equal(t, 1.0, *byName["nats-1"].CPU)

	assert.Equal(t, "route", byName["nats-2"].Role)
	assert.True(t, byName["nats-2"].Leader)
	require.NotNil(t, byName["nats-2"].CPU)
	assert.Equal(t, 3.5, *byName["nats-2"].CPU)
	require.NotNil(t, byName["nats-2"].Mem)
	assert.Equal(t, int64(2000), *byName["nats-2"].Mem)
	require.NotNil(t, byName["nats-2"].Connections)
	assert.Equal(t, 7, *byName["nats-2"].Connections)
	assert.Equal(t, "2.14.1", byName["nats-2"].Version)
	assert.Equal(t, "100µs", byName["nats-2"].RTT)

	require.NotNil(t, byName["nats-3"].CPU)
	assert.Equal(t, 0.5, *byName["nats-3"].CPU)
	assert.Equal(t, "nats-2", snap.Peers[0].Name)
}

func TestBuildReplicasSnapshotMetaOnlyOfflinePeer(t *testing.T) {
	t.Parallel()

	varz := commonstrings.StringToBytes(`{"server_name":"n1","cluster":{"name":"c1"},"connections":1}`)
	routez := commonstrings.StringToBytes(`{"server_name":"n1","routes":[{"remote_name":"n2"}]}`)
	jsz := commonstrings.StringToBytes(`{
		"server_name": "n1",
		"meta_cluster": {
			"name": "hub",
			"leader": "n1",
			"cluster_size": 3,
			"replicas": [
				{"name": "n2", "current": true},
				{"name": "n3", "current": false, "offline": true}
			]
		}
	}`)

	snap, err := buildReplicasSnapshot([][]byte{varz}, routez, jsz)
	require.NoError(t, err)
	assert.Equal(t, 3, snap.PeerCount)
	assert.Equal(t, 2, snap.OnlineCount)

	byName := map[string]replicaPeer{}
	for _, p := range snap.Peers {
		byName[p.Name] = p
	}
	assert.Equal(t, "meta", byName["n3"].Role)
	assert.False(t, byName["n3"].Online)
}

func TestBuildReplicasSnapshotLiveVarzBeatsMetaOffline(t *testing.T) {
	t.Parallel()

	varz1 := commonstrings.StringToBytes(`{"server_name":"n1","connections":1,"cpu":1,"mem":1}`)
	varz2 := commonstrings.StringToBytes(`{"server_name":"n2","connections":2,"cpu":2,"mem":2,"version":"2.14.0"}`)
	routez := commonstrings.StringToBytes(`{"server_name":"n1","routes":[{"remote_name":"n2"}]}`)
	jsz := commonstrings.StringToBytes(`{
		"server_name": "n1",
		"meta_cluster": {
			"leader": "n1",
			"cluster_size": 2,
			"replicas": [{"name": "n2", "offline": true}]
		}
	}`)

	snap, err := buildReplicasSnapshot([][]byte{varz1, varz2}, routez, jsz)
	require.NoError(t, err)
	byName := map[string]replicaPeer{}
	for _, p := range snap.Peers {
		byName[p.Name] = p
	}
	assert.True(t, byName["n2"].Online)
	require.NotNil(t, byName["n2"].CPU)
	assert.Equal(t, 2.0, *byName["n2"].CPU)
}

func TestReplicaPeerCurrentFalseSurvivesJSON(t *testing.T) {
	t.Parallel()

	raw, err := serializer.Marshal(replicaPeer{
		Name:    "nats-5",
		Role:    "route",
		Online:  true,
		Current: false,
	})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"current":false`)

	var got replicaPeer
	require.NoError(t, serializer.Unmarshal(raw, &got))
	assert.False(t, got.Current)
	assert.True(t, got.Online)
}
