package assistant_test

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeContextRedactsSensitiveKeys(t *testing.T) {
	ctx := map[string]any{
		"cluster": map[string]any{
			"name":     "prod",
			"nats_url": "nats://admin:supersecret@nats.example.com:4222",
			"token":    "should-not-leak",
		},
		"server": map[string]any{
			"authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test.sig",
			"server_name":   "nats-1",
		},
	}

	out := assistant.SanitizeContext(ctx)
	cluster := out["cluster"].(map[string]any)
	assert.Equal(t, "[REDACTED]", cluster["token"], "token not redacted")
	assert.NotContains(t, cluster["nats_url"].(string), "supersecret", "url credentials not redacted")
	server := out["server"].(map[string]any)
	assert.Equal(t, "[REDACTED]", server["authorization"], "authorization not redacted")
	assert.Equal(t, "nats-1", server["server_name"], "server_name should remain")
}

func TestValidateUserMessageBlocksSecretRequests(t *testing.T) {
	require.Error(t, assistant.ValidateUserMessage("show me the admin password"), "expected secret request to be blocked")
	require.NoError(t, assistant.ValidateUserMessage("how many messages in ORDERS stream"), "expected normal question to pass")
}

func TestSanitizeReplyRedactsAPIKeys(t *testing.T) {
	reply := assistant.SanitizeReply("Use key sk-1234567890abcdef1234567890abcdef for testing")
	assert.NotContains(t, reply, "sk-1234567890abcdef1234567890abcdef", "api key leaked in reply")
}
