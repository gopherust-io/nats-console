package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventGenomeKeyPromptExamples(t *testing.T) {
	t.Parallel()

	want := "order.created"
	assert.Equal(t, want, EventGenomeKey("orders.created"))
	assert.Equal(t, want, EventGenomeKey("orders.new"))
	assert.Equal(t, want, EventGenomeKey("order.created"))
	assert.Equal(t, want, EventGenomeKey("Orders.Added"))
	assert.Equal(t, want, EventGenomeKey("orderCreated"))
	assert.Equal(t, "order.shipped", EventGenomeKey("orders.shipped"))
	assert.Equal(t, "order.updated", EventGenomeKey("orders.changed"))
	assert.Equal(t, "order.deleted", EventGenomeKey("orders.removed"))
}

func TestAnalyzeEventGenomePromptCluster(t *testing.T) {
	t.Parallel()

	snap := AnalyzeEventGenome([]EventGenomeInput{{
		Name: "ORDERS",
		Subjects: []string{
			"orders.created",
			"orders.new",
			"order.created",
			"orders.shipped",
		},
	}})

	require.Equal(t, 1, snap.Totals.Clusters)
	require.GreaterOrEqual(t, snap.Totals.Duplicates, 2)
	assert.Equal(t, snap.Totals.Total, len(snap.Findings))
	assert.Equal(t, snap.Totals.Duplicates, len(snap.Findings))

	var cluster []string
	var suggested string
	for _, f := range snap.Findings {
		assert.Equal(t, "order.created", f.Genome)
		assert.NotEqual(t, "orders.shipped", f.Subject)
		cluster = f.Cluster
		suggested = f.Suggested
	}
	require.NotEmpty(t, cluster)
	assert.Contains(t, cluster, "orders.created")
	assert.Contains(t, cluster, "orders.new")
	assert.Contains(t, cluster, "order.created")
	assert.NotContains(t, cluster, "orders.shipped")
	assert.True(t, strings.HasSuffix(suggested, ".created") || suggested == "created")
	assert.Contains(t, suggested, "order")
}

func TestAnalyzeEventGenomeActionSynonymReason(t *testing.T) {
	t.Parallel()

	snap := AnalyzeEventGenome([]EventGenomeInput{{
		Name:     "ORDERS",
		Subjects: []string{"orders.created", "orders.added"},
	}})

	require.NotEmpty(t, snap.Findings)
	foundSynonym := false
	for _, f := range snap.Findings {
		if f.Subject == "orders.added" {
			assert.Contains(t, f.Reasons, EventGenomeReasonActionSynonym)
			foundSynonym = true
		}
	}
	assert.True(t, foundSynonym)
}

func TestAnalyzeEventGenomeNoClusterForSingleton(t *testing.T) {
	t.Parallel()

	snap := AnalyzeEventGenome([]EventGenomeInput{{
		Name:     "ORDERS",
		Subjects: []string{"orders.shipped", "billing.invoice.paid"},
	}})

	assert.Equal(t, 0, snap.Totals.Clusters)
	assert.Empty(t, snap.Findings)
}

func TestAnalyzeEventGenomeSkipsWildcards(t *testing.T) {
	t.Parallel()

	snap := AnalyzeEventGenome([]EventGenomeInput{{
		Name:     "ORDERS",
		Subjects: []string{"orders.>", "orders.*", "orders.created", "orders.new"},
	}})

	require.Equal(t, 1, snap.Totals.Clusters)
	for _, f := range snap.Findings {
		assert.NotContains(t, f.Subject, ">")
		assert.NotContains(t, f.Subject, "*")
	}
}
