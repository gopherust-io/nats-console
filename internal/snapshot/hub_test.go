package snapshot_test

import (
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/snapshot"
)

func TestHubPublishAndMonitoringPayload(t *testing.T) {
	hub := snapshot.NewHub()
	hub.Publish("c1", snapshot.ClusterSnapshot{
		CapturedAt:  time.Unix(1700000000, 0).UTC(),
		Varz:        []byte(`{"connections":1}`),
		Jsz:         []byte(`{"streams":2}`),
		JszTopology: []byte(`{"account_details":[]}`),
	})

	if _, _, ok := hub.MonitoringPayload("c1", "/varz"); !ok {
		t.Fatal("expected varz hit")
	}
	if _, _, ok := hub.MonitoringPayload("c1", "/jsz"); !ok {
		t.Fatal("expected jsz hit")
	}
	if _, _, ok := hub.MonitoringPayload("c1", snapshot.TopologyJSZPath); !ok {
		t.Fatal("expected topology jsz hit")
	}
	if _, _, ok := hub.MonitoringPayload("c1", "/jsz?streams=1&consumers=1&config=1"); !ok {
		t.Fatal("expected equivalent topology query hit")
	}
	if _, _, ok := hub.MonitoringPayload("missing", "/varz"); ok {
		t.Fatal("expected miss")
	}
}

func TestHubSubscribe(t *testing.T) {
	hub := snapshot.NewHub()
	ch, unsub := hub.Subscribe(4)
	defer unsub()

	hub.Publish("c1", snapshot.ClusterSnapshot{Varz: []byte(`{}`)})
	select {
	case ev := <-ch:
		if ev.ClusterID != "c1" {
			t.Fatalf("cluster id = %q", ev.ClusterID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestHubInvalidate(t *testing.T) {
	hub := snapshot.NewHub()
	hub.Publish("c1", snapshot.ClusterSnapshot{
		Varz:        []byte(`{}`),
		JszTopology: []byte(`{"account_details":[]}`),
	})
	if _, _, ok := hub.MonitoringPayload("c1", snapshot.TopologyJSZPath); !ok {
		t.Fatal("expected topology hit before invalidate")
	}
	hub.Invalidate("c1")
	if _, _, ok := hub.MonitoringPayload("c1", snapshot.TopologyJSZPath); ok {
		t.Fatal("expected miss after invalidate")
	}
}
