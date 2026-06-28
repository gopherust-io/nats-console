package natsclient

import (
	"testing"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/require"
)

func TestParseAccountJWT(t *testing.T) {
	t.Parallel()

	kp, err := nkeys.CreateAccount()
	require.NoError(t, err)
	pub, err := kp.PublicKey()
	require.NoError(t, err)

	claims := jwt.NewAccountClaims(pub)
	claims.Name = "TEST"
	claims.Subject = pub
	token, err := claims.Encode(kp)
	require.NoError(t, err)

	parsed, err := ParseAccountJWT(token)
	require.NoError(t, err)
	require.Equal(t, "TEST", parsed.Name)
}
