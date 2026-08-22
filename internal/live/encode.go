package live

import (
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/base64x"
	"github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/pkg/common/bufpool"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func encodeLiveFrame(frame liveFrame) ([]byte, error) {
	scratch := bufpool.Get()
	buf := *scratch

	buf = append(buf, `{"type":`...)
	buf = appendJSONString(buf, frame.Type.String())
	if !commonstrings.IsEmpty(frame.Subject) {
		buf = append(buf, `,"subject":`...)
		buf = appendJSONString(buf, frame.Subject)
	}
	if !commonstrings.IsEmpty(frame.Time) {
		buf = append(buf, `,"time":`...)
		buf = appendJSONString(buf, frame.Time)
	}
	if !commonstrings.IsEmpty(frame.Data) {
		buf = append(buf, `,"data":`...)
		buf = appendJSONString(buf, frame.Data)
	}
	if len(frame.Headers) > 0 {
		buf = append(buf, `,"headers":`...)
		buf = appendJSONHeaders(buf, frame.Headers)
	}
	if !commonstrings.IsEmpty(frame.Error) {
		buf = append(buf, `,"error":`...)
		buf = appendJSONString(buf, frame.Error)
	}
	if frame.Seq != 0 {
		buf = append(buf, `,"seq":`...)
		buf = strconv.AppendUint(buf, frame.Seq, 10)
	}
	buf = append(buf, '}')

	// Take ownership of the pooled buffer so callers get a single allocation
	// without a final memcpy. Replace the pool slot with a fresh small buffer.
	*scratch = make([]byte, 0, 512)
	bufpool.Put(scratch)
	return buf, nil
}

// EncodeMessageFrame encodes a live message frame for benchmarks and tests.
func EncodeMessageFrame(seq uint64, subject string, payload []byte, now time.Time, headers map[string]string) ([]byte, error) {
	return encodeMessageFrame(seq, subject, payload, headers, now)
}

func encodeMessageFrame(seq uint64, subject string, payload []byte, headers map[string]string, now time.Time) ([]byte, error) {
	n := base64x.StdEncoding.EncodedLen(len(payload))
	buf := make([]byte, 0, messageFrameCap(subject, n, headerMapBytesHint(headers)))

	buf = append(buf, `{"type":"message","seq":`...)
	buf = strconv.AppendUint(buf, seq, 10)
	buf = append(buf, `,"subject":`...)
	buf = appendJSONString(buf, subject)
	buf = append(buf, `,"time":"`...)
	buf = now.UTC().AppendFormat(buf, time.RFC3339Nano)
	buf = append(buf, `","data":"`...)
	start := len(buf)
	buf = growSlice(buf, n)
	base64x.StdEncoding.Encode(buf[start:start+n], payload)
	buf = append(buf, '"')

	if len(headers) > 0 {
		buf = append(buf, `,"headers":`...)
		buf = appendJSONHeaders(buf, headers)
	}

	return append(buf, '}'), nil
}

// encodeMessageFrameFromNATS encodes without materializing a header map.
func encodeMessageFrameFromNATS(seq uint64, subject string, payload []byte, headers nats.Header, now time.Time) ([]byte, error) {
	n := base64x.StdEncoding.EncodedLen(len(payload))
	buf := make([]byte, 0, messageFrameCap(subject, n, natsHeaderBytesHint(headers)))

	buf = append(buf, `{"type":"message","seq":`...)
	buf = strconv.AppendUint(buf, seq, 10)
	buf = append(buf, `,"subject":`...)
	buf = appendJSONString(buf, subject)
	buf = append(buf, `,"time":"`...)
	buf = now.UTC().AppendFormat(buf, time.RFC3339Nano)
	buf = append(buf, `","data":"`...)
	start := len(buf)
	buf = growSlice(buf, n)
	base64x.StdEncoding.Encode(buf[start:start+n], payload)
	buf = append(buf, '"')

	if len(headers) > 0 {
		buf = append(buf, `,"headers":`...)
		buf = appendJSONNATSHeaders(buf, headers)
	}

	return append(buf, '}'), nil
}

func growSlice(buf []byte, n int) []byte {
	if cap(buf)-len(buf) >= n {
		return buf[:len(buf)+n]
	}
	out := make([]byte, len(buf)+n, len(buf)+n+1024)
	copy(out, buf)
	return out
}

func messageFrameCap(subject string, b64Len, headersHint int) int {
	// Fixed JSON keys + RFC3339Nano time + quotes/commas, with slack for escaping.
	return 96 + len(subject)*2 + b64Len + headersHint
}

func headerMapBytesHint(headers map[string]string) int {
	if len(headers) == 0 {
		return 0
	}
	n := 16
	for k, v := range headers {
		n += len(k)*2 + len(v)*2 + 8
	}
	return n
}

func natsHeaderBytesHint(headers nats.Header) int {
	if len(headers) == 0 {
		return 0
	}
	n := 16
	for k, vals := range headers {
		if len(vals) == 0 {
			continue
		}
		n += len(k)*2 + len(vals[0])*2 + 8
	}
	return n
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

func appendJSONNATSHeaders(dst []byte, headers nats.Header) []byte {
	dst = append(dst, '{')
	first := true
	for k, vals := range headers {
		if len(vals) == 0 {
			continue
		}
		if !first {
			dst = append(dst, ',')
		}
		first = false
		dst = appendJSONString(dst, k)
		dst = append(dst, ':')
		dst = appendJSONString(dst, vals[0])
	}
	return append(dst, '}')
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
