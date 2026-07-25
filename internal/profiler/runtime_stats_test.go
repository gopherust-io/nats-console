package profiler

import (
	"testing"
	"time"
)

func TestReadRuntimeStats(t *testing.T) {
	t.Parallel()
	stats := ReadRuntimeStats()
	if stats.Goroutines < 1 {
		t.Fatalf("goroutines=%d", stats.Goroutines)
	}
	if stats.FetchedAt.IsZero() || time.Since(stats.FetchedAt) > time.Minute {
		t.Fatalf("unexpected FetchedAt %v", stats.FetchedAt)
	}
	if stats.Memory.Sys == 0 {
		t.Fatal("expected non-zero Sys")
	}
}
