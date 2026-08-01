package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

func TestValidateAlertRuleCreate(t *testing.T) {
	t.Parallel()
	err := validateAlertRuleCreate(domain.AlertRuleCreate{
		Name:       "High CPU",
		Metric:     domain.MetricServerCPUPercent,
		Comparator: domain.AlertComparatorGTE,
		Severity:   domain.AlertSeverityWarning,
		Threshold:  90,
	})
	require.NoError(t, err)

	err = validateAlertRuleCreate(domain.AlertRuleCreate{
		Name:       "Bad",
		Metric:     "not.a.metric",
		Comparator: domain.AlertComparatorGTE,
		Severity:   domain.AlertSeverityWarning,
	})
	assert.Error(t, err)
}
