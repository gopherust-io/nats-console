package domain_test

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplayConsumerRequestValidate(t *testing.T) {
	t.Parallel()

	err := (domain.ReplayConsumerRequest{From: "seq"}).Validate()
	require.Error(t, err)

	err = (domain.ReplayConsumerRequest{From: "seq", Seq: 1}).Validate()
	require.NoError(t, err)

	err = (domain.ReplayConsumerRequest{Mode: "sidecar", From: "beginning"}).Validate()
	require.NoError(t, err)

	err = (domain.ReplayConsumerRequest{From: "time", Time: "2026-01-02T03:04:05Z"}).Validate()
	require.NoError(t, err)

	err = (domain.ReplayConsumerRequest{From: "bogus"}).Validate()
	require.Error(t, err)

	assert.Equal(t, domain.ReplayModeReset, (domain.ReplayConsumerRequest{From: "new"}).NormalizedMode())
}
