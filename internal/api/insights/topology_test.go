package insights

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/app/monitoring"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestBuildTopologyTreeRaftRolesAndUnhealthy(t *testing.T) {
	raw := strings.StringToBytes(`{
		"server_name": "n1",
		"meta_cluster": {
			"name": "meta",
			"leader": "n1",
			"cluster_size": 3,
			"replicas": [
				{"name": "n2", "current": true, "offline": false},
				{"name": "n3", "current": true, "offline": false}
			]
		},
		"account_details": [{
			"name": "ACC",
			"stream_detail": [
				{
					"name": "ORDERS",
					"config": {
						"retention": "limits",
						"storage": "file",
						"subjects": ["orders.>"],
						"num_replicas": 3
					},
					"state": {"messages": 10, "consumer_count": 1, "bytes": 100, "last_seq": 3000},
					"cluster": {
						"name": "C1",
						"raft_group": "S-ORDERS",
						"leader": "n1",
						"replicas": [
							{"name": "n2", "current": true, "offline": false, "lag": 0},
							{"name": "n3", "current": false, "offline": true, "lag": 40}
						]
					},
					"consumer_detail": [{
						"name": "worker",
						"num_pending": 2000,
						"num_ack_pending": 950,
						"num_waiting": 1,
						"num_redelivered": 4,
						"delivered": {"consumer_seq": 50, "stream_seq": 90},
						"config": {"filter_subject": "orders.*", "deliver_policy": "all", "num_replicas": 3, "max_ack_pending": 1000},
						"cluster": {
							"raft_group": "C-worker",
							"leader": "n2",
							"replicas": [{"name": "n1", "current": true}, {"name": "n3", "current": true}]
						}
					}]
				},
				{
					"name": "LOCAL",
					"config": {
						"retention": "limits",
						"storage": "memory",
						"subjects": ["local"],
						"num_replicas": 1
					},
					"state": {"messages": 0, "consumer_count": 0, "bytes": 0},
					"consumer_detail": []
				},
				{
					"name": "NOLEADER",
					"config": {
						"retention": "limits",
						"storage": "file",
						"subjects": ["x"],
						"num_replicas": 3
					},
					"state": {"messages": 0, "consumer_count": 0, "bytes": 0},
					"cluster": {
						"raft_group": "S-NOLEADER",
						"leader": "",
						"replicas": [
							{"name": "n2", "current": true},
							{"name": "n3", "current": true}
						]
					},
					"consumer_detail": []
				}
			]
		}]
	}`)

	payload, err := monitoring.ParsePayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := buildTopologyTree(payload, "demo", "n1", domain.SlowConsumerThresholds{})
	if err != nil {
		t.Fatal(err)
	}
	if tree.Kind != "cluster" {
		t.Fatalf("kind=%s", tree.Kind)
	}
	if tree.Role != "leader" {
		t.Fatalf("meta role=%s want leader", tree.Role)
	}
	if tree.Status != "healthy" {
		t.Fatalf("meta status=%s", tree.Status)
	}
	if tree.Raft == nil || tree.Raft.ClusterSize != 3 || tree.Raft.Leader != "n1" {
		t.Fatalf("meta raft=%+v", tree.Raft)
	}

	byName := map[string]topologyNode{}
	for _, s := range tree.Children {
		byName[s.Name] = s
	}

	orders := byName["ORDERS"]
	if orders.Role != "leader" {
		t.Fatalf("ORDERS role=%s", orders.Role)
	}
	if orders.Status != "unhealthy" {
		t.Fatalf("ORDERS status=%s want unhealthy (offline peer)", orders.Status)
	}
	if orders.Raft == nil || orders.Raft.Group != "S-ORDERS" {
		t.Fatalf("ORDERS raft=%+v", orders.Raft)
	}
	foundRoleMeta := false
	for _, m := range orders.Meta {
		if m == "leader" || m == "R3" {
			foundRoleMeta = true
		}
	}
	if !foundRoleMeta {
		t.Fatalf("ORDERS meta missing role chips: %v", orders.Meta)
	}

	var worker *topologyNode
	for i := range orders.Children {
		if orders.Children[i].Kind == "consumer" {
			worker = &orders.Children[i]
			break
		}
	}
	if worker == nil {
		t.Fatal("expected consumer")
	}
	if worker.Role != "replica" {
		t.Fatalf("worker role=%s", worker.Role)
	}
	if worker.Status != "warning" {
		t.Fatalf("worker status=%s want warning (slow consumer + healthy raft)", worker.Status)
	}
	wantMeta := map[string]bool{"slow": false, "pending": false, "ack pending": false, "waiting": false, "redelivered": false, "lag 2910": false}
	for _, m := range worker.Meta {
		if _, ok := wantMeta[m]; ok {
			wantMeta[m] = true
		}
	}
	for chip, ok := range wantMeta {
		if !ok {
			t.Fatalf("worker meta missing %q: %v", chip, worker.Meta)
		}
	}

	local := byName["LOCAL"]
	if local.Role != "standalone" {
		t.Fatalf("LOCAL role=%s", local.Role)
	}
	if local.Status != "healthy" {
		t.Fatalf("LOCAL status=%s", local.Status)
	}
	if local.Raft != nil {
		t.Fatalf("LOCAL should have no raft, got %+v", local.Raft)
	}

	noleader := byName["NOLEADER"]
	if noleader.Status != "unhealthy" {
		t.Fatalf("NOLEADER status=%s", noleader.Status)
	}
}

func TestProjectJSZForTopologyKeepsClusterFields(t *testing.T) {
	raw := strings.StringToBytes(`{
		"server_name": "n1",
		"meta_cluster": {"leader": "n1", "cluster_size": 3, "replicas": [{"name": "n2", "current": true}]},
		"account_details": [{
			"name": "A",
			"stream_detail": [{
				"name": "S",
				"config": {"subjects": ["a"], "num_replicas": 3, "retention": "limits", "storage": "file"},
				"state": {"messages": 1},
				"cluster": {"leader": "n1", "raft_group": "G", "replicas": [{"name": "n2", "offline": true}]},
				"consumer_detail": [],
				"unused_bulky": {"x": 1}
			}]
		}],
		"noise": true
	}`)
	out := projectJSZForTopology(raw)
	payload, err := monitoring.ParsePayload(out)
	if err != nil {
		t.Fatal(err)
	}
	if payload.MetaCluster == nil || payload.MetaCluster.Leader != "n1" {
		t.Fatalf("meta=%+v", payload.MetaCluster)
	}
	if len(payload.AccountDetails) != 1 || len(payload.AccountDetails[0].StreamDetail) != 1 {
		t.Fatalf("accounts=%+v", payload.AccountDetails)
	}
	st := payload.AccountDetails[0].StreamDetail[0]
	if st.Cluster == nil || st.Cluster.Leader != "n1" || st.Config == nil || st.Config.NumReplicas != 3 {
		t.Fatalf("stream=%+v", st)
	}
}

func TestRaftRoleWithoutServerName(t *testing.T) {
	r := &topologyRaft{Leader: "n1", ClusterSize: 3, Peers: []topologyPeer{{Name: "n2", Current: true}}}
	if got := raftRole(r, 3, ""); got != "replica" {
		t.Fatalf("got %s", got)
	}
	r2 := &topologyRaft{Leader: "n1", ClusterSize: 3}
	if got := raftRole(r2, 3, ""); got != "leader" {
		t.Fatalf("got %s", got)
	}
}
