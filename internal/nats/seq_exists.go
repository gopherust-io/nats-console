package natsclient

import (
	"sync"
)

// seqExistsCache remembers which stream sequences were confirmed present/absent
// during message navigation, avoiding repeated GetMsg probes in a session
type seqExistsCache struct {
	byKey map[string]*streamSeqBits
	mu    sync.Mutex
}

type streamSeqBits struct {
	present map[uint64]struct{}
	absent  map[uint64]struct{}
}

func newSeqExistsCache() *seqExistsCache {
	return &seqExistsCache{byKey: make(map[string]*streamSeqBits)}
}

func (c *seqExistsCache) known(stream string, seq uint64) (exists bool, known bool) {
	if c == nil {
		return false, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	bits, ok := c.byKey[stream]
	if !ok {
		return false, false
	}
	if _, ok := bits.present[seq]; ok {
		return true, true
	}
	if _, ok := bits.absent[seq]; ok {
		return false, true
	}
	return false, false
}

func (c *seqExistsCache) mark(stream string, seq uint64, exists bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	bits, ok := c.byKey[stream]
	if !ok {
		bits = &streamSeqBits{
			present: make(map[uint64]struct{}),
			absent:  make(map[uint64]struct{}),
		}
		c.byKey[stream] = bits
	}
	if exists {
		bits.present[seq] = struct{}{}
		delete(bits.absent, seq)
	} else {
		bits.absent[seq] = struct{}{}
		delete(bits.present, seq)
	}
	if len(bits.present)+len(bits.absent) > 4096 {
		bits.present = make(map[uint64]struct{})
		bits.absent = make(map[uint64]struct{})
		if exists {
			bits.present[seq] = struct{}{}
		} else {
			bits.absent[seq] = struct{}{}
		}
	}
}
