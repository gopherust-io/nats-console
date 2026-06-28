//go:build integration

package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsHistoryQuery(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)
	ctx := t.Context()

	now := time.Now().UTC().Truncate(time.Second)
	samples := []store.MetricSampleRow{
		{Metric: domain.MetricJetStreamStorageBytes, Value: 1024},
		{Metric: domain.MetricJetStreamMemoryBytes, Value: 512},
		{Metric: domain.MetricJSZMessages, Value: 10},
	}
	require.NoError(t, stack.Store.InsertMetricSamples(ctx, clusterID, now.Add(-5*time.Minute), samples))

	samples[0].Value = 2048
	samples[1].Value = 768
	samples[2].Value = 20
	require.NoError(t, stack.Store.InsertMetricSamples(ctx, clusterID, now, samples))

	from := now.Add(-1 * time.Hour).Format(time.RFC3339)
	to := now.Add(time.Minute).Format(time.RFC3339)
	url := fmt.Sprintf("%s/metrics/history?from=%s&to=%s&metrics=jetstream.storage_bytes,jsz.messages&step=5m",
		srv.BaseURL(clusterID), from, to)

	resp, err := srv.Client.Get(url)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	var history struct {
		ClusterID string `json:"clusterId"`
		Series    []struct {
			Metric string `json:"metric"`
			Points []struct {
				V float64 `json:"v"`
			} `json:"points"`
		} `json:"series"`
	}
	require.NoError(t, sonic.Unmarshal(body, &history))
	assert.Equal(t, clusterID, history.ClusterID)
	assert.Len(t, history.Series, 2)

	for _, series := range history.Series {
		if series.Metric == domain.MetricJetStreamStorageBytes {
			require.NotEmpty(t, series.Points)
			assert.Equal(t, float64(2048), series.Points[len(series.Points)-1].V)
		}
	}
}

func TestMetricsHistoryRetentionCleanup(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := t.Context()
	clusterID := stack.DefaultClusterID(t)

	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	require.NoError(t, stack.Store.InsertMetricSamples(ctx, clusterID, old, []store.MetricSampleRow{
		{Metric: domain.MetricJetStreamStreams, Value: 1},
	}))

	deleted, err := stack.Store.DeleteMetricSamplesOlderThan(ctx, time.Now().UTC().Add(-7*24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
}
