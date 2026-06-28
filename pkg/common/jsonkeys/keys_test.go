package jsonkeys

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnakeToCamel(t *testing.T) {
	tests := map[string]string{
		"name":                "name",
		"nats_url":            "natsUrl",
		"max_msgs":            "maxMsgs",
		"consumer_count":      "consumerCount",
		"retry_after_seconds": "retryAfterSeconds",
	}
	for in, want := range tests {
		assert.Equal(t, want, SnakeToCamel(in), "SnakeToCamel(%q)", in)
	}
}

func TestCamelToSnake(t *testing.T) {
	tests := map[string]string{
		"name":              "name",
		"natsUrl":           "nats_url",
		"maxMsgs":           "max_msgs",
		"consumerCount":     "consumer_count",
		"retryAfterSeconds": "retry_after_seconds",
	}
	for in, want := range tests {
		assert.Equal(t, want, CamelToSnake(in), "CamelToSnake(%q)", in)
	}
}

func TestRoundTrip(t *testing.T) {
	input := map[string]any{
		"durable_name":   "worker",
		"deliver_policy": "all",
		"nested": map[string]any{
			"first_seq": uint64(1),
			"items": []any{
				map[string]any{"filter_subject": "orders.>"},
			},
		},
	}
	raw, err := sonic.Marshal(input)
	require.NoError(t, err)
	camel, err := ToCamelCaseJSON(raw)
	require.NoError(t, err)
	snake, err := FromCamelCaseJSON(camel)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, sonic.Unmarshal(snake, &got))
	assert.Equal(t, "worker", got["durable_name"], "round trip durable_name")
	nested := got["nested"].(map[string]any)
	assert.Equal(t, float64(1), nested["first_seq"], "round trip first_seq")
}
