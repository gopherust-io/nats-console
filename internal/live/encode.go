package live

import (
	"time"

	"github.com/bytedance/sonic"

	"github.com/gopherust-io/nats-consol/pkg/common/b64util"
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
		Data:    b64util.EncodeToString(payload),
	}
}

func formatTimeUTC(t time.Time) string {
	buf := bufpool.GetBytes()
	defer bufpool.PutBytes(buf)
	buf = t.UTC().AppendFormat(buf, time.RFC3339Nano)
	return string(buf)
}
