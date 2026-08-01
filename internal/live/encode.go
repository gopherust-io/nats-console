package live

import (
	"encoding/base64"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/pkg/common/bufpool"
)

func encodeLiveFrame(frame liveFrame) ([]byte, error) {
	scratch := bufpool.Get()
	defer bufpool.Put(scratch)
	buf := *scratch

	buf = append(buf, `{"type":`...)
	buf = appendJSONString(buf, frame.Type.String())
	if frame.Subject != "" {
		buf = append(buf, `,"subject":`...)
		buf = appendJSONString(buf, frame.Subject)
	}
	if frame.Time != "" {
		buf = append(buf, `,"time":`...)
		buf = appendJSONString(buf, frame.Time)
	}
	if frame.Data != "" {
		buf = append(buf, `,"data":`...)
		buf = appendJSONString(buf, frame.Data)
	}
	if len(frame.Headers) > 0 {
		buf = append(buf, `,"headers":`...)
		buf = appendJSONHeaders(buf, frame.Headers)
	}
	if frame.Error != "" {
		buf = append(buf, `,"error":`...)
		buf = appendJSONString(buf, frame.Error)
	}
	if frame.Seq != 0 {
		buf = append(buf, `,"seq":`...)
		buf = strconv.AppendUint(buf, frame.Seq, 10)
	}
	buf = append(buf, '}')
	*scratch = buf

	out := make([]byte, len(buf))
	copy(out, buf)
	return out, nil
}

// EncodeMessageFrame encodes a live message frame for benchmarks and tests.
func EncodeMessageFrame(seq uint64, subject string, payload []byte, now time.Time, headers map[string]string) ([]byte, error) {
	return encodeMessageFrame(seq, subject, payload, headers, now)
}

func headerMapFromNATS(h nats.Header) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, vals := range h {
		if len(vals) > 0 {
			out[k] = vals[0]
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func encodeMessageFrame(seq uint64, subject string, payload []byte, headers map[string]string, now time.Time) ([]byte, error) {
	scratch := bufpool.Get()
	defer bufpool.Put(scratch)
	buf := *scratch

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
	buf = append(buf, '"')

	if len(headers) > 0 {
		buf = append(buf, `,"headers":`...)
		buf = appendJSONHeaders(buf, headers)
	}

	buf = append(buf, '}')
	*scratch = buf

	out := make([]byte, len(buf))
	copy(out, buf)
	return out, nil
}

func appendJSONHeaders(dst []byte, headers map[string]string) []byte {
	dst = append(dst, '{')
	first := true
	for k, v := range headers {
		if !first {
			dst = append(dst, ',')
		}
		first = false
		dst = appendJSONString(dst, k)
		dst = append(dst, ':')
		dst = appendJSONString(dst, v)
	}
	return append(dst, '}')
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
