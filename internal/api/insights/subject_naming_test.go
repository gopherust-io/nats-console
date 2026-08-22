package insights

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/app/monitoring"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestSubjectNamingInputsFromJSZ(t *testing.T) {
	t.Parallel()

	raw := strings.StringToBytes(`{
		"account_details": [
			{
				"name": "DEFAULT",
				"stream_detail": [
					{
						"name": "ORDERS",
						"config": {"subjects": ["order.created", "Orders.Created", "orderCreated"]},
						"consumer_detail": [
							{
								"name": "worker",
								"config": {"filter_subject": "orders.created"}
							}
						]
					}
				]
			}
		]
	}`)

	inputs, err := monitoring.SubjectNamingInputsFromJSZ(raw)
	require.NoError(t, err)
	require.Len(t, inputs, 1)
	assert.Equal(t, "ORDERS", inputs[0].Name)
	assert.Equal(t, []string{"order.created", "Orders.Created", "orderCreated"}, inputs[0].Subjects)
	require.Len(t, inputs[0].Consumers, 1)
	assert.Equal(t, "worker", inputs[0].Consumers[0].Name)
	assert.Equal(t, "orders.created", inputs[0].Consumers[0].FilterSubject)

	snap := domain.AnalyzeSubjectNaming(inputs)
	assert.Greater(t, snap.Totals.Total, 0)
	assert.Greater(t, snap.Totals.InconsistentVariants, 0)

	var suggested string
	for _, f := range snap.Findings {
		if f.Kind == domain.SubjectNamingKindInconsistentVariant {
			suggested = f.Suggested
			break
		}
	}
	assert.Equal(t, "orders.order.created", suggested)
}
