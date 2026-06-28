package profiler_test

import (
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/profiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectorSamplesProfiles(t *testing.T) {
	c := profiler.Start(profiler.Options{
		Interval: 30 * time.Second,
		CPUSlice: 1 * time.Second,
	})
	t.Cleanup(c.Stop)

	require.Eventually(t, func() bool {
		snapshot := c.Snapshot()
		_, ok := snapshot.Profiles["heap"]
		return ok && len(snapshot.RuntimeHistory) > 0
	}, 10*time.Second, 100*time.Millisecond)

	snapshot := c.Snapshot()
	assert.Contains(t, snapshot.Profiles, "heap")
	assert.Contains(t, snapshot.Profiles, "goroutine")
	assert.NotEmpty(t, snapshot.RuntimeHistory)
}

func TestSummarizeHeap(t *testing.T) {
	data, _, err := profiler.CollectNamed("heap")
	require.NoError(t, err)

	summary, err := profiler.Summarize("heap", data, 0)
	require.NoError(t, err)
	assert.Equal(t, "heap", summary.ProfileType)
	assert.NotEmpty(t, summary.Entries)
}
