package live_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/live"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkEncodeLiveFrameAllocs(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := live.EncodeMessageFrame(42, "events.created", []byte("hello world"), time.Now()); err != nil {
			b.Fatal(err)
		}
	}
}

func TestEncodeMessageFrameJSON(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 123456789, time.UTC)
	raw, err := live.EncodeMessageFrame(42, `events."created"`, []byte("hello world"), now)
	require.NoError(t, err)

	var frame struct {
		Type    string `json:"type"`
		Subject string `json:"subject"`
		Time    string `json:"time"`
		Data    string `json:"data"`
		Seq     uint64 `json:"seq"`
	}
	require.NoError(t, json.Unmarshal(raw, &frame))
	assert.Equal(t, "message", frame.Type)
	assert.Equal(t, uint64(42), frame.Seq)
	assert.Equal(t, `events."created"`, frame.Subject)
	assert.Equal(t, "aGVsbG8gd29ybGQ=", frame.Data)
	assert.Equal(t, now.Format(time.RFC3339Nano), frame.Time)
}
