package jetstream

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteAccountOverviewSSE(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	require.NoError(t, writeAccountOverviewSSE(w, []byte(`{"account":{"streams":1}}`)))
	require.Equal(t, "event: account-overview\ndata: {\"account\":{\"streams\":1}}\n\n", buf.String())
}
