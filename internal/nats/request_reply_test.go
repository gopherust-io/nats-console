package natsclient_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/domain"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestParseRttMs(t *testing.T) {
	t.Parallel()

	ms, ok := natsclient.ParseRttMs("1.23ms")
	require.True(t, ok)
	assert.InDelta(t, 1.23, ms, 0.001)

	ms, ok = natsclient.ParseRttMs("2.5s")
	require.True(t, ok)
	assert.InDelta(t, 2500, ms, 0.001)

	_, ok = natsclient.ParseRttMs("")
	assert.False(t, ok)
	_, ok = natsclient.ParseRttMs("bad")
	assert.False(t, ok)
}

func TestBuildRequestReplySnapshot(t *testing.T) {
	t.Parallel()

	raw := strings.StringToBytes(`{
		"connections": [
			{
				"cid": 1,
				"name": "requester",
				"account": "APP",
				"rtt": "2ms",
				"subscriptions": [
					{"subject": "_INBOX.>"}
				]
			},
			{
				"cid": 2,
				"name": "responder",
				"account": "APP",
				"rtt": "4ms",
				"subscriptions": [
					{"subject": "orders.status", "queue": "workers"}
				]
			},
			{
				"cid": 3,
				"name": "js-internal",
				"subscriptions": [
					{"subject": "$JS.API.STREAM.INFO.ORDERS"}
				]
			}
		]
	}`)

	snap := natsclient.BuildRequestReplySnapshot(raw, []domain.RequestReplyProbeResult{
		{Subject: "orders.status", LatencyMs: 12.5, OK: true},
	})

	require.Len(t, snap.Patterns, 1)
	pat := snap.Patterns[0]
	assert.Equal(t, "orders.status", pat.Subject)
	assert.Equal(t, "workers", pat.Queue)
	assert.Equal(t, 1, pat.RequesterCount)
	assert.Equal(t, 1, pat.ResponderCount)
	require.NotNil(t, pat.RttMinMs)
	require.NotNil(t, pat.RttMedianMs)
	require.NotNil(t, pat.RttMaxMs)
	assert.InDelta(t, 4, *pat.RttMinMs, 0.001)
	assert.InDelta(t, 4, *pat.RttMedianMs, 0.001)
	assert.InDelta(t, 4, *pat.RttMaxMs, 0.001)
	require.NotNil(t, pat.ProbeLatencyMs)
	assert.InDelta(t, 12.5, *pat.ProbeLatencyMs, 0.001)
	require.NotNil(t, pat.ProbeOk)
	assert.True(t, *pat.ProbeOk)

	assert.Equal(t, 1, snap.Requesters)
	assert.Equal(t, 1, snap.Responders)
	require.NotNil(t, snap.MedianRttMs)
	assert.InDelta(t, 3, *snap.MedianRttMs, 0.001) // (2+4)/2
	require.NotNil(t, snap.MaxProbeMs)
	assert.InDelta(t, 12.5, *snap.MaxProbeMs, 0.001)

	require.Len(t, snap.Connections, 2)
	assert.Equal(t, 1, snap.Connections[0].CID)
	require.NotNil(t, snap.Connections[0].RttMs)
	assert.InDelta(t, 2, *snap.Connections[0].RttMs, 0.001)
	assert.Equal(t, 2, snap.Connections[1].CID)
	require.NotNil(t, snap.Connections[1].RttMs)
	assert.InDelta(t, 4, *snap.Connections[1].RttMs, 0.001)
	require.Len(t, snap.Connections[1].ServiceSubs, 1)
	assert.Equal(t, "orders.status (queue:workers)", snap.Connections[1].ServiceSubs[0])
}

func TestBuildRequestReplySnapshotSubscriptionsList(t *testing.T) {
	t.Parallel()

	// Modern NATS /connz?subs=1: count in "subscriptions", subjects in subscriptions_list.
	raw := strings.StringToBytes(`{
		"connections": [
			{
				"cid": 10,
				"name": "requester",
				"rtt": "1ms",
				"subscriptions": 1,
				"subscriptions_list": ["_INBOX.abc123"]
			},
			{
				"cid": 11,
				"name": "responder",
				"rtt": "2ms",
				"subscriptions": 1,
				"subscriptions_list": ["orders.status"]
			}
		]
	}`)

	snap := natsclient.BuildRequestReplySnapshot(raw, nil)
	assert.Equal(t, 1, snap.Requesters)
	assert.Equal(t, 1, snap.Responders)
	require.Len(t, snap.Patterns, 1)
	assert.Equal(t, "orders.status", snap.Patterns[0].Subject)
	require.Len(t, snap.Connections, 2)
}

func TestBuildRequestReplySnapshotSubscriptionsListDetail(t *testing.T) {
	t.Parallel()

	// Modern NATS /connz?subs=detail: subjects + qgroup in subscriptions_list_detail.
	raw := strings.StringToBytes(`{
		"connections": [
			{
				"cid": 20,
				"name": "worker",
				"rtt": "3ms",
				"subscriptions": 2,
				"subscriptions_list_detail": [
					{"subject": "_INBOX.xyz", "sid": "1"},
					{"subject": "orders.status", "qgroup": "workers", "sid": "2"}
				]
			}
		]
	}`)

	snap := natsclient.BuildRequestReplySnapshot(raw, nil)
	assert.Equal(t, 1, snap.Requesters)
	assert.Equal(t, 1, snap.Responders)
	require.Len(t, snap.Patterns, 1)
	assert.Equal(t, "orders.status", snap.Patterns[0].Subject)
	assert.Equal(t, "workers", snap.Patterns[0].Queue)
	require.Len(t, snap.Connections, 1)
	require.Len(t, snap.Connections[0].ServiceSubs, 1)
	assert.Equal(t, "orders.status (queue:workers)", snap.Connections[0].ServiceSubs[0])
}

func TestBuildRequestReplySnapshotJSONPointersStable(t *testing.T) {
	t.Parallel()

	// Many patterns exercise ptrSlab; values must remain readable after the function returns.
	raw := strings.StringToBytes(`{
		"connections": [
			{"cid":1,"rtt":"1ms","subscriptions":[{"subject":"_INBOX.>"}]},
			{"cid":2,"rtt":"3ms","subscriptions":[
				{"subject":"svc.a","queue":"q"},
				{"subject":"svc.b"},
				{"subject":"svc.c","queue":"q2"}
			]},
			{"cid":3,"rtt":"5ms","subscriptions":[
				{"subject":"svc.d"},
				{"subject":"svc.e","queue":"workers"}
			]}
		]
	}`)
	probes := []domain.RequestReplyProbeResult{
		{Subject: "svc.a", LatencyMs: 10, OK: true},
		{Subject: "svc.b", LatencyMs: 20, OK: false, Error: "timeout"},
		{Subject: "svc.e", LatencyMs: 7.5, OK: true},
	}
	snap := natsclient.BuildRequestReplySnapshot(raw, probes)
	require.GreaterOrEqual(t, len(snap.Patterns), 5)

	for _, p := range snap.Patterns {
		require.NotNil(t, p.RttMinMs)
		require.NotNil(t, p.RttMedianMs)
		require.NotNil(t, p.RttMaxMs)
		assert.Greater(t, *p.RttMinMs, 0.0)
		if p.Subject == "svc.a" {
			require.NotNil(t, p.ProbeLatencyMs)
			assert.InDelta(t, 10, *p.ProbeLatencyMs, 0.001)
			require.NotNil(t, p.ProbeOk)
			assert.True(t, *p.ProbeOk)
		}
		if p.Subject == "svc.b" {
			require.NotNil(t, p.ProbeOk)
			assert.False(t, *p.ProbeOk)
			assert.Equal(t, "timeout", p.ProbeError)
		}
	}
	for _, c := range snap.Connections {
		require.NotNil(t, c.RttMs)
		assert.Greater(t, *c.RttMs, 0.0)
	}
	require.NotNil(t, snap.MedianRttMs)
	require.NotNil(t, snap.MaxProbeMs)
	assert.InDelta(t, 10, *snap.MaxProbeMs, 0.001)
}
