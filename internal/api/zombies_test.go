package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestZombieStreamsFromJSZ(t *testing.T) {
	t.Parallel()

	raw := strings.StringToBytes(`{
		"account_details": [
			{
				"name": "DEFAULT",
				"stream_detail": [
					{
						"name": "ORDERS",
						"config": {"subjects": ["orders.>", "events.>"]},
						"state": {"messages": 0, "last_seq": 0, "consumer_count": 1},
						"consumer_detail": [
							{
								"name": "worker",
								"config": {"filter_subject": "orders.>"},
								"delivered": {"consumer_seq": 0, "stream_seq": 0}
							}
						]
					}
				]
			}
		]
	}`)

	streams, err := zombieStreamsFromJSZ(raw)
	require.NoError(t, err)
	require.Len(t, streams, 1)
	assert.Equal(t, "ORDERS", streams[0].Name)
	assert.Equal(t, []string{"orders.>", "events.>"}, streams[0].Subjects)
	require.Len(t, streams[0].Consumers, 1)
	assert.Equal(t, "worker", streams[0].Consumers[0].Name)
	assert.Equal(t, "orders.>", streams[0].Consumers[0].FilterSubject)

	snap := domain.AnalyzeZombies(streams)
	assert.GreaterOrEqual(t, snap.Totals.EmptyStreams, 1)
	assert.GreaterOrEqual(t, snap.Totals.UnpublishedSubjects, 1)
}
