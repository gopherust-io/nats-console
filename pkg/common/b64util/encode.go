package b64util

import (
	"encoding/base64"
	"sync"
)

var scratchPool = sync.Pool{New: func() any {
	b := make([]byte, 0, 1024)
	return &b
}}

// EncodeToString encodes src using a pooled scratch buffer when possible.
func EncodeToString(src []byte) string {
	if len(src) == 0 {
		return ""
	}
	n := base64.StdEncoding.EncodedLen(len(src))
	scratch := scratchPool.Get().(*[]byte)
	buf := *scratch
	if cap(buf) < n {
		buf = make([]byte, n)
	} else {
		buf = buf[:n]
	}
	base64.StdEncoding.Encode(buf, src)
	out := string(buf)
	if cap(*scratch) >= n {
		*scratch = (*scratch)[:0]
		scratchPool.Put(scratch)
	}
	return out
}
