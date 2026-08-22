package b64util

import (
	"sync"

	"github.com/cloudwego/base64x"
)

var scratchPool = sync.Pool{New: func() any {
	b := make([]byte, 0, 1024)
	return &b
}}

// EncodeToString encodes src using a pooled scratch buffer when possible.
// The returned string owns its bytes (copied); the pool buffer is never aliased.
func EncodeToString(src []byte) string {
	if len(src) == 0 {
		return ""
	}
	n := base64x.StdEncoding.EncodedLen(len(src))
	scratch := scratchPool.Get().(*[]byte)
	buf := *scratch
	if cap(buf) < n {
		buf = make([]byte, n)
	} else {
		buf = buf[:n]
	}
	base64x.StdEncoding.Encode(buf, src)
	out := string(buf) // must copy; BytesToString would alias the pooled scratch
	*scratch = buf[:0]
	scratchPool.Put(scratch)
	return out
}

// Encode writes the standard base64 encoding of src into dst, which must have
// capacity for EncodedLen(len(src)) bytes. Returns EncodedLen(len(src)).
func Encode(dst, src []byte) int {
	n := base64x.StdEncoding.EncodedLen(len(src))
	base64x.StdEncoding.Encode(dst[:n], src)
	return n
}

// EncodedLen returns the length in bytes of the base64 encoding of an input
// buffer of length n.
func EncodedLen(n int) int {
	return base64x.StdEncoding.EncodedLen(n)
}
