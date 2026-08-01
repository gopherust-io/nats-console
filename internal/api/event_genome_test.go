package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestEventGenomeInputsFromJSZ(t *testing.T) {
	t.Parallel()

	raw := strings.StringToBytes(`{
		"account_details": [
			{
				"name": "DEFAULT",
				"stream_detail": [
					{
						"name": "ORDERS",
						"config": {"subjects": ["orders.created", "orders.new", "order.created"]},
						"consumer_detail": [
							{
								"name": "worker",
								"config": {"filter_subject": "orders.added"}
							}
						]
					}
				]
			}
		]
	}`)

	inputs, err := eventGenomeInputsFromJSZ(raw)
	require.NoError(t, err)
	require.Len(t, inputs, 1)
	assert.Equal(t, "ORDERS", inputs[0].Name)
	assert.Equal(t, []string{"orders.created", "orders.new", "order.created"}, inputs[0].Subjects)
	require.Len(t, inputs[0].Consumers, 1)
	assert.Equal(t, "worker", inputs[0].Consumers[0].Name)
	assert.Equal(t, "orders.added", inputs[0].Consumers[0].FilterSubject)

	snap := domain.AnalyzeEventGenome(inputs)
	assert.Equal(t, 1, snap.Totals.Clusters)
	assert.Greater(t, snap.Totals.Total, 0)

	for _, f := range snap.Findings {
		assert.Equal(t, "order.created", f.Genome)
		assert.Contains(t, f.Cluster, "orders.new")
	}
}
