package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeEventArchitectureTooManyConsumers(t *testing.T) {
	t.Parallel()

	consumers := make([]EventArchitectureConsumerInput, 0, 9)
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		consumers = append(consumers, EventArchitectureConsumerInput{Name: name, FilterSubject: "orders.created"})
	}
	snap := AnalyzeEventArchitecture([]EventArchitectureInput{{
		Name:      "ORDERS",
		Subjects:  []string{"orders.created"},
		Consumers: consumers,
	}})

	require.NotEmpty(t, snap.Problems)
	found := false
	for _, p := range snap.Problems {
		if p.Kind == ArchKindTooManyConsumers && p.Subject == "orders.created" {
			found = true
			assert.Equal(t, ArchSeverityWarn, p.Severity)
		}
	}
	assert.True(t, found)
	assert.NotEqual(t, ArchVerdictHealthy, snap.Verdict)
	assert.NotEmpty(t, snap.Suggestions)
}

func TestAnalyzeEventArchitectureCircular(t *testing.T) {
	t.Parallel()

	snap := AnalyzeEventArchitecture([]EventArchitectureInput{
		{
			Name:     "A",
			Subjects: []string{"a.event"},
			Consumers: []EventArchitectureConsumerInput{
				{Name: "from-b", FilterSubject: "b.>"},
			},
		},
		{
			Name:     "B",
			Subjects: []string{"b.event"},
			Consumers: []EventArchitectureConsumerInput{
				{Name: "from-a", FilterSubject: "a.>"},
			},
		},
	})

	found := false
	for _, p := range snap.Problems {
		if p.Kind == ArchKindCircularDependency {
			found = true
			assert.Equal(t, ArchSeverityCritical, p.Severity)
		}
	}
	assert.True(t, found)
	assert.Equal(t, ArchVerdictAtRisk, snap.Verdict)
}

func TestAnalyzeEventArchitecturePayload(t *testing.T) {
	t.Parallel()

	snap := AnalyzeEventArchitecture([]EventArchitectureInput{{
		Name:     "BIG",
		Subjects: []string{"big.blob"},
		Messages: 10,
		Bytes:    10 * 300 * 1024,
	}})

	found := false
	for _, p := range snap.Problems {
		if p.Kind == ArchKindPayloadTooLarge && p.Stream == "BIG" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestAnalyzeEventArchitectureHealthy(t *testing.T) {
	t.Parallel()

	snap := AnalyzeEventArchitecture([]EventArchitectureInput{{
		Name:     "CLEAN",
		Subjects: []string{"acme.clean.created"},
		Messages: 10,
		Bytes:    1000,
		Consumers: []EventArchitectureConsumerInput{
			{Name: "one", FilterSubject: "acme.clean.created"},
		},
	}})

	assert.Equal(t, ArchVerdictHealthy, snap.Verdict)
	assert.Empty(t, snap.Problems)
}

func TestNatsSubjectMatch(t *testing.T) {
	t.Parallel()
	assert.True(t, natsSubjectMatch("orders.>", "orders.created"))
	assert.True(t, natsSubjectMatch("orders.*", "orders.created"))
	assert.False(t, natsSubjectMatch("orders.*", "orders.created.extra"))
	assert.True(t, subjectIntersects("orders.created", ">"))
}

func TestDemoEventArchitectureSnapshot(t *testing.T) {
	t.Parallel()
	snap := DemoEventArchitectureSnapshot()
	assert.True(t, snap.Demo)
	assert.NotEqual(t, ArchVerdictHealthy, snap.Verdict)
	assert.NotEmpty(t, snap.Problems)
}
