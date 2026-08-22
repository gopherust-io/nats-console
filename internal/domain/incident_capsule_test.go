package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIncidentCapsuleCaptureRequestValidate(t *testing.T) {
	t.Parallel()
	require.ErrorIs(t, IncidentCapsuleCaptureRequest{}.Validate(), ErrCapsuleConsumerRequired)
	require.NoError(t, IncidentCapsuleCaptureRequest{Consumer: "worker"}.Validate())
	require.Error(t, IncidentCapsuleCaptureRequest{Consumer: "worker", Window: MaxCapsuleWindow + 1}.Validate())
	assert.Equal(t, DefaultCapsuleWindow, IncidentCapsuleCaptureRequest{Consumer: "w"}.NormalizedWindow())
	assert.Equal(t, 10, IncidentCapsuleCaptureRequest{Consumer: "w", Window: 10}.NormalizedWindow())
}

func TestCapsuleBuckets(t *testing.T) {
	t.Parallel()
	store, index := CapsuleBuckets("", "")
	assert.Equal(t, DefaultIncidentCapsuleBucket, store)
	assert.Equal(t, DefaultIncidentIndexBucket, index)
	store, index = CapsuleBuckets(" custom ", " idx ")
	assert.Equal(t, " custom ", store)
	assert.Equal(t, " idx ", index)
}
