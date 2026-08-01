package api

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventWikipediaAssemblesFromJSZHelpers(t *testing.T) {
	raw := strings.StringToBytes(`{
		"account_details": [{
			"name": "DEFAULT",
			"stream_detail": [{
				"name": "ORDERS",
				"config": {"subjects": ["orders.created", "orders.new"]},
				"consumer_detail": [{
					"name": "billing",
					"config": {
						"filter_subject": "orders.>",
						"durable_name": "billing",
						"metadata": {"nats-consol/service": "billing-svc"}
					}
				}]
			}]
		}]
	}`)
	live, err := eventCatalogLiveFromJSZ(raw)
	require.NoError(t, err)
	genomeInputs, err := eventGenomeInputsFromJSZ(raw)
	require.NoError(t, err)

	docs := []domain.EventCatalogDoc{{
		Subject:     "orders.new",
		Owner:       "Growth",
		Description: "Legacy",
		Deprecated:  true,
	}}
	catalog := domain.BuildEventCatalog(live, docs)
	genome := domain.AnalyzeEventGenome(genomeInputs)
	snap := domain.BuildEventWikipedia(catalog, genome)
	require.GreaterOrEqual(t, len(snap.Pages), 2)

	bySubj := map[string]domain.EventWikipediaPage{}
	for _, p := range snap.Pages {
		bySubj[p.Subject] = p
	}
	legacy := bySubj["orders.new"]
	assert.Equal(t, "Legacy", legacy.Purpose)
	assert.True(t, legacy.Deprecation.Deprecated)
	assert.Contains(t, legacy.RelatedEvents, "orders.created")
	assert.NotEmpty(t, legacy.KnownIncidents)

	filtered := domain.FilterEventWikipediaPages(snap, "orders.created")
	require.Len(t, filtered.Pages, 1)
	assert.Equal(t, "orders.created", filtered.Pages[0].Subject)
}
