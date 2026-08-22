package live

import (
	"bytes"
)

// parseControlAction recognizes the common live WS control payloads without JSON unmarshal.
// Unknown or malformed input returns "".
func parseControlAction(data []byte) FrameAction {
	data = bytes.TrimSpace(data)
	switch {
	case bytes.Equal(data, []byte(`{"action":"pause"}`)),
		bytes.Equal(data, []byte(`{"action": "pause"}`)):
		return Pause
	case bytes.Equal(data, []byte(`{"action":"resume"}`)),
		bytes.Equal(data, []byte(`{"action": "resume"}`)):
		return Resume
	case bytes.Equal(data, []byte(`{"action":"clear"}`)),
		bytes.Equal(data, []byte(`{"action": "clear"}`)):
		return Clear
	}

	// Tolerant path for minor key reordering / extra whitespace around the action value.
	if i := bytes.Index(data, []byte(`"action"`)); i >= 0 {
		rest := data[i+len(`"action"`):]
		if j := bytes.IndexByte(rest, ':'); j >= 0 {
			rest = bytes.TrimSpace(rest[j+1:])
			switch {
			case bytes.HasPrefix(rest, []byte(`"pause"`)):
				return Pause
			case bytes.HasPrefix(rest, []byte(`"resume"`)):
				return Resume
			case bytes.HasPrefix(rest, []byte(`"clear"`)):
				return Clear
			}
		}
	}
	return ""
}
