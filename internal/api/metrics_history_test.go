package api

import (
	"testing"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCounterRatesUsesElapsedSeconds(t *testing.T) {
	t0 := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	points := []domain.MetricPoint{
		{T: t0, V: 100},
		{T: t0.Add(time.Minute), V: 700},   // +600 over 60s → 10/s
		{T: t0.Add(2 * time.Minute), V: 1300}, // +600 over 60s → 10/s
	}

	rates := counterRates(points, time.Minute)
	require.Len(t, rates, 2)
	assert.Equal(t, t0.Add(time.Minute), rates[0].T)
	assert.InDelta(t, 10, rates[0].V, 1e-9)
	assert.InDelta(t, 10, rates[1].V, 1e-9)
}

func TestCounterRatesFallsBackToStep(t *testing.T) {
	t0 := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	// Identical timestamps → use step duration.
	points := []domain.MetricPoint{
		{T: t0, V: 0},
		{T: t0, V: 300},
	}

	rates := counterRates(points, 5*time.Minute)
	require.Len(t, rates, 1)
	assert.InDelta(t, 1, rates[0].V, 1e-9) // 300 / 300s
}

func TestCounterRatesClampsNegativeDelta(t *testing.T) {
	t0 := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	points := []domain.MetricPoint{
		{T: t0, V: 1000},
		{T: t0.Add(time.Minute), V: 100}, // counter reset
	}

	rates := counterRates(points, time.Minute)
	require.Len(t, rates, 1)
	assert.Equal(t, float64(0), rates[0].V)
}
