package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemoArchitectureRefactorPlan(t *testing.T) {
	t.Parallel()
	plan := DemoArchitectureRefactorPlan()
	assert.True(t, plan.Demo)
	assert.Equal(t, "Reduce coupling.", plan.Question)
	require.GreaterOrEqual(t, len(plan.Before.Nodes), 2)
	require.NotEmpty(t, plan.After.Nodes)
	require.GreaterOrEqual(t, len(plan.Steps), 4)
	assert.NotEmpty(t, plan.EventSubject)
	assert.Contains(t, plan.Verdict, "→")
}

func TestBuildArchitectureRefactorPlanChain(t *testing.T) {
	t.Parallel()
	inv := ArchitectureInventory{
		ClusterName: "c",
		Streams: []EventArchitectureInput{
			{
				Name:     "ORDERS",
				Subjects: []string{"orders.created"},
				Consumers: []EventArchitectureConsumerInput{
					{Name: "billing-bridge", FilterSubject: "billing.>"},
				},
			},
			{
				Name:     "BILLING",
				Subjects: []string{"billing.charged"},
				Consumers: []EventArchitectureConsumerInput{
					{Name: "orders-sync", FilterSubject: "orders.>"},
				},
			},
			{
				Name:     "NOTIFY",
				Subjects: []string{"notify.email"},
			},
		},
	}
	plan := BuildArchitectureRefactorPlan(inv, ArchitectureRefactorSeed{Kind: ArchKindCircularDependency, Stream: "ORDERS"})
	require.GreaterOrEqual(t, len(plan.Before.Edges), 1)
	require.NotEmpty(t, plan.After.Edges)
	assert.True(t, len(plan.Steps) >= 5)
}

func TestBuildArchitectureRefactorPlanHealthy(t *testing.T) {
	t.Parallel()
	plan := BuildArchitectureRefactorPlan(ArchitectureInventory{
		Streams: []EventArchitectureInput{{Name: "ONLY", Subjects: []string{"acme.only.created"}}},
	}, ArchitectureRefactorSeed{})
	assert.Contains(t, plan.Verdict, "No strong coupling")
	assert.Len(t, plan.Steps, 2)
}
