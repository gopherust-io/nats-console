package crypto_test

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := crypto.New("test-encryption-key-32chars!")
	require.NoError(t, err)

	plain := "super-secret-nats-token"
	cipher, err := enc.Encrypt(plain)
	require.NoError(t, err)
	assert.True(t, crypto.IsEncrypted(cipher), "expected encrypted prefix")

	out, err := enc.Decrypt(cipher)
	require.NoError(t, err)
	assert.Equal(t, plain, out)
}

func TestPlaintextPassthrough(t *testing.T) {
	enc, err := crypto.New("another-valid-key-here!!")
	require.NoError(t, err)
	out, err := enc.Decrypt("plain-token")
	require.NoError(t, err)
	assert.Equal(t, "plain-token", out)
}
