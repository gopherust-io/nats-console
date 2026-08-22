package insights

import (
	"encoding/json"
	"testing"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestChaosStoryDemo(t *testing.T) {
	h := &Handler{Core: &apikit.Core{}}
	ctx := &fasthttp.RequestCtx{}
	h.ChaosStoryDemo(ctx)
	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	var body map[string]any
	require.NoError(t, json.Unmarshal(ctx.Response.Body(), &body))
	data := body["data"].(map[string]any)
	story := data["story"].(map[string]any)
	assert.NotEmpty(t, story["title"])
	acts := story["acts"].([]any)
	assert.GreaterOrEqual(t, len(acts), 3)
}

func TestChaosStoryGenerateNotEnabled(t *testing.T) {
	h := &Handler{Core: &apikit.Core{}}
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetBody(commonstrings.StringToBytes(`{"hint":"Black Friday"}`))
	h.ChaosStoryGenerate(ctx)
	assert.Equal(t, fasthttp.StatusNotFound, ctx.Response.StatusCode())
	var body map[string]any
	require.NoError(t, json.Unmarshal(ctx.Response.Body(), &body))
	errObj := body["error"].(map[string]any)
	assert.Equal(t, "not_enabled", errObj["code"])
}
