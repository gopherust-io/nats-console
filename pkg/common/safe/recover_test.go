package safe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunRecoversPanic(t *testing.T) {
	t.Parallel()
	assert.NotPanics(t, func() {
		Run("test", func() { panic("boom") })
	})
}

func TestRecoverDefer(t *testing.T) {
	t.Parallel()
	assert.NotPanics(t, func() {
		defer Recover("test")
		panic("boom")
	})
}
