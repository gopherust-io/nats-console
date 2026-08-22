package insights

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/app/monitoring"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventCatalogLiveFromJSZ(t *testing.T) {
	raw := strings.StringToBytes(`{
		"account_details": [{
			"name": "DEFAULT",
			"stream_detail": [{
				"name": "ORDERS",
				"config": {"subjects": ["orders.created", "orders.>"]},
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
	live, err := monitoring.EventCatalogLiveFromJSZ(raw)
	require.NoError(t, err)
	require.Len(t, live, 1)
	assert.Equal(t, "ORDERS", live[0].Name)
	assert.Equal(t, []string{"orders.created", "orders.>"}, live[0].Subjects)
	require.Len(t, live[0].Consumers, 1)
	assert.Equal(t, "billing", live[0].Consumers[0].Name)
	assert.Equal(t, "billing", live[0].Consumers[0].DurableName)
	assert.Equal(t, "orders.>", live[0].Consumers[0].FilterSubject)
	assert.Equal(t, "billing-svc", live[0].Consumers[0].Metadata["nats-consol/service"])
}
