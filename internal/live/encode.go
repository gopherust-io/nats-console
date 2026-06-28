package live

import (
	"encoding/base64"
	"time"

	"github.com/bytedance/sonic"

	"github.com/gopherust-io/nats-consol/pkg/common/bufpool"
)

func encodeLiveFrame(frame liveFrame) ([]byte, error) {
	buf := bufpool.GetBuffer()
	defer bufpool.PutBuffer(buf)
	enc := sonic.ConfigDefault.NewEncoder(buf)
	if err := enc.Encode(frame); err != nil {
		return nil, err
	}
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
}

// EncodeMessageFrame encodes a live message frame for benchmarks and tests.
func EncodeMessageFrame(seq uint64, subject string, payload []byte, now time.Time) ([]byte, error) {
	return encodeLiveFrame(messageLiveFrame(seq, subject, payload, now))
}

func messageLiveFrame(seq uint64, subject string, payload []byte, now time.Time) liveFrame {
	return liveFrame{
		Type:    "message",
		Seq:     seq,
		Subject: subject,
		Time:    formatTimeUTC(now),
		Data:    encodePayloadBase64(payload),
	}
}

func formatTimeUTC(t time.Time) string {
	buf := bufpool.GetBytes()
	defer bufpool.PutBytes(buf)
	buf = t.UTC().AppendFormat(buf, time.RFC3339Nano)
	return string(buf)
}

func encodePayloadBase64(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	n := base64.StdEncoding.EncodedLen(len(payload))
	buf := bufpool.GetBytes()
	if cap(buf) < n {
		bufpool.PutBytes(buf)
		buf = make([]byte, n)
	} else {
		buf = buf[:n]
	}
	base64.StdEncoding.Encode(buf, payload)
	out := string(buf)
	bufpool.PutBytes(buf)
	return out
}
