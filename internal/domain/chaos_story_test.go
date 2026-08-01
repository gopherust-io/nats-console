package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemoChaosStory(t *testing.T) {
	story := DemoChaosStory()
	require.NotEmpty(t, story.Title)
	require.GreaterOrEqual(t, len(story.Acts), 3)
	assert.Equal(t, ChaosStorySourceDemo, story.Source)
	assert.True(t, story.Demo)
	assert.Contains(t, story.Summary, "Black Friday")
}

func TestBuildChaosStorySeed(t *testing.T) {
	seed := BuildChaosStorySeed([]ChaosStoryInventoryInput{
		{
			Name:      "ORDERS",
			Subjects:  []string{"orders.created", "orders.>"},
			Consumers: []string{"billing"},
		},
		{
			Name:      "PAYMENTS",
			Subjects:  []string{"payments.authorized"},
			Consumers: []string{"ledger"},
		},
	})
	assert.Equal(t, []string{"ORDERS", "PAYMENTS"}, seed.Streams)
	assert.Equal(t, []string{"billing", "ledger"}, seed.Consumers)
	assert.Equal(t, []string{"orders.created", "payments.authorized"}, seed.Subjects)
	assert.NotContains(t, seed.Subjects, "orders.>")
}

func TestFilterChaosStoryTargets(t *testing.T) {
	story := ChaosStory{
		Acts: []ChaosStoryAct{
			{Targets: []string{"ORDERS", "invented", "billing"}},
		},
	}
	seed := ChaosStorySeed{
		Streams:   []string{"ORDERS"},
		Consumers: []string{"billing"},
	}
	got := FilterChaosStoryTargets(story, seed)
	assert.Equal(t, []string{"ORDERS", "billing"}, got.Acts[0].Targets)
}

func TestNextChaosActIndex(t *testing.T) {
	next, done := NextChaosActIndex(0, 3)
	assert.Equal(t, 1, next)
	assert.False(t, done)

	next, done = NextChaosActIndex(2, 3)
	assert.Equal(t, 2, next)
	assert.True(t, done)

	_, done = NextChaosActIndex(0, 0)
	assert.True(t, done)
}
