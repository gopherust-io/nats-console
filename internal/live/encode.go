package live

import (
	"encoding/base64"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/bytedance/sonic"

	"github.com/gopherust-io/nats-consol/pkg/common/bufpool"
)

func encodeLiveFrame(frame liveFrame) ([]byte, error) {
	return sonic.Marshal(frame)
}

// EncodeMessageFrame encodes a live message frame for benchmarks and tests.
func EncodeMessageFrame(seq uint64, subject string, payload []byte, now time.Time) ([]byte, error) {
	return encodeMessageFrame(seq, subject, payload, now)
}

func encodeMessageFrame(seq uint64, subject string, payload []byte, now time.Time) ([]byte, error) {
	buf := bufpool.GetBytes()
	defer bufpool.PutBytes(buf)

	buf = append(buf, `{"type":"message","seq":`...)
	buf = strconv.AppendUint(buf, seq, 10)
	buf = append(buf, `,"subject":`...)
	buf = appendJSONString(buf, subject)
	buf = append(buf, `,"time":"`...)
	buf = now.UTC().AppendFormat(buf, time.RFC3339Nano)
	buf = append(buf, `","data":"`...)
	n := base64.StdEncoding.EncodedLen(len(payload))
	start := len(buf)
	buf = growSlice(buf, n)
	base64.StdEncoding.Encode(buf[start:start+n], payload)
	buf = buf[:start+n]
	buf = append(buf, `"}`...)

	out := make([]byte, len(buf))
	copy(out, buf)
	return out, nil
}

func growSlice(buf []byte, n int) []byte {
	if cap(buf)-len(buf) >= n {
		return buf[:len(buf)+n]
	}
	out := make([]byte, len(buf)+n, len(buf)+n+1024)
	copy(out, buf)
	return out
}

func appendJSONString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c >= 0x20 && c != '"' && c != '\\' {
			i++
			continue
		}
		if start < i {
			dst = append(dst, s[start:i]...)
		}
		switch c {
		case '"', '\\':
			dst = append(dst, '\\', c)
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				dst = append(dst, `\uFFFD`...)
				i++
				start = i
				continue
			}
			dst = append(dst, `\u00`...)
			dst = append(dst, hexDigit(c>>4), hexDigit(c&0xf))
		}
		i++
		start = i
	}
	if start < len(s) {
		dst = append(dst, s[start:]...)
	}
	return append(dst, '"')
}

func hexDigit(b byte) byte {
	if b < 10 {
		return '0' + b
	}
	return 'a' + (b - 10)
}
