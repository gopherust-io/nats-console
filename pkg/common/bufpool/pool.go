package bufpool

import (
	"sync"
)

var bytePool = sync.Pool{New: func() any {
	b := make([]byte, 0, 4096)
	return &b
}}

// Get returns a pooled *[]byte reset to length 0. Call Put with the same pointer.
func Get() *[]byte {
	b := bytePool.Get().(*[]byte)
	*b = (*b)[:0]
	return b
}

// Put returns a *[]byte to the pool. Pass the same pointer from Get.
func Put(b *[]byte) {
	if b == nil || cap(*b) > 1<<20 {
		return
	}
	*b = (*b)[:0]
	bytePool.Put(b)
}

// GetBytes is a convenience that returns the slice header; prefer Get/Put
// when the buffer may grow via append so the pooled pointer stays in sync.
func GetBytes() []byte {
	b := bytePool.Get().(*[]byte)
	*b = (*b)[:0]
	return *b
}

// PutBytes returns a buffer to the pool. Prefer Put with the original *[]byte
// when possible; this path re-wraps the slice header for callers that only
// hold []byte.
func PutBytes(b []byte) {
	if cap(b) > 1<<20 {
		return
	}
	b = b[:0]
	bytePool.Put(&b)
}
