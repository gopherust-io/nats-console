package apikit_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
)

func TestWriteSSEEventMultilineJSON(t *testing.T) {
	t.Parallel()

	pretty := []byte("{\n  \"num_connections\": 1,\n  \"connections\": []\n}\n")
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	require.NoError(t, apikit.WriteSSEEvent(w, "connz", pretty))

	raw := buf.String()
	assert.True(t, strings.HasPrefix(raw, "event: connz\n"))
	assert.Contains(t, raw, "data: {")
	assert.Contains(t, raw, "data:   \"num_connections\": 1,")
	assert.True(t, strings.HasSuffix(raw, "\n\n"))

	// Reconstruct the way EventSource joins consecutive data: lines.
	var parts []string
	for line := range strings.SplitSeq(raw, "\n") {
		if data, ok := strings.CutPrefix(line, "data: "); ok {
			parts = append(parts, data)
		}
	}
	joined := strings.Join(parts, "\n")
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(joined), &payload))
	assert.EqualValues(t, 1, payload["num_connections"])
}

func TestWriteSSEEventCompactJSON(t *testing.T) {
	t.Parallel()

	compact := []byte(`{"ok":true}`)
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	require.NoError(t, apikit.WriteSSEEvent(w, "ping", compact))
	assert.Equal(t, "event: ping\ndata: {\"ok\":true}\n\n", buf.String())
}
