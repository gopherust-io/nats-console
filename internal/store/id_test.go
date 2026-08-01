package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIDIsUUIDv7(t *testing.T) {
	id, err := uuid.Parse(newID())
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(7), id.Version())
}

func TestPartitionUpperBound(t *testing.T) {
	bound := `FOR VALUES FROM ('2026-07-30 00:00:00+00') TO ('2026-07-31 00:00:00+00')`
	got, ok := partitionUpperBound(bound)
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC), got)

	_, ok = partitionUpperBound("DEFAULT")
	assert.False(t, ok)
}
