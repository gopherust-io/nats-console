package serializer_test

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSONUsesCamelCaseTags(t *testing.T) {
	cluster := domain.Cluster{
		ID:        "id-1",
		Name:      "default",
		NATSURL:   "nats://localhost:4222",
		IsDefault: true,
	}
	raw, err := sonic.Marshal(cluster)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, sonic.Unmarshal(raw, &decoded))
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

func TestStreamInfoDTOUsesCamelCase(t *testing.T) {
	info := domain.StreamInfo{
		Config: domain.StreamConfigDTO{
			Name:      "ORDERS",
			Retention: "limits",
			Storage:   "file",
		},
		State: domain.StreamStateDTO{
			FirstSeq:      1,
			LastSeq:       10,
			ConsumerCount: 2,
		},
	}
	raw, err := sonic.Marshal(info)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, sonic.Unmarshal(raw, &decoded))
	state := decoded["state"].(map[string]any)
	assert.Equal(t, float64(1), state["firstSeq"])
	assert.NotContains(t, state, "first_seq")
}
