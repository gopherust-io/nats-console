package snapshot_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/snapshot"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestConnzBrokerStartsOnFirstStopsOnLast(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	payload := commonstrings.StringToBytes(`{"num_connections":1,"connections":[]}`)
	broker := snapshot.NewConnzBroker(func(ctx context.Context, clusterID string) ([]byte, error) {
		calls.Add(1)
		assert.Equal(t, "c1", clusterID)
		return payload, nil
	}, 30*time.Millisecond)

	updates1, latest1, unsub1 := broker.Subscribe("c1")
	require.Nil(t, latest1)
	assert.Equal(t, 1, broker.ActiveClusters())
	assert.Equal(t, 1, broker.SubscriberCount("c1"))

	select {
	case got := <-updates1:
		assert.JSONEq(t, commonstrings.BytesToString(payload), commonstrings.BytesToString(got))
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first scrape")
	}
	require.GreaterOrEqual(t, calls.Load(), int32(1))

	updates2, latest2, unsub2 := broker.Subscribe("c1")
	require.JSONEq(t, commonstrings.BytesToString(payload), commonstrings.BytesToString(latest2))
	assert.Equal(t, 2, broker.SubscriberCount("c1"))
	assert.Equal(t, 1, broker.ActiveClusters())

	callsBefore := calls.Load()
	unsub1()
	assert.Equal(t, 1, broker.SubscriberCount("c1"))
	assert.Equal(t, 1, broker.ActiveClusters())

	// Still scraping for the remaining subscriber.
	deadline := time.Now().Add(500 * time.Millisecond)
	for calls.Load() <= callsBefore && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	assert.Greater(t, calls.Load(), callsBefore)

	unsub2()
	assert.Equal(t, 0, broker.SubscriberCount("c1"))
	assert.Equal(t, 0, broker.ActiveClusters())

	// Channel closed path is not required; ensure scrape stops.
	final := calls.Load()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, final, calls.Load(), "scrape loop should stop after last unsubscribe")

	// Drain so test doesn't leak blocked goroutines if any send pending.
	select {
	case <-updates2:
	default:
	}
}

func TestConnzBrokerIndependentClusters(t *testing.T) {
	t.Parallel()

	var c1Calls, c2Calls atomic.Int32
	broker := snapshot.NewConnzBroker(func(ctx context.Context, clusterID string) ([]byte, error) {
		switch clusterID {
		case "c1":
			c1Calls.Add(1)
			return commonstrings.StringToBytes(`{"cluster":"c1"}`), nil
		case "c2":
			c2Calls.Add(1)
			return commonstrings.StringToBytes(`{"cluster":"c2"}`), nil
		default:
			t.Fatalf("unexpected cluster %s", clusterID)
			return nil, nil
		}
	}, 40*time.Millisecond)

	_, _, unsub1 := broker.Subscribe("c1")
	_, _, unsub2 := broker.Subscribe("c2")
	defer unsub1()
	defer unsub2()

	assert.Equal(t, 2, broker.ActiveClusters())

	deadline := time.Now().Add(time.Second)
	for (c1Calls.Load() < 1 || c2Calls.Load() < 1) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	assert.GreaterOrEqual(t, c1Calls.Load(), int32(1))
	assert.GreaterOrEqual(t, c2Calls.Load(), int32(1))

	unsub1()
	assert.Equal(t, 1, broker.ActiveClusters())
	assert.Equal(t, 0, broker.SubscriberCount("c1"))
	assert.Equal(t, 1, broker.SubscriberCount("c2"))
}

func TestReplicasScrapeTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, snapshot.DefaultReplicasScrapeTimeout, snapshot.ReplicasScrapeTimeout(0))
	assert.Equal(t, snapshot.DefaultReplicasScrapeTimeout, snapshot.ReplicasScrapeTimeout(1))
	// 5 candidates: 2×5×1s + 3s = 13s → capped at maxReplicasScrapeTimeout (8s)
	assert.Equal(t, 8*time.Second, snapshot.ReplicasScrapeTimeout(5))
	assert.Equal(t, 8*time.Second, snapshot.ReplicasScrapeTimeout(8))
}
