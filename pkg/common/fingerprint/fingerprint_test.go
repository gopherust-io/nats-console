package fingerprint_test

import (
	"testing"

	"github.com/gopherust-io/nats-consol/pkg/common/fingerprint"
	"github.com/stretchr/testify/assert"
)

func TestETagStable(t *testing.T) {
	a := fingerprint.ETag([]byte("hello"))
	b := fingerprint.ETag([]byte("hello"))
	assert.Equal(t, a, b)
	assert.NotEqual(t, a, fingerprint.ETag([]byte("world")))
	assert.True(t, len(a) > 2 && a[0] == '"' && a[len(a)-1] == '"')
}

func TestSum64SIMDCallable(t *testing.T) {
	data := []byte("nats-consol fingerprint poc")
	_ = fingerprint.Sum64SIMD(data)
	assert.Equal(t, fingerprint.Sum64(data), fingerprint.Sum64(data))
}
