package insights

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/snapshot"
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
				{"name": "nats-5", "current": false, "offline": true, "lag": 12}
			]
		}
	}`)

	snap, err := buildReplicasSnapshot([][]byte{varz}, routez, jsz, 1)
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
	require.NotNil(t, byName["nats-2"].Current)
	assert.True(t, *byName["nats-2"].Current)
	assert.True(t, byName["nats-2"].Online)
	assert.Equal(t, "10.0.0.2", byName["nats-2"].IP)

	assert.Equal(t, "route", byName["nats-5"].Role)
	assert.False(t, byName["nats-5"].Online)
	require.NotNil(t, byName["nats-5"].Current)
	assert.False(t, *byName["nats-5"].Current)
	// Meta-offline must not wipe routez link metrics.
	assert.Equal(t, "150µs", byName["nats-5"].RTT)
	assert.Equal(t, "4s", byName["nats-5"].Idle)
	require.NotNil(t, byName["nats-5"].InMsgs)
	assert.Equal(t, int64(40), *byName["nats-5"].InMsgs)
	require.NotNil(t, byName["nats-5"].OutMsgs)
	assert.Equal(t, int64(42), *byName["nats-5"].OutMsgs)
	require.NotNil(t, byName["nats-5"].Lag)
	assert.Equal(t, int64(12), *byName["nats-5"].Lag)
	// Monitored peer not listed in meta.replicas — current stays unknown (nil).
	assert.Nil(t, byName["nats-1"].Current)

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

	snap, err := buildReplicasSnapshot([][]byte{varz1, varz2, varz3}, routez, jsz, 5)
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

	snap, err := buildReplicasSnapshot([][]byte{varz}, routez, jsz, 1)
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

func TestBuildReplicasSnapshotMetaOfflineWinsOverVarz(t *testing.T) {
	t.Parallel()

	varz1 := commonstrings.StringToBytes(`{"server_name":"n1","connections":1,"cpu":1,"mem":1}`)
	varz2 := commonstrings.StringToBytes(`{"server_name":"n2","connections":2,"cpu":2,"mem":2,"version":"2.14.0"}`)
	routez := commonstrings.StringToBytes(`{"server_name":"n1","routes":[{"remote_name":"n2","rtt":"90µs","in_msgs":9,"out_msgs":8}]}`)
	jsz := commonstrings.StringToBytes(`{
		"server_name": "n1",
		"meta_cluster": {
			"leader": "n1",
			"cluster_size": 2,
			"replicas": [{"name": "n2", "offline": true}]
		}
	}`)

	snap, err := buildReplicasSnapshot([][]byte{varz1, varz2}, routez, jsz, 5)
	require.NoError(t, err)
	assert.Equal(t, 1, snap.OnlineCount)

	byName := map[string]replicaPeer{}
	for _, p := range snap.Peers {
		byName[p.Name] = p
	}
	assert.False(t, byName["n2"].Online)
	// Metrics from the last successful scrape are kept for diagnostics.
	require.NotNil(t, byName["n2"].CPU)
	assert.Equal(t, 2.0, *byName["n2"].CPU)
	assert.Equal(t, "90µs", byName["n2"].RTT)
	require.NotNil(t, byName["n2"].InMsgs)
	assert.Equal(t, int64(9), *byName["n2"].InMsgs)
}

func TestBuildReplicasSnapshotVarzAbsenceBeatsLingeringRoute(t *testing.T) {
	t.Parallel()

	// Multi-monitor: nats-2 gone, but routez still lists it until routes drain.
	varz1 := commonstrings.StringToBytes(`{
		"server_name": "nats-1",
		"version": "2.14.0",
		"connections": 1,
		"cpu": 1,
		"mem": 1,
		"cluster": {"name": "c1"}
	}`)
	varz3 := commonstrings.StringToBytes(`{
		"server_name": "nats-3",
		"version": "2.14.0",
		"connections": 2,
		"cpu": 0.5,
		"mem": 2
	}`)
	routez := commonstrings.StringToBytes(`{
		"server_name": "nats-1",
		"routes": [
			{"remote_name": "nats-2", "ip": "10.0.0.2", "rtt": "120µs", "in_msgs": 10, "out_msgs": 12},
			{"remote_name": "nats-3", "ip": "10.0.0.3", "rtt": "130µs", "in_msgs": 20, "out_msgs": 22}
		]
	}`)
	jsz := commonstrings.StringToBytes(`{
		"server_name": "nats-1",
		"meta_cluster": {
			"name": "hub",
			"leader": "nats-1",
			"cluster_size": 3,
			"replicas": [
				{"name": "nats-2", "current": false, "offline": false},
				{"name": "nats-3", "current": true, "offline": false}
			]
		}
	}`)

	snap, err := buildReplicasSnapshot([][]byte{varz1, varz3}, routez, jsz, 5)
	require.NoError(t, err)
	assert.Equal(t, 2, snap.OnlineCount)

	byName := map[string]replicaPeer{}
	for _, p := range snap.Peers {
		byName[p.Name] = p
	}
	assert.True(t, byName["nats-1"].Online)
	assert.False(t, byName["nats-2"].Online)
	assert.True(t, byName["nats-3"].Online)
	assert.Equal(t, "120µs", byName["nats-2"].RTT)
	require.NotNil(t, byName["nats-2"].InMsgs)
	assert.Equal(t, int64(10), *byName["nats-2"].InMsgs)
}

func TestBuildReplicasSnapshotUnknownPeerIDMapsToKnownName(t *testing.T) {
	t.Parallel()

	const peerID = "yrzKKRBu"
	replicasPeerIDNames.Store(peerID, "nats-1")
	t.Cleanup(func() { replicasPeerIDNames.Delete(peerID) })

	varz1 := commonstrings.StringToBytes(`{"server_name":"nats-2","connections":1,"cpu":1,"mem":1,"cluster":{"name":"c1"}}`)
	routez := commonstrings.StringToBytes(`{
		"server_name": "nats-2",
		"routes": [{"remote_name": "nats-1", "rtt": "100µs", "in_msgs": 1, "out_msgs": 1}]
	}`)
	jsz := commonstrings.StringToBytes(`{
		"server_name": "nats-2",
		"meta_cluster": {
			"leader": "nats-2",
			"cluster_size": 2,
			"replicas": [
				{"name": "Server name unknown at this time (peerID: yrzKKRBu)", "peer": "yrzKKRBu", "offline": true, "current": false}
			]
		}
	}`)

	snap, err := buildReplicasSnapshot([][]byte{varz1}, routez, jsz, 5)
	require.NoError(t, err)

	byName := map[string]replicaPeer{}
	for _, p := range snap.Peers {
		byName[p.Name] = p
	}
	_, ghost := byName["Server name unknown at this time (peerID: yrzKKRBu)"]
	assert.False(t, ghost, "placeholder name must not remain as a separate peer")
	require.Contains(t, byName, "nats-1")
	assert.False(t, byName["nats-1"].Online)
	assert.Equal(t, peerID, byName["nats-1"].PeerID)
	assert.Equal(t, 1, snap.OnlineCount)
}

func TestBuildReplicasSnapshotSingleMonitorLingeringRouteStaysUntilMeta(t *testing.T) {
	t.Parallel()

	// Single monitoring base cannot fan-out varz; route peers stay online until meta offline.
	varz := commonstrings.StringToBytes(`{"server_name":"nats-1","connections":1,"cluster":{"name":"c1"}}`)
	routez := commonstrings.StringToBytes(`{
		"server_name": "nats-1",
		"routes": [{"remote_name": "nats-2", "rtt": "120µs", "in_msgs": 10, "out_msgs": 12}]
	}`)
	jsz := commonstrings.StringToBytes(`{
		"server_name": "nats-1",
		"meta_cluster": {
			"leader": "nats-1",
			"cluster_size": 2,
			"replicas": [{"name": "nats-2", "offline": false}]
		}
	}`)
	snap, err := buildReplicasSnapshot([][]byte{varz}, routez, jsz, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, snap.OnlineCount)
}

func TestReplicasOfflineFromLatest(t *testing.T) {
	t.Parallel()

	replicasPreferredBase.Store("c-offline", "http://dead:8222")
	t.Cleanup(func() { replicasPreferredBase.Delete("c-offline") })

	latest := commonstrings.StringToBytes(`{
		"jetstreamLeader": "nats-1",
		"clusterSize": 2,
		"peerCount": 2,
		"onlineCount": 1,
		"peers": [
			{"name": "nats-1", "role": "monitored", "online": true, "leader": true},
			{"name": "nats-2", "role": "route", "online": false}
		]
	}`)
	got := replicasOfflineFromLatest("c-offline", latest)
	require.NotNil(t, got)

	var snap replicasSnapshot
	require.NoError(t, serializer.Unmarshal(got, &snap))
	assert.Equal(t, 0, snap.OnlineCount)
	assert.Equal(t, 2, snap.PeerCount)
	assert.Empty(t, snap.JetStreamLeader)
	for _, p := range snap.Peers {
		assert.False(t, p.Online, p.Name)
		assert.False(t, p.Leader, p.Name)
	}
	_, sticky := replicasPreferredBase.Load("c-offline")
	assert.False(t, sticky)

	assert.Nil(t, replicasOfflineFromLatest("c-offline", got), "already-offline snapshot should not republish")
}

func TestReplicasOfflineFromLatestUsesLastGoodWhenBrokerEmpty(t *testing.T) {
	t.Parallel()

	const id = "c-last-good-fallback"
	t.Cleanup(func() { replicasLastGood.Delete(id) })

	replicasLastGood.Store(id, commonstrings.StringToBytes(`{
		"peerCount": 1,
		"onlineCount": 1,
		"peers": [{"name": "nats-1", "role": "monitored", "online": true, "leader": true}]
	}`))

	got := replicasOfflineFromLatest(id, nil)
	require.NotNil(t, got)
	var snap replicasSnapshot
	require.NoError(t, serializer.Unmarshal(got, &snap))
	assert.Equal(t, 0, snap.OnlineCount)
	assert.Empty(t, snap.JetStreamLeader)
	require.Len(t, snap.Peers, 1)
	assert.False(t, snap.Peers[0].Online)
}

func TestReplicasDegradedFromLastGood(t *testing.T) {
	t.Parallel()

	const id = "c-degraded"
	t.Cleanup(func() { replicasLastGood.Delete(id) })

	good := commonstrings.StringToBytes(`{
		"peerCount": 2,
		"onlineCount": 2,
		"clusterSize": 2,
		"peers": [
			{"name": "nats-1", "role": "monitored", "online": true, "leader": true},
			{"name": "nats-2", "role": "route", "online": true}
		]
	}`)
	replicasLastGood.Store(id, good)

	snap, ok := replicasDegradedFromLastGood(id)
	require.True(t, ok)
	assert.Equal(t, 0, snap.OnlineCount)
	assert.Equal(t, 2, snap.PeerCount)
	for _, p := range snap.Peers {
		assert.False(t, p.Online, p.Name)
		assert.False(t, p.Leader, p.Name)
	}

	// Second call keeps serving all-offline from last-good.
	snap2, ok := replicasDegradedFromLastGood(id)
	require.True(t, ok)
	assert.Equal(t, 0, snap2.OnlineCount)
	assert.Equal(t, 2, len(snap2.Peers))
}

func TestReplicaPeerCurrentFalseSurvivesJSON(t *testing.T) {
	t.Parallel()

	cur := false
	raw, err := serializer.Marshal(replicaPeer{
		Name:    "nats-5",
		Role:    "route",
		Online:  true,
		Current: &cur,
	})
	require.NoError(t, err)
	assert.Contains(t, commonstrings.BytesToString(raw), `"current":false`)

	var got replicaPeer
	require.NoError(t, serializer.Unmarshal(raw, &got))
	require.NotNil(t, got.Current)
	assert.False(t, *got.Current)
	assert.True(t, got.Online)
}

func TestReplicaPeerCurrentOmittedWhenUnknown(t *testing.T) {
	t.Parallel()

	raw, err := serializer.Marshal(replicaPeer{
		Name:   "nats-1",
		Role:   "monitored",
		Online: true,
	})
	require.NoError(t, err)
	assert.NotContains(t, commonstrings.BytesToString(raw), `"current"`)

	var got replicaPeer
	require.NoError(t, serializer.Unmarshal(raw, &got))
	assert.Nil(t, got.Current)
}

func TestPreferBasePutsPreferredFirst(t *testing.T) {
	t.Parallel()

	bases := []string{"http://a:8222", "http://b:8222", "http://c:8222"}
	got := preferBase(bases, "http://b:8222")
	assert.Equal(t, []string{"http://b:8222", "http://a:8222", "http://c:8222"}, got)
	assert.Equal(t, bases, preferBase(bases, ""))
	// Unknown preferred is still tried first (sticky base may briefly outlive candidate list).
	assert.Equal(t, append([]string{"http://missing:8222"}, bases...), preferBase(bases, "http://missing:8222"))
}

func TestJSZHasMetaCluster(t *testing.T) {
	t.Parallel()

	assert.False(t, jszHasMetaCluster(nil))
	assert.False(t, jszHasMetaCluster(commonstrings.StringToBytes(`{"streams":1,"consumers":2,"messages":3}`)))
	assert.False(t, jszHasMetaCluster(commonstrings.StringToBytes(`{"meta_cluster":null}`)))
	assert.True(t, jszHasMetaCluster(commonstrings.StringToBytes(`{"meta_cluster":{"leader":"n1","cluster_size":3}}`)))
}

func TestHubJSZForReplicasIgnoresSlimJSZ(t *testing.T) {
	t.Parallel()

	hub := snapshot.NewHub()
	hub.Publish("c1", snapshot.ClusterSnapshot{
		Jsz:         commonstrings.StringToBytes(`{"streams":1,"consumers":2,"messages":3}`),
		JszTopology: commonstrings.StringToBytes(`{"server_name":"n1","meta_cluster":{"leader":"n1","cluster_size":2}}`),
	})

	got := hubJSZForReplicas(hub, "c1")
	require.NotNil(t, got)
	assert.True(t, jszHasMetaCluster(got))

	hubSlimOnly := snapshot.NewHub()
	hubSlimOnly.Publish("c1", snapshot.ClusterSnapshot{
		Jsz: commonstrings.StringToBytes(`{"streams":1,"consumers":2,"messages":3}`),
	})
	assert.Nil(t, hubJSZForReplicas(hubSlimOnly, "c1"))
}
