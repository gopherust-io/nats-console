package live

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestViewer() *muxViewer {
	return newMuxViewer(
		func(frame []byte) bool { return true },
		new(atomic.Bool),
		new(atomic.Bool),
		0,
	)
}

// H3: viewers requesting different fromSeq values (including 0/DeliverNew)
// must not share a mux subscription key, since sharing would silently drop
// the second viewer's requested replay position.
func TestMuxKeyDistinguishesFromSeq(t *testing.T) {
	t.Parallel()

	base := muxKey{cluster: "c1", stream: "s1", filter: ">"}
	deliverNew := base
	deliverNew.fromSeq = 0
	replay := base
	replay.fromSeq = 42
	otherReplay := base
	otherReplay.fromSeq = 7

	assert.NotEqual(t, deliverNew, replay)
	assert.NotEqual(t, replay, otherReplay)
}

// TestAttachSerializesConcurrentFirstSubscribe exercises the race where many
// viewers concurrently attach() to a brand-new key for the first time: the
// underlying subscribe function must be invoked exactly once, and every
// viewer must observe the resulting subscription (or its error).
func TestAttachSerializesConcurrentFirstSubscribe(t *testing.T) {
	t.Parallel()

	m := newSubMux()
	key := muxKey{cluster: "c1", stream: "s1", filter: ">", fromSeq: 0}

	var subscribeCalls atomic.Int32
	start := make(chan struct{})
	subscribeFn := func(ss *sharedSub) (*nats.Subscription, error) {
		subscribeCalls.Add(1)
		<-start // hold every concurrent attach() until we release them together
		return &nats.Subscription{}, nil
	}

	const n = 20
	viewers := make([]*muxViewer, n)
	unsubs := make([]func(), n)
	errs := make([]error, n)

	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		viewers[i] = newTestViewer()
		go func() {
			defer wg.Done()
			unsubs[i], errs[i] = m.attachWithSubscriber(key, viewers[i], subscribeFn)
		}()
	}
	close(start)
	wg.Wait()

	assert.Equal(t, int32(1), subscribeCalls.Load(), "subscribe function must run exactly once for concurrent first-attach")
	for i := range n {
		require.NoError(t, errs[i])
		require.NotNil(t, unsubs[i])
	}

	m.mu.Lock()
	ss := m.subs[key]
	m.mu.Unlock()
	require.NotNil(t, ss)
	ss.mu.Lock()
	assert.Len(t, ss.viewers, n)
	ss.mu.Unlock()

	for _, unsub := range unsubs {
		unsub()
	}
	m.mu.Lock()
	_, stillPresent := m.subs[key]
	m.mu.Unlock()
	assert.False(t, stillPresent, "shared sub should be removed once all viewers detach")
}

// TestAttachPropagatesSubscribeErrorToAllWaiters ensures that when the single
// underlying Subscribe call fails, every concurrent waiter (not just the one
// that triggered it) receives the error and is cleanly detached.
func TestAttachPropagatesSubscribeErrorToAllWaiters(t *testing.T) {
	t.Parallel()

	m := newSubMux()
	key := muxKey{cluster: "c1", stream: "s1", filter: ">", fromSeq: 5}
	boom := assert.AnError
	subscribeFn := func(ss *sharedSub) (*nats.Subscription, error) {
		return nil, boom
	}

	const n = 5
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make([]error, n)
	for i := range n {
		go func() {
			defer wg.Done()
			_, errs[i] = m.attachWithSubscriber(key, newTestViewer(), subscribeFn)
		}()
	}
	wg.Wait()

	for i := range n {
		assert.ErrorIs(t, errs[i], boom)
	}

	m.mu.Lock()
	_, present := m.subs[key]
	m.mu.Unlock()
	assert.False(t, present, "failed shared sub should not linger in the map")
}

// TestAttachReusesExistingSubscription verifies that a second, sequential
// attach to an already-established key reuses the existing subscription
// instead of subscribing again.
func TestAttachReusesExistingSubscription(t *testing.T) {
	t.Parallel()

	m := newSubMux()
	key := muxKey{cluster: "c1", stream: "s1", filter: ">", fromSeq: 0}
	var subscribeCalls atomic.Int32
	subscribeFn := func(ss *sharedSub) (*nats.Subscription, error) {
		subscribeCalls.Add(1)
		return &nats.Subscription{}, nil
	}

	v1 := newTestViewer()
	unsub1, err := m.attachWithSubscriber(key, v1, subscribeFn)
	require.NoError(t, err)
	defer unsub1()

	v2 := newTestViewer()
	unsub2, err := m.attachWithSubscriber(key, v2, subscribeFn)
	require.NoError(t, err)
	defer unsub2()

	assert.Equal(t, int32(1), subscribeCalls.Load())
}

func TestFanoutDeliversSharedFrameToAllViewers(t *testing.T) {
	t.Parallel()

	m := newSubMux()
	key := muxKey{cluster: "c1", stream: "s1", filter: ">", fromSeq: 0}
	const n = 5
	var delivers atomic.Int32
	viewers := make([]*muxViewer, n)
	unsubs := make([]func(), n)
	for i := range n {
		viewers[i] = newMuxViewer(func(frame []byte) bool {
			delivers.Add(1)
			assert.Contains(t, string(frame), `"type":"message"`)
			assert.Contains(t, string(frame), "orders.created")
			return true
		}, new(atomic.Bool), new(atomic.Bool), 0)
		unsub, err := m.attachWithSubscriber(key, viewers[i], func(ss *sharedSub) (*nats.Subscription, error) {
			return &nats.Subscription{}, nil
		})
		require.NoError(t, err)
		unsubs[i] = unsub
	}

	m.mu.Lock()
	ss := m.subs[key]
	m.mu.Unlock()
	require.NotNil(t, ss)

	ss.fanout(&nats.Msg{Subject: "orders.created", Data: []byte(`{"ok":true}`)})
	deadline := time.Now().Add(time.Second)
	for delivers.Load() < int32(n) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	assert.Equal(t, int32(n), delivers.Load())

	for _, unsub := range unsubs {
		unsub()
	}
}
