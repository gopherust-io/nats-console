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

// H6: reject NATS/monitoring URLs pointing at cloud metadata / link-local hosts.
func TestValidateNATSURLRejectsSSRFHosts(t *testing.T) {
	for _, u := range []string{
		"nats://169.254.169.254:4222",
		"tls://[fe80::1]:4222",
		"nats://0.0.0.0:4222",
	} {
		require.Error(t, validateNATSURL(u), "%q should be rejected", u)
	}
}

func TestValidateMonitoringURL(t *testing.T) {
	for _, u := range []string{"http://localhost:8222", "https://nats.example.com:8222"} {
		require.NoError(t, validateMonitoringURL(u), "%q", u)
	}
	for _, u := range []string{
		"",
		"ftp://nats.example.com",
		"file:///etc/passwd",
		"gopher://169.254.169.254/",
		"http://169.254.169.254/latest/meta-data/",
		"http://metadata.google.internal/computeMetadata/v1/",
		"http://[fe80::1]/",
	} {
		require.Error(t, validateMonitoringURL(u), "%q should be rejected", u)
	}
}

func TestIsBlockedSSRFHost(t *testing.T) {
	require.True(t, isBlockedSSRFHost("169.254.169.254"))
	require.True(t, isBlockedSSRFHost("fe80::1"))
	require.True(t, isBlockedSSRFHost("metadata.google.internal"))
	require.True(t, isBlockedSSRFHost(""))
	require.False(t, isBlockedSSRFHost("localhost"))
	require.False(t, isBlockedSSRFHost("127.0.0.1"))
	require.False(t, isBlockedSSRFHost("10.0.0.5"))
	require.False(t, isBlockedSSRFHost("nats.example.com"))
}

func TestValidateRoles(t *testing.T) {
	require.NoError(t, validateRoles([]string{"admin"}))
	require.Error(t, validateRoles([]string{"superuser"}), "unknown role should fail")
}
