package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBehaviorFingerprintKVKey(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "ORDERS/billing-worker", BehaviorFingerprintKVKey("ORDERS", "billing-worker"))
	assert.Equal(t, "ORDERS/billing-worker", BehaviorFingerprintKVKey(" ORDERS ", " billing-worker "))
}

func TestParseBehaviorFingerprintKV(t *testing.T) {
	t.Parallel()

	assert.False(t, ParseBehaviorFingerprintKV(nil, "ORDERS", "billing-worker").Available)
	assert.False(t, ParseBehaviorFingerprintKV(strings.StringToBytes("not-json"), "ORDERS", "billing-worker").Available)
	assert.False(t, ParseBehaviorFingerprintKV(strings.StringToBytes(`{"anomaly":true}`), "ORDERS", "billing-worker").Available)

	updated := time.Date(2026, 7, 29, 6, 0, 0, 0, time.UTC)
	raw, err := json.Marshal(map[string]any{
		"stream":  "ORDERS",
		"durable": "billing-worker",
		"anomaly": true,
		"normal": map[string]any{
			"msgPerMin":    1000,
			"processingMs": 200,
		},
		"current": map[string]any{
			"msgPerMin":    1000,
			"processingMs": 2400,
		},
		"sustainedForMs": 30000,
		"updatedAt":      updated,
	})
	require.NoError(t, err)

	got := ParseBehaviorFingerprintKV(raw, "", "")
	require.True(t, got.Available)
	assert.Equal(t, "ORDERS", got.Stream)
	assert.Equal(t, "billing-worker", got.Durable)
	assert.True(t, got.Anomaly)
	require.NotNil(t, got.Normal)
	require.NotNil(t, got.Current)
	assert.Equal(t, float64(1000), got.Normal.MsgPerMin)
	assert.Equal(t, float64(200), got.Normal.ProcessingMs)
	assert.Equal(t, float64(1000), got.Current.MsgPerMin)
	assert.Equal(t, float64(2400), got.Current.ProcessingMs)
	assert.Equal(t, int64(30000), got.SustainedForMs)
	require.NotNil(t, got.UpdatedAt)
	assert.True(t, got.UpdatedAt.Equal(updated))
}

func TestParseBehaviorFingerprintKVFallbackIdentity(t *testing.T) {
	t.Parallel()
	raw := strings.StringToBytes(`{"anomaly":false,"normal":{"msgPerMin":10,"processingMs":50},"current":{"msgPerMin":11,"processingMs":55}}`)
	got := ParseBehaviorFingerprintKV(raw, "S", "c")
	require.True(t, got.Available)
	assert.Equal(t, "S", got.Stream)
	assert.Equal(t, "c", got.Durable)
	assert.False(t, got.Anomaly)
}
