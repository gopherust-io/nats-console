package profiler_test

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/profiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummarizeHeap(t *testing.T) {
	data, _, err := profiler.CollectNamed("heap")
	require.NoError(t, err)

	summary, err := profiler.Summarize("heap", data, 0)
	require.NoError(t, err)
	assert.Equal(t, "heap", summary.ProfileType)
	assert.NotEmpty(t, summary.Entries)
}
