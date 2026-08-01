package snapshot_test

import (
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestHubPublishAndMonitoringPayload(t *testing.T) {
	hub := snapshot.NewHub()
	hub.Publish("c1", snapshot.ClusterSnapshot{
		CapturedAt:  time.Unix(1700000000, 0).UTC(),
		Varz:        strings.StringToBytes(`{"connections":1}`),
		Jsz:         strings.StringToBytes(`{"streams":2}`),
		JszTopology: strings.StringToBytes(`{"account_details":[]}`),
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
	ch, unsub := hub.SubscribeCluster("c1", 4)
	defer unsub()

	hub.Publish("c1", snapshot.ClusterSnapshot{Varz: strings.StringToBytes(`{}`)})
	select {
	case ev := <-ch:
		if ev.ClusterID != "c1" {
			t.Fatalf("cluster id = %q", ev.ClusterID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Other cluster publishes must not wake this subscriber.
	hub.Publish("c2", snapshot.ClusterSnapshot{Varz: strings.StringToBytes(`{}`)})
	select {
	case ev := <-ch:
		t.Fatalf("unexpected event for other cluster: %+v", ev)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHubUnsubscribeDoesNotPanicOnConcurrentPublish(t *testing.T) {
	hub := snapshot.NewHub()
	ch, unsub := hub.SubscribeCluster("c1", 1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 200 {
			hub.Publish("c1", snapshot.ClusterSnapshot{Varz: strings.StringToBytes(`{}`)})
		}
	}()

	// Drain a bit then unsubscribe while publishes are in flight.
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for first event")
	}
	unsub()
	<-done

	// Unsub must not close the channel (Publish may still hold a copy).
	select {
	case _, ok := <-ch:
		if !ok {
			t.Fatal("unsubscribe closed the channel; concurrent Publish can panic")
		}
	default:
	}
}

func TestHubInvalidate(t *testing.T) {
	hub := snapshot.NewHub()
	hub.Publish("c1", snapshot.ClusterSnapshot{
		Varz:        strings.StringToBytes(`{}`),
		JszTopology: strings.StringToBytes(`{"account_details":[]}`),
	})
	if _, _, ok := hub.MonitoringPayload("c1", snapshot.TopologyJSZPath); !ok {
		t.Fatal("expected topology hit before invalidate")
	}
	hub.Invalidate("c1")
	if _, _, ok := hub.MonitoringPayload("c1", snapshot.TopologyJSZPath); ok {
		t.Fatal("expected miss after invalidate")
	}
}
