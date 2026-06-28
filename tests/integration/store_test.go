//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/gopherust-io/nats-consol/internal/crypto"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreClusterCRUD(t *testing.T) {
	testutil.RequireDocker(t)
	ctx := context.Background()
	pgURL := testutil.StartPostgres(t, ctx)
	st := testutil.OpenStore(t, ctx, pgURL)

	created, err := st.CreateCluster(ctx, store.ClusterCreate{
		Name:          "prod",
		NATSURL:       "nats://nats.example:4222",
		MonitoringURL: "http://nats.example:8222",
		IsDefault:     true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	assert.Equal(t, "prod", created.Name)

	got, err := st.GetCluster(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.NATSURL, got.NATSURL)

	clusters, err := st.ListClusters(ctx)
	require.NoError(t, err)
	require.Len(t, clusters, 1)

	require.NoError(t, st.DeleteCluster(ctx, created.ID))
	count, err := st.CountClusters(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestStoreUserRoles(t *testing.T) {
	testutil.RequireDocker(t)
	ctx := context.Background()
	pgURL := testutil.StartPostgres(t, ctx)
	st := testutil.OpenStore(t, ctx, pgURL)

	user, err := st.CreateUser(ctx, store.UserCreate{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "secret-password",
		Roles:    []string{store.RoleViewer},
	})
	require.NoError(t, err)

	require.NoError(t, st.SetUserRoles(ctx, user.ID, []string{store.RoleOperator}))

	got, err := st.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, got.Roles, 1)
	assert.Equal(t, store.RoleOperator, got.Roles[0])
}

func TestStoreEncryptsClusterToken(t *testing.T) {
	testutil.RequireDocker(t)
	ctx := context.Background()
	pgURL := testutil.StartPostgres(t, ctx)

	enc, err := crypto.New("test-encryption-key-32chars!")
	require.NoError(t, err)

	st, err := store.Open(ctx, pgURL, testutil.MigrationsDir(), enc, store.DefaultPoolConfig())
	require.NoError(t, err)
	t.Cleanup(st.Close)

	created, err := st.CreateCluster(ctx, store.ClusterCreate{
		Name:    "secure",
		NATSURL: "nats://localhost:4222",
		Token:   "super-secret-token",
	})
	require.NoError(t, err)

	creds, err := st.GetClusterCredentials(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "super-secret-token", creds.Token)

	// Raw DB value must not contain plaintext token.
	var rawToken string
	err = st.Pool().QueryRow(ctx, `SELECT token FROM clusters WHERE id = $1`, created.ID).Scan(&rawToken)
	require.NoError(t, err)
	assert.NotEqual(t, "super-secret-token", rawToken, "token stored in plaintext")
	assert.True(t, crypto.IsEncrypted(rawToken), "expected encrypted token, got %q", rawToken)
}
