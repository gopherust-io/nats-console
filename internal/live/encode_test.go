package live_test

import (
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/live"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"

	"github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkEncodeLiveFrameAllocs(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := live.EncodeMessageFrame(42, "events.created", strings.StringToBytes("hello world"), time.Now(), nil); err != nil {
			b.Fatal(err)
		}
	}
}

func TestEncodeMessageFrameJSON(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 123456789, time.UTC)
	raw, err := live.EncodeMessageFrame(42, `events."created"`, strings.StringToBytes("hello world"), now, nil)
	require.NoError(t, err)

	var frame struct {
		Type    string `json:"type"`
		Subject string `json:"subject"`
		Time    string `json:"time"`
		Data    string `json:"data"`
		Seq     uint64 `json:"seq"`
	}
	require.NoError(t, serializer.Unmarshal(raw, &frame))
	assert.Equal(t, "message", frame.Type)
	assert.Equal(t, uint64(42), frame.Seq)
	assert.Equal(t, `events."created"`, frame.Subject)
	assert.Equal(t, "aGVsbG8gd29ybGQ=", frame.Data)
	assert.Equal(t, now.Format(time.RFC3339Nano), frame.Time)
}

func TestEncodeMessageFrameWithHeaders(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	raw, err := live.EncodeMessageFrame(7, "events.created", strings.StringToBytes("{}"), now, map[string]string{
		"Content-Type": "application/json",
		"X-Trace":      "abc\"def",
	})
	require.NoError(t, err)

	var frame struct {
		Headers map[string]string `json:"headers"`
		Type    string            `json:"type"`
		Data    string            `json:"data"`
		Seq     uint64            `json:"seq"`
	}
	require.NoError(t, serializer.Unmarshal(raw, &frame))
	assert.Equal(t, "message", frame.Type)
	assert.Equal(t, uint64(7), frame.Seq)
	assert.Equal(t, "e30=", frame.Data)
	assert.Equal(t, "application/json", frame.Headers["Content-Type"])
	assert.Equal(t, `abc"def`, frame.Headers["X-Trace"])
}

func TestEncodeMessageFrameOmitsEmptyHeaders(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	raw, err := live.EncodeMessageFrame(1, "s", nil, now, nil)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"headers"`)
}
