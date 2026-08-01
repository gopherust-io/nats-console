package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeZombiesEmptyStreamAndUnpublishedSubjects(t *testing.T) {
	t.Parallel()

	snap := AnalyzeZombies([]ZombieStreamInput{{
		Name:     "ORDERS",
		Subjects: []string{"orders.>", "metrics.tick"},
		Messages: 0,
		LastSeq:  0,
	}})

	require.NotEmpty(t, snap.Findings)
	assert.Equal(t, 1, snap.Totals.EmptyStreams)
	assert.Equal(t, 2, snap.Totals.UnpublishedSubjects)

	kinds := map[string]int{}
	for _, f := range snap.Findings {
		kinds[f.Kind]++
		if f.Kind == ZombieKindEmptyStream {
			assert.Equal(t, "ORDERS", f.Stream)
			assert.Contains(t, f.Reasons, ZombieReasonNeverReceived)
		}
	}
	assert.Equal(t, 1, kinds[ZombieKindEmptyStream])
	assert.Equal(t, 2, kinds[ZombieKindUnpublishedSubject])
}

func TestAnalyzeZombiesIdleConsumerSkippedWhenStreamEmpty(t *testing.T) {
	t.Parallel()

	snap := AnalyzeZombies([]ZombieStreamInput{{
		Name:     "EMPTY",
		Subjects: []string{"x"},
		Messages: 0,
		LastSeq:  0,
		Consumers: []ZombieConsumerInput{{
			Name:             "worker",
			DeliveredConsSeq: 0,
		}},
	}})

	for _, f := range snap.Findings {
		assert.NotEqual(t, ZombieKindIdleConsumer, f.Kind)
	}
	assert.Equal(t, 0, snap.Totals.IdleConsumers)
}

func TestAnalyzeZombiesIdleConsumerOnActiveStream(t *testing.T) {
	t.Parallel()

	snap := AnalyzeZombies([]ZombieStreamInput{{
		Name:     "ORDERS",
		Subjects: []string{"orders.>"},
		Messages: 10,
		LastSeq:  10,
		Consumers: []ZombieConsumerInput{{
			Name:             "idle-worker",
			FilterSubject:    "orders.>",
			DeliveredConsSeq: 0,
		}},
	}})

	require.Equal(t, 1, snap.Totals.IdleConsumers)
	f := snap.Findings[0]
	for _, x := range snap.Findings {
		if x.Kind == ZombieKindIdleConsumer {
			f = x
			break
		}
	}
	assert.Equal(t, ZombieKindIdleConsumer, f.Kind)
	assert.Equal(t, "idle-worker", f.Consumer)
	assert.Contains(t, f.Reasons, ZombieReasonZeroDelivered)
}

func TestAnalyzeZombiesUnconsumedSubject(t *testing.T) {
	t.Parallel()

	snap := AnalyzeZombies([]ZombieStreamInput{{
		Name:     "MIXED",
		Subjects: []string{"orders.>", "events.>"},
		Messages: 5,
		LastSeq:  5,
		Consumers: []ZombieConsumerInput{{
			Name:             "orders-only",
			FilterSubject:    "orders.>",
			DeliveredConsSeq: 3,
		}},
	}})

	require.Equal(t, 1, snap.Totals.UnconsumedSubjects)
	var found *ZombieFinding
	for i := range snap.Findings {
		if snap.Findings[i].Kind == ZombieKindUnconsumedSubject {
			found = &snap.Findings[i]
			break
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, "events.>", found.Subject)
}

func TestAnalyzeZombiesWildcardFilterCoversAll(t *testing.T) {
	t.Parallel()

	snap := AnalyzeZombies([]ZombieStreamInput{{
		Name:     "ALL",
		Subjects: []string{"orders.>", "events.>"},
		Messages: 5,
		LastSeq:  5,
		Consumers: []ZombieConsumerInput{{
			Name:             "catch-all",
			DeliveredConsSeq: 5,
		}},
	}})

	assert.Equal(t, 0, snap.Totals.UnconsumedSubjects)
}

func TestAnalyzeZombiesUnboundConsumer(t *testing.T) {
	t.Parallel()

	snap := AnalyzeZombies([]ZombieStreamInput{{
		Name:     "ORDERS",
		Subjects: []string{"orders.>"},
		Messages: 2,
		LastSeq:  2,
		Consumers: []ZombieConsumerInput{{
			Name:             "wrong-filter",
			FilterSubject:    "payments.>",
			DeliveredConsSeq: 1,
		}},
	}})

	require.Equal(t, 1, snap.Totals.UnboundConsumers)
	var found *ZombieFinding
	for i := range snap.Findings {
		if snap.Findings[i].Kind == ZombieKindUnboundConsumer {
			found = &snap.Findings[i]
			break
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, "payments.>", found.Subject)
	assert.Equal(t, "wrong-filter", found.Consumer)
}

func TestSubjectsOverlap(t *testing.T) {
	t.Parallel()

	assert.True(t, subjectsOverlap("orders.create", "orders.>"))
	assert.True(t, subjectsOverlap("orders.>", "orders.create"))
	assert.True(t, subjectsOverlap("orders.*", "orders.create"))
	assert.False(t, subjectsOverlap("orders.>", "events.>"))
}
