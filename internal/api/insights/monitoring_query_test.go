package insights

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
)

func TestSanitizeMonitoringQueryDropsUnknownAndClampsLimit(t *testing.T) {
	t.Parallel()

	args := &fasthttp.Args{}
	args.Set("limit", "1000000")
	args.Set("auth", "1")
	args.Set("evil", "1")
	got := sanitizeMonitoringQuery("connz", args)
	assert.Contains(t, got, "limit=1024")
	assert.Contains(t, got, "auth=1")
	assert.NotContains(t, got, "evil")
}

func TestSanitizeMonitoringQueryEmptyForUnknownEndpointParams(t *testing.T) {
	t.Parallel()

	args := &fasthttp.Args{}
	args.Set("limit", "10")
	assert.Empty(t, sanitizeMonitoringQuery("varz", args))
}

func TestSanitizeMonitoringQueryJszAllowlist(t *testing.T) {
	t.Parallel()

	args := &fasthttp.Args{}
	args.Set("streams", "1")
	args.Set("limit", "99999")
	args.Set("evil", "1")
	got := sanitizeMonitoringQuery("jsz", args)
	assert.Contains(t, got, "streams=1")
	assert.Contains(t, got, "limit=1024")
	assert.NotContains(t, got, "evil")
}
