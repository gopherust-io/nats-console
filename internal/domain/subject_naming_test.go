package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSubject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want string
	}{
		{"orderCreated", "order.created"},
		{"Orders.Created", "orders.created"},
		{"order.created", "order.created"},
		{"orders.created", "orders.created"},
		{"orders_order_created", "orders.order.created"},
		{"orders-order-created", "orders.order.created"},
		{"XMLParser", "xml.parser"},
		{"", ""},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, NormalizeSubject(tc.in), "in=%q", tc.in)
	}
}

func TestSubjectFingerprintGroupsVariants(t *testing.T) {
	t.Parallel()

	fp := SubjectFingerprint("orders.created")
	assert.Equal(t, fp, SubjectFingerprint("order.created"))
	assert.Equal(t, fp, SubjectFingerprint("Orders.Created"))
	assert.Equal(t, fp, SubjectFingerprint("orderCreated"))
}

func TestAnalyzeSubjectNamingFourExamples(t *testing.T) {
	t.Parallel()

	snap := AnalyzeSubjectNaming([]SubjectNamingInput{{
		Name: "ORDERS",
		Subjects: []string{
			"order.created",
			"orders.created",
			"Orders.Created",
			"orderCreated",
		},
	}})

	require.NotEmpty(t, snap.Findings)

	byKind := map[string][]SubjectNamingFinding{}
	for _, f := range snap.Findings {
		byKind[f.Kind] = append(byKind[f.Kind], f)
	}

	assert.NotEmpty(t, byKind[SubjectNamingKindWrongCase])
	assert.NotEmpty(t, byKind[SubjectNamingKindMissingDots])
	assert.NotEmpty(t, byKind[SubjectNamingKindShallowHierarchy])
	assert.NotEmpty(t, byKind[SubjectNamingKindInconsistentVariant])

	// Cluster suggestion should expand to domain.entity.action with plural domain.
	var variant *SubjectNamingFinding
	for i := range snap.Findings {
		if snap.Findings[i].Kind == SubjectNamingKindInconsistentVariant {
			variant = &snap.Findings[i]
			break
		}
	}
	require.NotNil(t, variant)
	assert.Equal(t, "orders.order.created", variant.Suggested)
	assert.Contains(t, variant.Cluster, "order.created")
	assert.Contains(t, variant.Cluster, "orders.created")
	assert.Contains(t, variant.Cluster, "Orders.Created")
	assert.Contains(t, variant.Cluster, "orderCreated")

	assert.Greater(t, snap.Totals.Total, 0)
	assert.Equal(t, snap.Totals.Total, len(snap.Findings))
}

func TestAnalyzeSubjectNamingNonDotSeparator(t *testing.T) {
	t.Parallel()

	snap := AnalyzeSubjectNaming([]SubjectNamingInput{{
		Name:     "PAY",
		Subjects: []string{"payments_transfer_settled"},
	}})

	found := false
	for _, f := range snap.Findings {
		if f.Kind == SubjectNamingKindNonDotSeparator {
			found = true
			assert.Equal(t, "payments.transfer.settled", f.Suggested)
		}
	}
	assert.True(t, found)
}

func TestAnalyzeSubjectNamingSkipsWildcards(t *testing.T) {
	t.Parallel()

	snap := AnalyzeSubjectNaming([]SubjectNamingInput{{
		Name:     "ORDERS",
		Subjects: []string{"orders.>", "orders.*.created"},
		Consumers: []SubjectNamingConsumerInput{{
			Name:          "worker",
			FilterSubject: "orders.>",
		}},
	}})

	assert.Empty(t, snap.Findings)
}

func TestAnalyzeSubjectNamingConsumerFilters(t *testing.T) {
	t.Parallel()

	snap := AnalyzeSubjectNaming([]SubjectNamingInput{{
		Name:     "ORDERS",
		Subjects: []string{"orders.>"},
		Consumers: []SubjectNamingConsumerInput{{
			Name:           "worker",
			FilterSubjects: []string{"OrderCreated"},
		}},
	}})

	require.NotEmpty(t, snap.Findings)
	foundMissing := false
	foundShallow := false
	for _, f := range snap.Findings {
		if f.Consumer != "worker" || f.Subject != "OrderCreated" {
			continue
		}
		switch f.Kind {
		case SubjectNamingKindMissingDots:
			foundMissing = true
			assert.Equal(t, "order.created", f.Suggested)
		case SubjectNamingKindShallowHierarchy:
			foundShallow = true
			assert.Equal(t, "order.order.created", f.Suggested)
		}
	}
	assert.True(t, foundMissing)
	assert.True(t, foundShallow)
}

func TestAnalyzeSubjectNamingCompliantSubject(t *testing.T) {
	t.Parallel()

	snap := AnalyzeSubjectNaming([]SubjectNamingInput{{
		Name:     "ORDERS",
		Subjects: []string{"orders.order.created"},
	}})

	assert.Empty(t, snap.Findings)
	assert.Equal(t, 0, snap.Totals.Total)
}

func TestExpandShallowViaLint(t *testing.T) {
	t.Parallel()

	snap := AnalyzeSubjectNaming([]SubjectNamingInput{{
		Name:     "ORDERS",
		Subjects: []string{"orders.created"},
	}})

	var shallow *SubjectNamingFinding
	for i := range snap.Findings {
		if snap.Findings[i].Kind == SubjectNamingKindShallowHierarchy {
			shallow = &snap.Findings[i]
			break
		}
	}
	require.NotNil(t, shallow)
	assert.Equal(t, "orders.order.created", shallow.Suggested)
}

func TestExpandShallowDomainEntity(t *testing.T) {
	t.Parallel()

	snap := AnalyzeSubjectNaming([]SubjectNamingInput{{
		Name:     "BILLING",
		Subjects: []string{"billing.invoice"},
	}})

	var shallow *SubjectNamingFinding
	for i := range snap.Findings {
		if snap.Findings[i].Kind == SubjectNamingKindShallowHierarchy {
			shallow = &snap.Findings[i]
			break
		}
	}
	require.NotNil(t, shallow)
	assert.Equal(t, "billing.invoice.created", shallow.Suggested)
}

func TestExpandShallow(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"orders", "created"}, "orders.order.created"},
		{[]string{"order", "created"}, "order.order.created"},
		{[]string{"billing", "invoice"}, "billing.invoice.created"},
		{[]string{"payments", "transfer"}, "payments.transfer.created"},
		{[]string{"payments", "settled"}, "payments.payment.settled"},
		{[]string{"a", "b", "c"}, "a.b.c"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, expandShallow(tc.in), "in=%v", tc.in)
	}
}

func TestLooksLikeAction(t *testing.T) {
	t.Parallel()

	assert.True(t, looksLikeAction("created"))
	assert.True(t, looksLikeAction("paid"))
	assert.True(t, looksLikeAction("settled"))
	assert.True(t, looksLikeAction("shipped"))
	assert.False(t, looksLikeAction("invoice"))
	assert.False(t, looksLikeAction("transfer"))
	assert.False(t, looksLikeAction("order"))
}
