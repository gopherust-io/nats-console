package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestBuildEventWikipedia_UndocumentedLive(t *testing.T) {
	catalog := EventCatalogSnapshot{
		Entries: []EventCatalogEntry{{
			Subject:   "orders.created",
			Streams:   []string{"ORDERS"},
			Consumers: []EventCatalogConsumer{{Name: "billing", Stream: "ORDERS", Service: "billing-svc"}},
		}},
		Totals: EventCatalogTotals{Total: 1, Undocumented: 1},
	}
	snap := BuildEventWikipedia(catalog, EventGenomeSnapshot{})
	require.Len(t, snap.Pages, 1)
	p := snap.Pages[0]
	assert.Equal(t, "orders.created", p.Subject)
	assert.Empty(t, p.Purpose)
	assert.Empty(t, p.Owner)
	assert.False(t, p.Documented)
	assert.Equal(t, []string{"ORDERS"}, p.History.Streams)
	require.Len(t, p.Consumers, 1)
	require.Len(t, p.KnownIncidents, 1)
	assert.Equal(t, "ORDERS", p.KnownIncidents[0].Stream)
	assert.Equal(t, "billing", p.KnownIncidents[0].Consumer)
	assert.Empty(t, p.RelatedEvents)
	assert.False(t, p.Deprecation.Deprecated)
	assert.Equal(t, 1, snap.Totals.Total)
	assert.Equal(t, 0, snap.Totals.Documented)
}

func TestBuildEventWikipedia_OrphanDeprecatedAndRelated(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := created.Add(time.Hour)
	schema := strings.StringToBytes(`{"type":"object"}`)
	example := strings.StringToBytes(`{"id":"ord_1"}`)
	catalog := EventCatalogSnapshot{
		Entries: []EventCatalogEntry{
			{
				Subject:          "orders.new",
				Owner:            "Growth",
				Description:      "Legacy create event",
				Schema:           schema,
				Example:          example,
				Deprecated:       true,
				SuccessorSubject: "orders.created",
				DeprecationNote:  "Use orders.created",
				UpdatedBy:        "user-1",
				CreatedAt:        created,
				UpdatedAt:        updated,
				Orphan:           true,
				Documented:       true,
			},
			{
				Subject:    "orders.created",
				Owner:      "Growth",
				Streams:    []string{"ORDERS"},
				Documented: true,
			},
		},
	}
	genome := EventGenomeSnapshot{
		Findings: []EventGenomeFinding{{
			Subject: "orders.new",
			Cluster: []string{"orders.created", "orders.new", "order.created"},
		}},
	}
	snap := BuildEventWikipedia(catalog, genome)
	require.Len(t, snap.Pages, 2)

	bySubj := map[string]EventWikipediaPage{}
	for _, p := range snap.Pages {
		bySubj[p.Subject] = p
	}

	legacy := bySubj["orders.new"]
	assert.True(t, legacy.Orphan)
	assert.True(t, legacy.Documented)
	assert.Equal(t, "Legacy create event", legacy.Purpose)
	assert.Equal(t, "Growth", legacy.Owner)
	assert.JSONEq(t, strings.BytesToString(schema), strings.BytesToString(legacy.Schema))
	assert.JSONEq(t, strings.BytesToString(example), strings.BytesToString(legacy.Example))
	assert.True(t, legacy.Deprecation.Deprecated)
	assert.Equal(t, "orders.created", legacy.Deprecation.SuccessorSubject)
	assert.Equal(t, "Use orders.created", legacy.Deprecation.Note)
	assert.Equal(t, created, legacy.History.CreatedAt)
	assert.Equal(t, updated, legacy.History.UpdatedAt)
	assert.Equal(t, "user-1", legacy.History.UpdatedBy)
	assert.Equal(t, []string{"order.created", "orders.created"}, legacy.RelatedEvents)
	assert.Empty(t, legacy.KnownIncidents)

	createdPage := bySubj["orders.created"]
	assert.Equal(t, []string{"order.created", "orders.new"}, createdPage.RelatedEvents)

	assert.Equal(t, 2, snap.Totals.Documented)
	assert.Equal(t, 1, snap.Totals.Deprecated)
	assert.Equal(t, 1, snap.Totals.Orphan)
}

func TestFilterEventWikipediaPages(t *testing.T) {
	snap := EventWikipediaSnapshot{
		Pages: []EventWikipediaPage{
			{Subject: "a.one", Documented: true},
			{Subject: "b.two", Orphan: true},
		},
		Totals: EventWikipediaTotals{Total: 2, Documented: 1, Orphan: 1},
	}
	filtered := FilterEventWikipediaPages(snap, "b.two")
	require.Len(t, filtered.Pages, 1)
	assert.Equal(t, "b.two", filtered.Pages[0].Subject)
	assert.Equal(t, 1, filtered.Totals.Total)
	assert.Equal(t, 1, filtered.Totals.Orphan)
	assert.Equal(t, 0, filtered.Totals.Documented)

	empty := FilterEventWikipediaPages(snap, "missing")
	assert.Empty(t, empty.Pages)
	assert.Equal(t, 0, empty.Totals.Total)

	all := FilterEventWikipediaPages(snap, "  ")
	require.Len(t, all.Pages, 2)
}

func TestValidateEventCatalogExample(t *testing.T) {
	assert.NoError(t, ValidateEventCatalogExample(nil))
	assert.NoError(t, ValidateEventCatalogExample(strings.StringToBytes(`{"id":"1"}`)))
	assert.Error(t, ValidateEventCatalogExample(strings.StringToBytes(`[]`)))
}
