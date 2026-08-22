package live

import (
	"testing"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/stretchr/testify/assert"
)

func TestParseControlAction(t *testing.T) {
	assert.Equal(t, Pause, parseControlAction([]byte(`{"action":"pause"}`)))
	assert.Equal(t, Resume, parseControlAction([]byte(`{"action":"resume"}`)))
	assert.Equal(t, Clear, parseControlAction([]byte(`{"action": "clear"}`)))
	assert.Equal(t, FrameAction(""), parseControlAction([]byte(`{"action":"noop"}`)))
	assert.Equal(t, Pause, parseControlAction(commonstrings.StringToBytes(`  {"action":"pause"}  `)))
}
