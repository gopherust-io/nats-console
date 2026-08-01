package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestIncidentReconstructionResponseShape(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 29, 14, 0, 0, 0, time.UTC)
	out := domain.ComputeIncidentTimeline(domain.IncidentReconstructionInput{
		ClusterID: "11111111-1111-1111-1111-111111111111",
		Stream:    "ORDERS",
		Consumer:  "billing-worker",
		From:      base,
		To:        base.Add(30 * time.Minute),
		Annotations: []domain.IncidentAnnotation{
			{OccurredAt: base.Add(2 * time.Minute), Type: "deploy", Title: "Deploy"},
		},
		Samples: []domain.IncidentConsumerSample{
			{CapturedAt: base.Add(3 * time.Minute), StreamName: "ORDERS", ConsumerName: "billing-worker", Lag: 10, NumRedelivered: 1, DeliveredSeq: 100, AckFloorSeq: 99},
			{CapturedAt: base.Add(4 * time.Minute), StreamName: "ORDERS", ConsumerName: "billing-worker", Lag: 200, NumRedelivered: 2, DeliveredSeq: 110, AckFloorSeq: 105},
			{CapturedAt: base.Add(6 * time.Minute), StreamName: "ORDERS", ConsumerName: "billing-worker", Lag: 220, NumRedelivered: 30, DeliveredSeq: 120, AckFloorSeq: 110},
			{CapturedAt: base.Add(7 * time.Minute), StreamName: "ORDERS", ConsumerName: "billing-worker", Lag: 300, NumRedelivered: 35, DeliveredSeq: 120, AckFloorSeq: 110},
			{CapturedAt: base.Add(9 * time.Minute), StreamName: "ORDERS", ConsumerName: "billing-worker", Lag: 400, NumRedelivered: 40, DeliveredSeq: 120, AckFloorSeq: 110},
		},
		NodeEvents: []domain.IncidentNodeEvent{
			{OccurredAt: base.Add(8 * time.Minute), NodeName: "Node B", EventType: domain.IncidentNodeDisconnect},
		},
	})

	raw, err := serializer.Marshal(out)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, serializer.Unmarshal(raw, &decoded))
	assert.Equal(t, "ORDERS", decoded["stream"])
	assert.Equal(t, "billing-worker", decoded["consumer"])
	assert.Equal(t, true, decoded["usedDeployAnnotations"])

	events, ok := decoded["events"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, events)
	first, ok := events[0].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, first, "at")
	assert.Contains(t, first, "category")
	assert.Contains(t, first, "label")
}

func TestIncidentAnnotationCreateJSON(t *testing.T) {
	t.Parallel()

	raw := commonstrings.StringToBytes(`{"type":"deploy","title":"Deploy","details":"v1.2.3","occurredAt":"2026-07-29T14:02:00Z"}`)
	var req domain.IncidentAnnotationCreate
	require.NoError(t, serializer.Unmarshal(raw, &req))
	require.NoError(t, req.Validate())
	assert.Equal(t, "Deploy", req.Title)
	require.NotNil(t, req.OccurredAt)
	assert.Equal(t, 14, req.OccurredAt.UTC().Hour())
}
