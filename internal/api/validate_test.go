package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateClusterName(t *testing.T) {
	require.NoError(t, validateClusterName("prod-cluster"), "valid name rejected")
	require.Error(t, validateClusterName("bad/name"), "slash in cluster name should fail")
	require.Error(t, validateClusterName(""), "empty name should fail")
}

func TestValidateNATSURL(t *testing.T) {
	for _, u := range []string{"nats://localhost:4222", "tls://nats.example.com", "ws://localhost:4222"} {
		require.NoError(t, validateNATSURL(u), "%q", u)
	}
	require.Error(t, validateNATSURL("http://bad"), "http scheme should fail")
}

func TestValidateRoles(t *testing.T) {
	require.NoError(t, validateRoles([]string{"admin"}))
	require.Error(t, validateRoles([]string{"superuser"}), "unknown role should fail")
}
