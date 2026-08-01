//go:build integration

package integration_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreRequestReplyProbeCRUD(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()
	clusterID := stack.DefaultClusterID(t)
	st := stack.Store

	payloadB64 := base64.StdEncoding.EncodeToString(commonstrings.StringToBytes(`{"ping":true}`))
	created, err := st.CreateRequestReplyProbe(ctx, clusterID, domain.RequestReplyProbeCreate{
		Subject:       "orders.status",
		PayloadB64:    payloadB64,
		PayloadFormat: domain.RequestReplyFormatJSON,
		TimeoutMs:     1500,
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	assert.Equal(t, "orders.status", created.Subject)
	assert.Equal(t, payloadB64, created.PayloadB64)
	assert.True(t, created.Enabled)
	assert.Equal(t, 1500, created.TimeoutMs)

	listed, err := st.ListRequestReplyProbes(ctx, clusterID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)
	assert.Equal(t, payloadB64, listed[0].PayloadB64)

	public, err := st.ListRequestReplyProbesPublic(ctx, clusterID)
	require.NoError(t, err)
	require.Len(t, public, 1)
	assert.Equal(t, created.ID, public[0].ID)
	assert.Empty(t, public[0].PayloadB64)
	assert.Equal(t, "orders.status", public[0].Subject)

	enabled := false
	newSubject := "orders.ping"
	updated, err := st.UpdateRequestReplyProbe(ctx, clusterID, created.ID, domain.RequestReplyProbeUpdate{
		Subject: &newSubject,
		Enabled: &enabled,
	})
	require.NoError(t, err)
	assert.Equal(t, "orders.ping", updated.Subject)
	assert.False(t, updated.Enabled)
	assert.Equal(t, payloadB64, updated.PayloadB64)

	enabledOnly, err := st.ListEnabledRequestReplyProbes(ctx, clusterID)
	require.NoError(t, err)
	assert.Empty(t, enabledOnly)

	require.NoError(t, st.DeleteRequestReplyProbe(ctx, clusterID, created.ID))
	_, err = st.GetRequestReplyProbe(ctx, clusterID, created.ID)
	require.ErrorIs(t, err, store.ErrRequestReplyProbeNotFound)
}
