package serializer_test

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/jsonkeys"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSONUsesCamelCase(t *testing.T) {
	cluster := domain.Cluster{
		ID:        "id-1",
		Name:      "default",
		NATSURL:   "nats://localhost:4222",
		IsDefault: true,
	}
	raw, err := sonic.Marshal(cluster)
	require.NoError(t, err)
	out, err := jsonkeys.ToCamelCaseJSON(raw)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, sonic.Unmarshal(out, &decoded))
	assert.Equal(t, "nats://localhost:4222", decoded["natsUrl"], "expected natsUrl")
	assert.NotContains(t, decoded, "nats_url", "unexpected snake_case key in response")
}

func TestUnmarshalRequestAcceptsCamelCase(t *testing.T) {
	body := []byte(`{"natsUrl":"nats://localhost:4222","isDefault":true}`)
	var req struct {
		NATSURL   string `json:"natsUrl"`
		IsDefault bool   `json:"isDefault"`
	}
	require.NoError(t, serializer.UnmarshalRequest(body, &req))
	assert.Equal(t, "nats://localhost:4222", req.NATSURL)
	assert.True(t, req.IsDefault)
}

func TestUnmarshalNATSRequestAcceptsCamelCase(t *testing.T) {
	body := []byte(`{"durableName":"worker","deliverPolicy":"all","ackPolicy":"explicit"}`)
	var cfg struct {
		DurableName   string `json:"durable_name"`
		DeliverPolicy string `json:"deliver_policy"`
		AckPolicy     string `json:"ack_policy"`
	}
	require.NoError(t, serializer.UnmarshalNATSRequest(body, &cfg))
	assert.Equal(t, "worker", cfg.DurableName)
	assert.Equal(t, "all", cfg.DeliverPolicy)
}
