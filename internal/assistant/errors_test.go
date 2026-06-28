package assistant

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrapErrorRateLimit(t *testing.T) {
	err := WrapError(errors.New(`Gemini rate limit exceeded for "gemini-2.5-flash": quota exceeded. Please retry in 47.108766854s.`))
	require.Equal(t, CodeRateLimit, err.Code, "expected rate_limit")
	assert.True(t, err.Retryable, "expected retryable")
	assert.Equal(t, 48, err.RetryAfter, "expected retry after 48s")
}

func TestMapGeminiHTTPErrorQuota(t *testing.T) {
	raw := []byte(`{"error":{"code":429,"message":"Quota exceeded for metric: generate_content_free_tier_requests, limit: 0, model: gemini-2.0-flash","status":"RESOURCE_EXHAUSTED"}}`)
	err := mapGeminiHTTPError("gemini-2.0-flash", 429, raw)
	require.Equal(t, CodeQuota, err.Code, "expected quota")
	assert.False(t, err.Retryable, "quota should not be retryable")
}

func TestWrapErrorBlocked(t *testing.T) {
	err := WrapError(errors.New("the assistant cannot access or reveal secrets, passwords, credentials, or internal database data"))
	require.Equal(t, CodeBlocked, err.Code, "expected blocked")
}
