//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/tests/testutil"
)

func TestStoreEventCatalogCRUD(t *testing.T) {
	stack := testutil.SetupStack(t)
	ctx := context.Background()
	clusterID := stack.DefaultClusterID(t)
	st := stack.Store

	schema := strings.StringToBytes(`{"type":"object","properties":{"id":{"type":"string"}}}`)
	row, err := st.UpsertEventCatalogEntry(ctx, clusterID, "orders.created", "", domain.EventCatalogUpsert{
		Owner:            "Growth Team",
		Description:      "Order successfully created",
		Schema:           schema,
		Example:          strings.StringToBytes(`{"id":"ord_1"}`),
		Deprecated:       true,
		SuccessorSubject: "orders.v2",
		DeprecationNote:  "Prefer v2",
	})
	require.NoError(t, err)
	assert.Equal(t, "orders.created", row.Subject)
	assert.Equal(t, "Growth Team", row.Owner)
	assert.Equal(t, "Order successfully created", row.Description)
	assert.JSONEq(t, strings.BytesToString(schema), strings.BytesToString(row.Schema))
	assert.JSONEq(t, `{"id":"ord_1"}`, strings.BytesToString(row.Example))
	assert.True(t, row.Deprecated)
	assert.Equal(t, "orders.v2", row.SuccessorSubject)
	assert.Equal(t, "Prefer v2", row.DeprecationNote)

	listed, err := st.ListEventCatalogEntries(ctx, clusterID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, "Growth Team", listed[0].Owner)
	assert.True(t, listed[0].Deprecated)

	updated, err := st.UpsertEventCatalogEntry(ctx, clusterID, "orders.created", "", domain.EventCatalogUpsert{
		Owner:       "Platform",
		Description: "Updated",
		Schema:      nil,
		Example:     nil,
		Deprecated:  false,
	})
	require.NoError(t, err)
	assert.Equal(t, "Platform", updated.Owner)
	assert.Equal(t, "Updated", updated.Description)
	assert.Empty(t, updated.Schema)
	assert.Empty(t, updated.Example)
	assert.False(t, updated.Deprecated)

	require.NoError(t, st.DeleteEventCatalogEntry(ctx, clusterID, "orders.created"))
	_, err = st.GetEventCatalogEntry(ctx, clusterID, "orders.created")
	require.ErrorIs(t, err, store.ErrEventCatalogEntryNotFound)

	_, err = st.UpsertEventCatalogEntry(ctx, clusterID, "orders.>", "", domain.EventCatalogUpsert{Owner: "x"})
	require.Error(t, err)
}
