package b64util

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeToStringStableAfterPoolReuse(t *testing.T) {
	t.Parallel()

	src := []byte("hello-pool-reuse-corruption-check")
	first := EncodeToString(src)
	want := base64.StdEncoding.EncodeToString(src)
	require.Equal(t, want, first)

	// Force another encode that reuses the pooled scratch buffer.
	secondSrc := make([]byte, len(src))
	for i := range secondSrc {
		secondSrc[i] = 'x'
	}
	_ = EncodeToString(secondSrc)

	assert.Equal(t, want, first, "first encode must not alias pooled buffer")
}

func TestEncodeToStringEmpty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", EncodeToString(nil))
	assert.Equal(t, "", EncodeToString([]byte{}))
}
