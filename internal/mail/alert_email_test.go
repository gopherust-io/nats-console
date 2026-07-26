package mail

import (
	"context"
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPlaceholderEmail(t *testing.T) {
	t.Parallel()
	assert.True(t, IsPlaceholderEmail(""))
	assert.True(t, IsPlaceholderEmail("admin@local"))
	assert.True(t, IsPlaceholderEmail("  Admin@Local "))
	assert.False(t, IsPlaceholderEmail("ops@example.com"))
}

func TestBuildAlertEmail(t *testing.T) {
	t.Parallel()
	content := BuildAlertEmail(domain.Alert{
		Severity:    domain.AlertSeverityCritical,
		RuleName:    "High CPU",
		Message:     "CPU too high",
		Metric:      domain.MetricServerCPUPercent,
		FiringValue: 95,
		Threshold:   90,
		ClusterID:   "c1",
		FirstSeenAt: time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC),
	}, "prod", "https://consol.example")

	assert.Contains(t, content.Subject, "critical")
	assert.Contains(t, content.Subject, "High CPU")
	assert.Contains(t, content.TextBody, "server.cpu_percent")
	assert.Contains(t, content.TextBody, "https://consol.example/admin/alerts")
	assert.Contains(t, content.HTMLBody, "View alert in console")
	assert.Contains(t, content.HTMLBody, "prod")
	assert.Contains(t, content.HTMLBody, "Critical")
	assert.Contains(t, content.HTMLBody, "#B91C1C")
	assert.Contains(t, content.HTMLBody, "Current value")
}

func TestNopSender(t *testing.T) {
	t.Parallel()
	require.NoError(t, NopSender{}.Send(context.Background(), []string{"a@b.com"}, "s", "t", "h"))
}

func TestNewSenderFromConfigDisabled(t *testing.T) {
	t.Parallel()
	s, err := NewSenderFromConfig(false, SMTPConfig{})
	require.NoError(t, err)
	_, ok := s.(NopSender)
	assert.True(t, ok)
}
