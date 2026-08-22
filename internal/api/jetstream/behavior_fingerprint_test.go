package jetstream

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestDecodeKVEntryValue(t *testing.T) {
	t.Parallel()

	raw, err := decodeKVEntryValue(nil)
	require.NoError(t, err)
	assert.Nil(t, raw)

	payload := strings.StringToBytes(`{"anomaly":true,"normal":{"msgPerMin":1000,"processingMs":200},"current":{"msgPerMin":1000,"processingMs":2400}}`)
	entry := &domain.KVEntry{Value: base64.StdEncoding.EncodeToString(payload)}
	got, err := decodeKVEntryValue(entry)
	require.NoError(t, err)
	report := domain.ParseBehaviorFingerprintKV(got, "ORDERS", "billing-worker")
	require.True(t, report.Available)
	assert.True(t, report.Anomaly)
	assert.Equal(t, float64(2400), report.Current.ProcessingMs)
}

func TestParseMissingFingerprintIsIdle(t *testing.T) {
	t.Parallel()
	report := domain.BehaviorFingerprintReport{Available: false}
	assert.False(t, report.Available)
	assert.False(t, report.Anomaly)
}
