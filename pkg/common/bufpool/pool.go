package bufpool

import (
	"bytes"
	"sync"
)

var (
	bytePool = sync.Pool{New: func() any {
		b := make([]byte, 0, 4096)
		return &b
	}}
	bufferPool = sync.Pool{New: func() any {
		return new(bytes.Buffer)
	}}
)

func GetBytes() []byte {
	b := bytePool.Get().(*[]byte)
	*b = (*b)[:0]
	return *b
}

func PutBytes(b []byte) {
	if cap(b) > 1<<20 {
		return
	}
	bytePool.Put(&b)
}

func GetBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func PutBuffer(buf *bytes.Buffer) {
	if buf.Cap() > 1<<20 {
		return
	}
	bufferPool.Put(buf)
}
