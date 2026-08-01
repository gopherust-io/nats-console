package live

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/pkg/common/safe"
)

const viewerOutboundBuffer = 64

// muxKey identifies a shared JetStream subscription. fromSeq is part of the
// key (0 meaning "DeliverNew") so that a viewer replaying from a specific
// sequence never shares a subscription - and therefore never shares delivery
// position - with a DeliverNew viewer or a viewer replaying from a different
// sequence (H3).
type muxKey struct {
	cluster string
	stream  string
	filter  string
	fromSeq uint64
}

type outboundMsg struct {
	frame []byte // pre-encoded immutable WS JSON frame
}

type muxViewer struct {
	deliver  func(frame []byte) bool
	out      chan outboundMsg
	paused   *atomic.Bool
	closed   *atomic.Bool
	stopOnce sync.Once
	truncate int
}

func newMuxViewer(
	deliver func(frame []byte) bool,
	paused, closed *atomic.Bool,
	truncate int,
) *muxViewer {
	v := &muxViewer{
		deliver:  deliver,
		out:      make(chan outboundMsg, viewerOutboundBuffer),
		paused:   paused,
		closed:   closed,
		truncate: truncate,
	}
	go v.writeLoop()
	return v
}

func (v *muxViewer) writeLoop() {
	for msg := range v.out {
		if v.closed.Load() {
			continue
		}
		frame := msg.frame
		safe.Run("live_ws", func() { _ = v.deliver(frame) })
	}
}

func (v *muxViewer) stop() {
	v.stopOnce.Do(func() {
		close(v.out)
	})
}

// trySendFrame enqueues a shared immutable encoded frame without copying.
func (v *muxViewer) trySendFrame(frame []byte) bool {
	if v.closed.Load() || v.paused.Load() || len(frame) == 0 {
		return false
	}
	select {
	case v.out <- outboundMsg{frame: frame}:
		return true
	default:
		metrics.IncLiveWSFramesDropped()
		return false
	}
}

type sharedSub struct {
	viewers map[*muxViewer]struct{}
	sub     *nats.Subscription
	subErr  error
	// ready is closed once the first attach for this key has finished
	// attempting the underlying JetStream subscribe (success or failure),
	// so concurrent late-arriving attach calls wait for that single attempt
	// instead of racing to subscribe themselves.
	ready      chan struct{}
	key        muxKey
	mu         sync.Mutex
	subStarted bool
}

type subMux struct {
	subs map[muxKey]*sharedSub
	mu   sync.Mutex
}

func newSubMux() *subMux {
	return &subMux{subs: make(map[muxKey]*sharedSub)}
}

func (m *subMux) attach(
	client port.JetStreamExecutor,
	key muxKey,
	fromSeq uint64,
	viewer *muxViewer,
) (unsubscribe func(), err error) {
	return m.attachWithSubscriber(key, viewer, func(ss *sharedSub) (*nats.Subscription, error) {
		subject := ">"
		subOpts := []nats.SubOpt{nats.BindStream(key.stream)}
		if !strings.IsEmpty(key.filter) {
			subject = key.filter
		}
		if fromSeq > 0 {
			subOpts = append(subOpts, nats.StartSequence(fromSeq))
		} else {
			subOpts = append(subOpts, nats.DeliverNew())
		}
		return client.JetStream().Subscribe(subject, func(msg *nats.Msg) {
			ss.fanout(msg)
		}, subOpts...)
	})
}

// attachWithSubscriber implements attach's join/first-subscriber-wins logic
// against an injectable subscribe function, so the concurrency behavior (H3)
// can be unit tested without a real NATS connection.
func (m *subMux) attachWithSubscriber(
	key muxKey,
	viewer *muxViewer,
	subscribe func(ss *sharedSub) (*nats.Subscription, error),
) (unsubscribe func(), err error) {
	m.mu.Lock()
	ss, ok := m.subs[key]
	if !ok {
		ss = &sharedSub{
			viewers: make(map[*muxViewer]struct{}),
			key:     key,
			ready:   make(chan struct{}),
		}
		m.subs[key] = ss
	}
	ss.mu.Lock()
	ss.viewers[viewer] = struct{}{}
	needSubscribe := !ss.subStarted
	if needSubscribe {
		ss.subStarted = true
	}
	ready := ss.ready
	ss.mu.Unlock()
	m.mu.Unlock()

	if needSubscribe {
		sub, subErr := subscribe(ss)
		ss.mu.Lock()
		ss.sub = sub
		ss.subErr = subErr
		ss.mu.Unlock()
		close(ready)
		if subErr != nil {
			m.detach(key, viewer)
			return nil, subErr
		}
		return func() { m.detach(key, viewer) }, nil
	}

	// A concurrent attach for the same brand-new key is already subscribing;
	// wait for that single attempt instead of racing our own Subscribe call.
	<-ready
	ss.mu.Lock()
	subErr := ss.subErr
	ss.mu.Unlock()
	if subErr != nil {
		m.detach(key, viewer)
		return nil, subErr
	}
	return func() { m.detach(key, viewer) }, nil
}

func (m *subMux) detach(key muxKey, viewer *muxViewer) {
	m.mu.Lock()
	ss, ok := m.subs[key]
	if !ok {
		m.mu.Unlock()
		viewer.stop()
		return
	}
	ss.mu.Lock()
	delete(ss.viewers, viewer)
	empty := len(ss.viewers) == 0
	var sub *nats.Subscription
	if empty {
		sub = ss.sub
		ss.sub = nil
	}
	ss.mu.Unlock()
	if empty {
		delete(m.subs, key)
	}
	m.mu.Unlock()
	viewer.stop()
	if sub != nil {
		_ = sub.Unsubscribe()
	}
}

func (ss *sharedSub) fanout(msg *nats.Msg) {
	seq := uint64(0)
	if meta, metaErr := msg.Metadata(); metaErr == nil && meta != nil {
		seq = meta.Sequence.Stream
	}
	now := time.Now()

	ss.mu.Lock()
	viewers := make([]*muxViewer, 0, len(ss.viewers))
	truncate := 0
	for v := range ss.viewers {
		if v.closed.Load() || v.paused.Load() {
			continue
		}
		viewers = append(viewers, v)
		if v.truncate > 0 && (truncate == 0 || v.truncate < truncate) {
			truncate = v.truncate
		}
	}
	ss.mu.Unlock()

	if len(viewers) == 0 {
		return
	}

	payload := msg.Data
	if truncate > 0 && len(payload) > truncate {
		payload = payload[:truncate]
	}
	headers := headerMapFromNATS(msg.Header)
	frame, err := encodeMessageFrame(seq, msg.Subject, payload, headers, now)
	if err != nil {
		return
	}

	metrics.IncLiveMuxFanout(len(viewers))
	for _, v := range viewers {
		_ = v.trySendFrame(frame)
	}
}
