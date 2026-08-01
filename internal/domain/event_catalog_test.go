package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestIsConcreteCatalogSubject(t *testing.T) {
	assert.True(t, IsConcreteCatalogSubject("orders.created"))
	assert.False(t, IsConcreteCatalogSubject("orders.>"))
	assert.False(t, IsConcreteCatalogSubject("orders.*"))
	assert.False(t, IsConcreteCatalogSubject(""))
	assert.False(t, IsConcreteCatalogSubject("  "))
}

func TestValidateEventCatalogSchema(t *testing.T) {
	assert.NoError(t, ValidateEventCatalogSchema(nil))
	assert.NoError(t, ValidateEventCatalogSchema(strings.StringToBytes(("null"))))
	assert.NoError(t, ValidateEventCatalogSchema(strings.StringToBytes(`{"type":"object"}`)))
	assert.Error(t, ValidateEventCatalogSchema(strings.StringToBytes(`[]`)))
	assert.Error(t, ValidateEventCatalogSchema(strings.StringToBytes(`"x"`)))
	assert.Error(t, ValidateEventCatalogSchema(strings.StringToBytes(`{`)))
}

func TestBuildEventCatalog_DiscoverConcreteAndConsumers(t *testing.T) {
	live := []EventCatalogLiveStream{
		{
			Name:     "ORDERS",
			Subjects: []string{"orders.created", "orders.>"},
			Consumers: []EventCatalogLiveConsumer{
				{
					Name:          "billing",
					DurableName:   "billing",
					FilterSubject: "orders.>",
					Metadata:      map[string]string{MetadataServiceKey: "billing-svc"},
				},
				{
					Name:          "analytics",
					FilterSubject: "orders.created",
				},
			},
		},
	}

	snap := BuildEventCatalog(live, nil)
	require.Len(t, snap.Entries, 1)
	e := snap.Entries[0]
	assert.Equal(t, "orders.created", e.Subject)
	assert.False(t, e.Orphan)
	assert.False(t, e.Documented)
	assert.Equal(t, []string{"ORDERS"}, e.Streams)
	require.Len(t, e.Consumers, 2)
	assert.Equal(t, "analytics", e.Consumers[0].Name)
	assert.Equal(t, "billing", e.Consumers[1].Name)
	assert.Equal(t, "billing-svc", e.Consumers[1].Service)
	assert.Equal(t, 1, snap.Totals.Total)
	assert.Equal(t, 1, snap.Totals.Undocumented)
	assert.Equal(t, 0, snap.Totals.Orphan)
}

func TestBuildEventCatalog_DiscoverFromConsumerFilter(t *testing.T) {
	live := []EventCatalogLiveStream{
		{
			Name:     "ORDERS",
			Subjects: []string{"orders.>"},
			Consumers: []EventCatalogLiveConsumer{
				{Name: "ship", FilterSubject: "orders.shipped"},
			},
		},
	}
	snap := BuildEventCatalog(live, nil)
	require.Len(t, snap.Entries, 1)
	assert.Equal(t, "orders.shipped", snap.Entries[0].Subject)
	assert.Equal(t, []string{"ORDERS"}, snap.Entries[0].Streams)
	require.Len(t, snap.Entries[0].Consumers, 1)
	assert.Equal(t, "ship", snap.Entries[0].Consumers[0].Name)
}

func TestBuildEventCatalog_OrphanDocAndEnrichment(t *testing.T) {
	schema := strings.StringToBytes(`{"type":"object","properties":{"id":{"type":"string"}}}`)
	docs := []EventCatalogDoc{
		{
			Subject:     "legacy.event",
			Owner:       "Growth Team",
			Description: "Order successfully created",
			Schema:      schema,
		},
		{
			Subject: "orders.created",
			Owner:   "Platform",
		},
	}
	live := []EventCatalogLiveStream{
		{
			Name:     "ORDERS",
			Subjects: []string{"orders.created"},
		},
	}

	snap := BuildEventCatalog(live, docs)
	require.Len(t, snap.Entries, 2)

	bySubj := map[string]EventCatalogEntry{}
	for _, e := range snap.Entries {
		bySubj[e.Subject] = e
	}

	liveEntry := bySubj["orders.created"]
	assert.False(t, liveEntry.Orphan)
	assert.True(t, liveEntry.Documented)
	assert.Equal(t, "Platform", liveEntry.Owner)
	assert.Equal(t, []string{"ORDERS"}, liveEntry.Streams)

	orphan := bySubj["legacy.event"]
	assert.True(t, orphan.Orphan)
	assert.True(t, orphan.Documented)
	assert.Equal(t, "Growth Team", orphan.Owner)
	assert.Equal(t, "Order successfully created", orphan.Description)
	assert.JSONEq(t, strings.BytesToString(schema), strings.BytesToString(orphan.Schema))
	assert.Empty(t, orphan.Streams)
	assert.Empty(t, orphan.Consumers)

	assert.Equal(t, 2, snap.Totals.Documented)
	assert.Equal(t, 1, snap.Totals.Orphan)
}

func TestBuildEventCatalog_DocCoveredByWildcardNotOrphan(t *testing.T) {
	live := []EventCatalogLiveStream{
		{
			Name:     "ORDERS",
			Subjects: []string{"orders.>"},
			Consumers: []EventCatalogLiveConsumer{
				{Name: "all", FilterSubject: "orders.>"},
			},
		},
	}
	docs := []EventCatalogDoc{{Subject: "orders.created", Owner: "Growth Team", Description: "created"}}
	snap := BuildEventCatalog(live, docs)
	require.Len(t, snap.Entries, 1)
	e := snap.Entries[0]
	assert.Equal(t, "orders.created", e.Subject)
	assert.False(t, e.Orphan)
	assert.True(t, e.Documented)
	assert.Equal(t, []string{"ORDERS"}, e.Streams)
	require.Len(t, e.Consumers, 1)
}

func TestCanonicalEventCatalogSubject(t *testing.T) {
	got, err := CanonicalEventCatalogSubject("  orders.created  ")
	require.NoError(t, err)
	assert.Equal(t, "orders.created", got)
	_, err = CanonicalEventCatalogSubject("orders.>")
	assert.Error(t, err)
}
