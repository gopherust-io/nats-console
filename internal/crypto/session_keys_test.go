package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustSessionPEMs(t *testing.T) (privPEM, pubPEM string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}))
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	pubPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))
	return privPEM, pubPEM
}

func TestParseSessionRSAKeyPairRejectsEmpty(t *testing.T) {
	_, _, err := ParseSessionRSAKeyPair("", "")
	require.Error(t, err)
}

func TestParseSessionRSAKeyPairRoundTrip(t *testing.T) {
	privPEM, pubPEM := mustSessionPEMs(t)
	gotPriv, gotPub, err := ParseSessionRSAKeyPair(privPEM, pubPEM)
	require.NoError(t, err)
	require.NotNil(t, gotPriv)
	require.NotNil(t, gotPub)
	assert.GreaterOrEqual(t, gotPriv.N.BitLen(), minSessionRSABits)

	escapedPriv := strings.ReplaceAll(privPEM, "\n", `\n`)
	escapedPub := strings.ReplaceAll(pubPEM, "\n", `\n`)
	_, _, err = ParseSessionRSAKeyPair(escapedPriv, escapedPub)
	require.NoError(t, err, "literal \\n escapes should be accepted")
}

func TestParseSessionRSAKeyPairRejectsMismatch(t *testing.T) {
	privPEM, _ := mustSessionPEMs(t)
	_, otherPub := mustSessionPEMs(t)
	_, _, err := ParseSessionRSAKeyPair(privPEM, otherPub)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "matching pair")
}

func TestParseSessionRSAKeyPairRejectsSmallKey(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}))
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))

	_, _, err = ParseSessionRSAKeyPair(privPEM, pubPEM)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least")
}
