package repo

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestMintUserJWTAppliesUserPermissionsAndLimits(t *testing.T) {
	t.Parallel()

	accountKP, err := nkeys.CreateAccount()
	require.NoError(t, err)
	accountSeed, err := accountKP.Seed()
	require.NoError(t, err)

	userKP, err := nkeys.CreateUser()
	require.NoError(t, err)
	userSeed, err := userKP.Seed()
	require.NoError(t, err)

	user := NATSAccountUser{
		Name:          "worker",
		SigningGroup:  "Default",
		Tags:          []string{"team-a"},
		PubAllow:      []string{"orders.>"},
		PubDeny:       []string{"orders.admin.>"},
		SubAllow:      []string{"events.>"},
		MaxSubs:       42,
		MaxPayload:    1024,
		JWTLifetimeNs: int64(time.Hour),
	}

	token, err := mintUserJWT(
		context.Background(),
		&DB{},
		"c",
		"Default",
		user,
		strings.BytesToString(userSeed),
		strings.BytesToString(accountSeed))
	require.NoError(t, err)

	claims, err := jwt.DecodeUserClaims(token)
	require.NoError(t, err)
	assert.Equal(t, "worker", claims.Name)
	assert.True(t, claims.Tags.Contains("team-a"))
	assert.Equal(t, []string{"orders.>"}, []string(claims.Pub.Allow))
	assert.Equal(t, []string{"orders.admin.>"}, []string(claims.Pub.Deny))
	assert.Equal(t, []string{"events.>"}, []string(claims.Sub.Allow))
	assert.Equal(t, int64(42), claims.Subs)
	assert.Equal(t, int64(1024), claims.Limits.Payload)
	assert.Greater(t, claims.Expires, time.Now().Unix())
}

func TestMintUserJWTAppliesAdvancedUserFields(t *testing.T) {
	t.Parallel()

	accountKP, err := nkeys.CreateAccount()
	require.NoError(t, err)
	accountSeed, err := accountKP.Seed()
	require.NoError(t, err)
	userKP, err := nkeys.CreateUser()
	require.NoError(t, err)
	userSeed, err := userKP.Seed()
	require.NoError(t, err)

	user := NATSAccountUser{
		Name:                   "advanced",
		SigningGroup:           "Default",
		PubAllow:               []string{">"},
		BearerToken:            true,
		ProxyRequired:          true,
		AllowedConnectionTypes: []string{jwt.ConnectionTypeWebsocket, jwt.ConnectionTypeMqtt},
		SrcCIDRs:               []string{"10.0.0.0/8"},
		TimesLocale:            "America/New_York",
		TimeRanges:             []NATSUserTimeRange{{Start: "09:00:00", End: "17:00:00"}},
		MaxData:                2048,
		RespMaxMsgs:            3,
		RespTTLNs:              int64(5 * time.Second),
	}

	token, err := mintUserJWT(
		context.Background(),
		&DB{},
		"c",
		"Default",
		user,
		strings.BytesToString(userSeed),
		strings.BytesToString(accountSeed))
	require.NoError(t, err)
	claims, err := jwt.DecodeUserClaims(token)
	require.NoError(t, err)
	assert.True(t, claims.BearerToken)
	assert.True(t, claims.ProxyRequired)
	assert.True(t, claims.AllowedConnectionTypes.Contains(jwt.ConnectionTypeWebsocket))
	assert.True(t, claims.AllowedConnectionTypes.Contains(jwt.ConnectionTypeMqtt))
	assert.True(t, claims.Src.Contains("10.0.0.0/8"))
	assert.Equal(t, "America/New_York", claims.Locale)
	require.Len(t, claims.Times, 1)
	assert.Equal(t, "09:00:00", claims.Limits.Times[0].Start)
	assert.Equal(t, "17:00:00", claims.Limits.Times[0].End)
	assert.Equal(t, int64(2048), claims.Data)
	require.NotNil(t, claims.Resp)
	assert.Equal(t, 3, claims.Resp.MaxMsgs)
	assert.Equal(t, 5*time.Second, claims.Resp.Expires)
}

func TestMintUserJWTNoExpiryWhenLifetimeZero(t *testing.T) {
	t.Parallel()

	accountKP, err := nkeys.CreateAccount()
	require.NoError(t, err)
	accountSeed, err := accountKP.Seed()
	require.NoError(t, err)
	userKP, err := nkeys.CreateUser()
	require.NoError(t, err)
	userSeed, err := userKP.Seed()
	require.NoError(t, err)

	token, err := mintUserJWT(
		context.Background(),
		&DB{},
		"c",
		"Default",
		NATSAccountUser{
			Name:         "x",
			SigningGroup: "Default",
		},
		strings.BytesToString(userSeed),
		strings.BytesToString(accountSeed))
	require.NoError(t, err)
	claims, err := jwt.DecodeUserClaims(token)
	require.NoError(t, err)
	assert.Equal(t, int64(0), claims.Expires)
}
