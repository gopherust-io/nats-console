package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsLongRunningProfilePath(t *testing.T) {
	assert.True(t, isLongRunningProfilePath("/debug/pprof/profile"))
	assert.True(t, isLongRunningProfilePath("/api/v1/pprof/profile/cpu"))
	assert.False(t, isLongRunningProfilePath("/api/v1/pprof/runtime"))
	assert.False(t, isLongRunningProfilePath("/api/v1/pprof/continuous"))
}

func TestIsPprofPath(t *testing.T) {
	assert.True(t, isPprofPath("/debug/pprof"))
	assert.True(t, isPprofPath("/debug/pprof/heap"))
	assert.False(t, isPprofPath("/api/v1/pprof/config"))
}
