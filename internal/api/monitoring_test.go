package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMonitoringEndpoint(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateMonitoringEndpoint("gatewayz"))
	require.NoError(t, validateMonitoringEndpoint("JSZ"))
	assert.Error(t, validateMonitoringEndpoint("../../../etc/passwd"))
	assert.Error(t, validateMonitoringEndpoint("unknown"))
}
