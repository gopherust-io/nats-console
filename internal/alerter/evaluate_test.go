package alerter

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestThresholdMet(t *testing.T) {
	t.Parallel()
	assert.True(t, domain.ThresholdMet(domain.AlertComparatorGT, 10, 9))
	assert.False(t, domain.ThresholdMet(domain.AlertComparatorGT, 10, 10))
	assert.True(t, domain.ThresholdMet(domain.AlertComparatorGTE, 10, 10))
	assert.True(t, domain.ThresholdMet(domain.AlertComparatorLT, 1, 2))
	assert.True(t, domain.ThresholdMet(domain.AlertComparatorLTE, 2, 2))
	assert.False(t, domain.ThresholdMet("unknown", 1, 0))
}
