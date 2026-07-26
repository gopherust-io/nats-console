package live

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type muxKey struct {
	cluster string
	stream  string
	filter  string
}

type muxViewer struct {
	send     func(seq uint64, subject string, payload []byte, now time.Time) bool
	paused   *atomic.Bool
	closed   *atomic.Bool
	truncate int
}

type sharedSub struct {
	viewers map[*muxViewer]struct{}
	sub     *nats.Subscription
	key     muxKey
	mu      sync.Mutex
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
	m.mu.Lock()
	ss, ok := m.subs[key]
	if !ok {
		ss = &sharedSub{
			viewers: make(map[*muxViewer]struct{}),
			key:     key,
		}
		m.subs[key] = ss
	}
	ss.mu.Lock()
	ss.viewers[viewer] = struct{}{}
	needSubscribe := ss.sub == nil
	ss.mu.Unlock()
	m.mu.Unlock()

	if needSubscribe {
		subject := ">"
		subOpts := []nats.SubOpt{nats.BindStream(key.stream)}
		if key.filter != "" {
			subject = key.filter
		}
		if fromSeq > 0 {
			subOpts = append(subOpts, nats.StartSequence(fromSeq))
		} else {
			subOpts = append(subOpts, nats.DeliverNew())
		}
		sub, subErr := client.JetStream().Subscribe(subject, func(msg *nats.Msg) {
			ss.fanout(msg)
		}, subOpts...)
		if subErr != nil {
			m.detach(key, viewer)
			return nil, subErr
		}
		ss.mu.Lock()
		if ss.sub == nil {
			ss.sub = sub
		} else {
			_ = sub.Unsubscribe()
		}
		ss.mu.Unlock()
	}

	return func() { m.detach(key, viewer) }, nil
}

func (m *subMux) detach(key muxKey, viewer *muxViewer) {
	m.mu.Lock()
	ss, ok := m.subs[key]
	if !ok {
		m.mu.Unlock()
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
	for v := range ss.viewers {
		viewers = append(viewers, v)
	}
	ss.mu.Unlock()

	metrics.IncLiveMuxFanout(len(viewers))
	for _, v := range viewers {
		if v.closed.Load() || v.paused.Load() {
			continue
		}
		payload := msg.Data
		if v.truncate > 0 && len(payload) > v.truncate {
			payload = payload[:v.truncate]
		}
		_ = v.send(seq, msg.Subject, payload, now)
	}
}
