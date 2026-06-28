package live_test

import (
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/live"
)

func BenchmarkEncodeLiveFrameAllocs(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := live.EncodeMessageFrame(42, "events.created", []byte("hello world"), time.Now()); err != nil {
			b.Fatal(err)
		}
	}
}
