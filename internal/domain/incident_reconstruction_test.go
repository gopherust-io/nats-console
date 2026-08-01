package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeIncidentTimelineMockupOrder(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 29, 14, 0, 0, 0, time.UTC)
	from := base
	to := base.Add(30 * time.Minute)

	out := ComputeIncidentTimeline(IncidentReconstructionInput{
		ClusterID: "c1",
		Stream:    "ORDERS",
		Consumer:  "billing-worker",
		From:      from,
		To:        to,
		Annotations: []IncidentAnnotation{
			{OccurredAt: base.Add(2 * time.Minute), Type: "deploy", Title: "Deploy"},
		},
		Samples: []IncidentConsumerSample{
			{CapturedAt: base.Add(3 * time.Minute), StreamName: "ORDERS", ConsumerName: "billing-worker", Lag: 10, NumRedelivered: 1, DeliveredSeq: 100, AckFloorSeq: 99},
			{CapturedAt: base.Add(4 * time.Minute), StreamName: "ORDERS", ConsumerName: "billing-worker", Lag: 200, NumRedelivered: 2, DeliveredSeq: 110, AckFloorSeq: 105},
			{CapturedAt: base.Add(5 * time.Minute), StreamName: "ORDERS", ConsumerName: "billing-worker", Lag: 250, NumRedelivered: 3, DeliveredSeq: 120, AckFloorSeq: 110},
			{CapturedAt: base.Add(6 * time.Minute), StreamName: "ORDERS", ConsumerName: "billing-worker", Lag: 300, NumRedelivered: 40, DeliveredSeq: 130, AckFloorSeq: 115},
			{CapturedAt: base.Add(7 * time.Minute), StreamName: "ORDERS", ConsumerName: "billing-worker", Lag: 400, NumRedelivered: 50, DeliveredSeq: 130, AckFloorSeq: 115},
			{CapturedAt: base.Add(9 * time.Minute), StreamName: "ORDERS", ConsumerName: "billing-worker", Lag: 500, NumRedelivered: 55, DeliveredSeq: 130, AckFloorSeq: 115},
		},
		NodeEvents: []IncidentNodeEvent{
			{OccurredAt: base.Add(8 * time.Minute), NodeName: "Node B", EventType: IncidentNodeDisconnect},
		},
		Audit: []IncidentAuditChange{
			{Timestamp: base.Add(1 * time.Minute), Action: "update", ResourceType: "stream", ResourceName: "ORDERS", Actor: "ci"},
		},
	})

	require.True(t, out.UsedDeploy)
	require.False(t, out.UsedAudit)
	require.GreaterOrEqual(t, out.EventCount, 5)

	labels := make([]string, 0, len(out.Events))
	for _, e := range out.Events {
		labels = append(labels, e.Label)
	}
	assert.Contains(t, labels, "Deploy")
	assert.Contains(t, labels, "Consumer lag grows")
	assert.Contains(t, labels, "Redeliveries spike")
	assert.Contains(t, labels, "Node B disconnects")
	assert.Contains(t, labels, "Processing stops")

	// Chronological order
	for i := 1; i < len(out.Events); i++ {
		assert.False(t, out.Events[i].At.Before(out.Events[i-1].At))
	}
}

func TestComputeIncidentTimelineAuditFallback(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	out := ComputeIncidentTimeline(IncidentReconstructionInput{
		ClusterID: "c1",
		Stream:    "ORDERS",
		Consumer:  "billing-worker",
		From:      base,
		To:        base.Add(time.Hour),
		Audit: []IncidentAuditChange{
			{Timestamp: base.Add(5 * time.Minute), Action: "update", ResourceType: "consumer", ResourceName: "billing-worker", Actor: "alice"},
		},
	})
	require.True(t, out.UsedAudit)
	require.False(t, out.UsedDeploy)
	require.Len(t, out.Events, 1)
	assert.Equal(t, IncidentEventChange, out.Events[0].Category)
	assert.Contains(t, out.Events[0].Label, "update")
}

func TestComputeIncidentTimelineDeployPrecedence(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	out := ComputeIncidentTimeline(IncidentReconstructionInput{
		From: base,
		To:   base.Add(time.Hour),
		Annotations: []IncidentAnnotation{
			{OccurredAt: base.Add(2 * time.Minute), Type: "deploy", Title: "Deploy"},
		},
		Audit: []IncidentAuditChange{
			{Timestamp: base.Add(1 * time.Minute), Action: "update", ResourceType: "stream", ResourceName: "ORDERS"},
		},
	})
	require.True(t, out.UsedDeploy)
	require.False(t, out.UsedAudit)
	for _, e := range out.Events {
		assert.NotEqual(t, IncidentEventChange, e.Category)
	}
}

func TestComputeIncidentTimelineDedupes(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 7, 29, 14, 8, 0, 0, time.UTC)
	out := ComputeIncidentTimeline(IncidentReconstructionInput{
		From: at.Add(-time.Minute),
		To:   at.Add(time.Minute),
		NodeEvents: []IncidentNodeEvent{
			{OccurredAt: at, NodeName: "Node B", EventType: IncidentNodeDisconnect},
			{OccurredAt: at, NodeName: "Node B", EventType: IncidentNodeDisconnect},
		},
	})
	require.Len(t, out.Events, 1)
}

func TestIncidentAnnotationCreateValidate(t *testing.T) {
	t.Parallel()

	assert.Error(t, (IncidentAnnotationCreate{Title: ""}).Validate())
	assert.Error(t, (IncidentAnnotationCreate{Title: "x", Type: "other"}).Validate())
	assert.NoError(t, (IncidentAnnotationCreate{Title: "Deploy"}).Validate())
}
